package runner

import (
	"fmt"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// runStepLoop executes an interaction's `steps:` sequentially. Every runner shares this body and
// supplies only execCommands — how to run the `run:` command strings of one step.
//
// It is one function rather than one per runner because the per-runner copies are what produced
// TASK-083, TASK-085, TASK-089 and TASK-091: four separate defects, each one a key implemented in
// LocalRunner and not in DockerComposeRunner or the reverse. TASK-094 was the same class again —
// KubectlRunner had no copy at all, so `runner: kubectl` with `steps:` ran nothing. Adding a third
// copy would have made a fourth divergence a matter of time.
//
// execCommands receives every non-empty `run:` string of the step at once; a runner that must
// issue them one at a time loops inside its own callback.
func runStepLoop(env *config.Environment, cfg *config.Config, steps []config.ProvisionItem, execCommands func(cmds []string) error) error {
	for i, step := range steps {
		label := step.Step
		if label == "" {
			label = fmt.Sprintf("step %d", i+1)
		}
		// Before the note check, though the order does not matter: an item with a note is not
		// inert. LocalRunner used to reach the emptiness test below and `continue` without ever
		// printing the label, so an inert step left no trace at all — the hook path at least
		// printed its label. Now every runner says the same thing (TASK-083).
		if step.IsInert() {
			fmt.Printf("  → %s\n", label)
			fmt.Printf("    ⚠ %s\n", config.InertStepMessage)
			continue
		}
		// A note documents the step, it does not replace it. This branch used to `continue`, so
		// adding a note to a working step silently stopped it running — while the same item under
		// `dva provision` printed the note and executed. TASK-089; provision.go was the correct
		// one, so every runner falls through.
		noted := step.Note != ""
		if noted {
			fmt.Printf("  → %s: %s\n", label, step.Note)
		}
		cmds := step.RunCommands()
		if len(cmds) == 0 && step.Raw != "" {
			cmds = []string{step.Raw}
		}
		// "No run: commands" is not "no work". compose_up/compose_exec/compose_run/echo/cmd are
		// payloads too, and this test used to be `len(cmds) == 0`, which dropped all five before
		// even printing the label — zero bytes of output, exit 0, while the identical item under
		// `dva provision` worked (TASK-085). A note-only step still falls through to `continue`,
		// which is what TASK-089's tests pin.
		if len(cmds) == 0 && !hasStepKeys(step) {
			continue
		}
		if !noted {
			// The note line already named the step; printing the label again would double it.
			fmt.Printf("  → %s\n", label)
		}
		// A compose key stands in for the entire step, exactly as it does on the provision path:
		// provision.go returns immediately after each one rather than falling through.
		handled, err := runComposeStepKeys(env, cfg, step)
		if err != nil {
			return fmt.Errorf("step %q failed: %w", label, err)
		}
		if handled {
			continue
		}
		if err := execCommands(cmds); err != nil {
			return fmt.Errorf("step %q failed: %w", label, err)
		}
		// After run:, matching provision.go's ordering — an item may carry both.
		if err := runLegacyStepKeys(env, step); err != nil {
			return fmt.Errorf("step %q failed: %w", label, err)
		}
	}
	return nil
}
