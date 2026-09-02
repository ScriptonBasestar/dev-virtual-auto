package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
	"github.com/ScriptonBasestar/dva/internal/output"
)

var provisionList bool

var provisionCmd = &cobra.Command{
	Use:   "provision [PROFILE]",
	Short: "Execute the provisioning steps defined in 'dva.yml'",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()

		// --list: show available profiles and exit
		if provisionList {
			return listProvisionProfiles(c)
		}

		requested := "default"
		explicit := len(args) > 0
		if explicit {
			requested = args[0]
		}

		profile, steps, err := resolveProvisionProfile(c.Provision.Profiles, c.Provision.DefaultProfile, requested, explicit)
		if err != nil {
			return err
		}

		// Owner and its environment are resolved after the profile name is settled and
		// before the first step runs. An imported profile executes against the child that
		// declared it, so its steps see the child's vars, environment: and env_file and run
		// from the child config directory; a root profile is unchanged. Resolving here also
		// keeps a broken root env_file from blocking a purely child-owned profile — the
		// warning-and-continue policy itself is TASK-248's to change (TASK-264).
		rt, err := resolveProvisionRuntime(c, profile)
		if err != nil {
			return err
		}
		e, owner := rt.env, rt.config

		if dryRun {
			fmt.Printf("🔍 DRY RUN — showing execution plan for profile: %s\n\n", profile)
		} else {
			fmt.Printf("🚀 Running provision profile: %s\n\n", profile)
		}

		// Execute steps with parallel batch support
		batches := groupParallelBatches(steps)
		stepOffset := 0
		for _, batch := range batches {
			if len(batch) == 1 || !batch[0].Parallel {
				// Sequential execution
				for _, step := range batch {
					if err := executeProvisionStep(e, owner, step, stepOffset, len(steps), dryRun); err != nil {
						return err
					}
					stepOffset++
				}
			} else {
				// Parallel execution
				if err := executeParallelBatch(e, owner, batch, stepOffset, len(steps), dryRun); err != nil {
					return err
				}
				stepOffset += len(batch)
			}
		}

		if dryRun {
			fmt.Println("\n🔍 Dry run complete — no commands were executed.")
		} else {
			// Write provision marker so `dva up` knows this profile was run
			writeProvisionMarker(c.FileDir(), profile)
			fmt.Println("\n✅ Provision complete!")
		}
		return nil
	},
}

func init() {
	provisionCmd.Flags().BoolVarP(&provisionList, "list", "l", false, "List available provision profiles")
}

// groupParallelBatches groups steps into batches. Consecutive steps with
// Parallel=true form a single batch; non-parallel steps are each their own batch.
func groupParallelBatches(steps []config.ProvisionItem) [][]config.ProvisionItem {
	var batches [][]config.ProvisionItem
	var current []config.ProvisionItem

	for _, step := range steps {
		if step.Parallel {
			current = append(current, step)
		} else {
			if len(current) > 0 {
				batches = append(batches, current)
				current = nil
			}
			batches = append(batches, []config.ProvisionItem{step})
		}
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches
}

// writeNote renders a step's `note:` — the key whose entire purpose is to be seen.
//
// It exists because three paths need the identical rendering and only one had it
// (TASK-086): the sequential provision step printed the note, while executeParallelBatch
// and compose.go's native build loop never read the field at all, so adding
// `parallel: true` — a scheduling hint — silently deleted the step's message.
//
// The blank line either side and the four-space indent are the sequential path's
// formatting, kept byte-for-byte because that path is the reference the other two are
// being brought into line with, not a third opinion. The io.Writer is what makes it
// usable from the parallel path, whose sink depends on the mode: a per-step
// stepPrefixWriter when executing, a per-step bytes.Buffer under --dry-run (TASK-168).
func writeNote(w io.Writer, note string) {
	if note == "" {
		return
	}

	// Composed first, written once. The bytes are identical either way, but the single Write
	// is what keeps the block a block on the parallel path: stepPrefixWriter holds its lock
	// for the whole of one Write, so every line of the note lands together. Emitted as three
	// Fprintf calls — a blank line, the body, a blank line — it took the lock three times and
	// another step's output could land inside the note it was separating itself from.
	var b strings.Builder
	b.WriteString("\n")
	for line := range strings.SplitSeq(note, "\n") {
		fmt.Fprintf(&b, "    %s\n", line)
	}
	b.WriteString("\n")

	// Error dropped explicitly rather than by a config-level exclusion: w is stdout, the
	// parallel path's per-step buffer, or its prefixing writer over stdout — none of which
	// can fail in a way this function could act on.
	_, _ = io.WriteString(w, b.String())
}

// executeProvisionStep runs a single provision step sequentially.
func executeProvisionStep(e *config.Environment, c *config.Config, step config.ProvisionItem, index, total int, dryRun bool) error {
	if step.Step != "" {
		fmt.Printf("  [%d/%d] %s\n", index+1, total, step.Step)
	}

	// Reported on the dry-run path too, and that is the point: `provision --dry-run` exists
	// to answer "what will happen", and it used to list an inert step in the plan with no
	// command beneath it, which is what a compose step looks like as well.
	if step.IsInert() {
		fmt.Printf("    ⚠ %s\n", config.InertStepMessage)
		return nil
	}

	writeNote(os.Stdout, step.Note)

	// Execute compose-aware commands
	if len(step.ComposeUp) > 0 {
		composeArgs := append([]string{"up", "-d"}, step.ComposeUp...)
		if dryRun {
			composeCmd, args, err := buildComposeArgs(e, c, composeArgs)
			if err != nil {
				return err
			}
			fmt.Printf("    [dry-run] $ %s %s\n", composeCmd, strings.Join(args, " "))
		} else {
			if err := runProvisionCompose(e, c, step.Step, composeArgs); err != nil {
				return err
			}
		}
		return nil
	}

	if step.ComposeExec != "" {
		composeArgs := append([]string{"exec"}, dvaexec.SplitCommand(step.ComposeExec)...)
		if dryRun {
			composeCmd, args, err := buildComposeArgs(e, c, composeArgs)
			if err != nil {
				return err
			}
			fmt.Printf("    [dry-run] $ %s %s\n", composeCmd, strings.Join(args, " "))
		} else {
			if err := runProvisionCompose(e, c, step.Step, composeArgs); err != nil {
				return err
			}
		}
		return nil
	}

	if step.ComposeRun != "" {
		composeArgs := append([]string{"run"}, dvaexec.SplitCommand(step.ComposeRun)...)
		if dryRun {
			composeCmd, args, err := buildComposeArgs(e, c, composeArgs)
			if err != nil {
				return err
			}
			fmt.Printf("    [dry-run] $ %s %s\n", composeCmd, strings.Join(args, " "))
		} else {
			if err := runProvisionCompose(e, c, step.Step, composeArgs); err != nil {
				return err
			}
		}
		return nil
	}

	// Execute raw commands
	cmds := step.RunCommands()
	for _, cmdStr := range cmds {
		if dryRun {
			fmt.Printf("    [dry-run] $ %s\n", cmdStr)
		} else {
			fmt.Printf("    $ %s\n", cmdStr)
			if err := runShellCommand(e, cmdStr); err != nil {
				return fmt.Errorf("provision step '%s' failed: %w", step.Step, err)
			}
		}
	}

	// Legacy format: echo
	if step.Echo != "" {
		fmt.Printf("    %s\n", step.Echo)
	}

	// Legacy format: cmd
	if step.Cmd != "" {
		if dryRun {
			fmt.Printf("    [dry-run] $ %s\n", step.Cmd)
		} else {
			fmt.Printf("    $ %s\n", step.Cmd)
			if err := runShellCommand(e, step.Cmd); err != nil {
				return fmt.Errorf("provision command failed: %w", err)
			}
		}
	}
	return nil
}

// executeParallelBatch runs a batch of parallel steps concurrently.
//
// Two output modes, because the two cases are genuinely different (TASK-168):
//
//   - Executing: each step writes through a stepPrefixWriter, and so do its commands, so every
//     line is tagged with the step that produced it and appears as it is produced. The old
//     per-step bytes.Buffer could not do this — it caught only the lines dva composed, while
//     the children wrote past it to os.Stdout, so the labels arrived after their own output.
//   - Dry run: the per-step buffer is kept, flushed in declaration order. No child runs, so
//     nothing can escape the buffer and the defect above cannot occur; and a dry run's job is
//     to show a *plan*, which should be listed in the order the steps are written rather than
//     in whatever order the goroutines happened to finish.
//
// Only ever reached with two or more steps: a one-step batch goes to the sequential executor.
func executeParallelBatch(e *config.Environment, c *config.Config, batch []config.ProvisionItem, startIndex, total int, dryRun bool) error {
	fmt.Printf("  ⚡ Running %d steps in parallel...\n", len(batch))

	type result struct {
		index  int
		output string
		err    error
	}

	labels := make([]string, len(batch))
	for i, s := range batch {
		name := s.Step
		if name == "" {
			name = "(unnamed)"
		}
		labels[i] = fmt.Sprintf("  [%d/%d] %s", startIndex+i+1, total, name)
	}
	writers := newStepPrefixWriters(os.Stdout, labels)

	results := make([]result, len(batch))
	var wg sync.WaitGroup

	for i, step := range batch {
		wg.Add(1)
		go func(idx int, s config.ProvisionItem) {
			defer wg.Done()
			var buf bytes.Buffer
			stepLabel := fmt.Sprintf("[%d/%d]", startIndex+idx+1, total)

			// w is where this step's own lines go. Under a dry run that is the buffer the
			// caller flushes in order; otherwise it is the prefixing writer, and the step's
			// commands below are pointed at the same one.
			var w io.Writer = &buf
			if !dryRun {
				w = writers[idx]
				defer writers[idx].Flush()
			}

			// The standalone label line is for the buffered shape only. When prefixing, every
			// line already carries `[i/n] name`, and printing it again would render as
			// "  [1/3] alpha │   [1/3] alpha".
			if dryRun && s.Step != "" {
				_, _ = fmt.Fprintf(w, "  %s %s\n", stepLabel, s.Step)
			}

			// One check covering both branches below, so the parallel path cannot drift
			// from the sequential one the way these loops already had.
			if s.IsInert() {
				_, _ = fmt.Fprintf(w, "    ⚠ %s\n", config.InertStepMessage)
				results[idx] = result{index: idx, output: buf.String()}
				return
			}

			// Before the dry-run branch, not inside it: a note describes the step, so it is
			// shown whether or not the step's commands are going to run. Ordering matches the
			// sequential path — after the label and the inert check, before any command.
			writeNote(w, s.Note)

			if dryRun {
				// A dry run that cannot build the argv has nothing true to print, so the
				// error goes back through result.err exactly as an execution failure would.
				// This branch runs in a goroutine, which is why it cannot simply return one.
				var dryErr error
				dryCompose := func(composeArgs []string) error {
					composeCmd, args, err := buildComposeArgs(e, c, composeArgs)
					if err != nil {
						return err
					}
					_, _ = fmt.Fprintf(w, "    [dry-run] $ %s %s\n", composeCmd, strings.Join(args, " "))
					return nil
				}

				// Dry run: just show what would run
				if len(s.ComposeUp) > 0 {
					dryErr = dryCompose(append([]string{"up", "-d"}, s.ComposeUp...))
				} else if s.ComposeExec != "" {
					dryErr = dryCompose(append([]string{"exec"}, dvaexec.SplitCommand(s.ComposeExec)...))
				} else if s.ComposeRun != "" {
					dryErr = dryCompose(append([]string{"run"}, dvaexec.SplitCommand(s.ComposeRun)...))
				} else {
					for _, cmdStr := range s.RunCommands() {
						_, _ = fmt.Fprintf(w, "    [dry-run] $ %s\n", cmdStr)
					}
					if s.Cmd != "" {
						_, _ = fmt.Fprintf(w, "    [dry-run] $ %s\n", s.Cmd)
					}
				}
				if s.Echo != "" {
					_, _ = fmt.Fprintf(w, "    %s\n", s.Echo)
				}
				results[idx] = result{index: idx, output: buf.String(), err: dryErr}
				return
			}

			// Actual execution. The children are pointed at w — the whole point of TASK-168:
			// before this they went to os.Stdout and left the batch's own lines describing
			// output that had already scrolled past.
			var err error
			if len(s.ComposeUp) > 0 {
				err = runProvisionComposeTo(e, c, s.Step, append([]string{"up", "-d"}, s.ComposeUp...), w, w, w)
			} else if s.ComposeExec != "" {
				err = runProvisionComposeTo(e, c, s.Step, append([]string{"exec"}, dvaexec.SplitCommand(s.ComposeExec)...), w, w, w)
			} else if s.ComposeRun != "" {
				err = runProvisionComposeTo(e, c, s.Step, append([]string{"run"}, dvaexec.SplitCommand(s.ComposeRun)...), w, w, w)
			} else {
				for _, cmdStr := range s.RunCommands() {
					_, _ = fmt.Fprintf(w, "    $ %s\n", cmdStr)
					if err = runShellCommandTo(e, cmdStr, w, w); err != nil {
						err = fmt.Errorf("provision step '%s' failed: %w", s.Step, err)
						break
					}
				}
				if err == nil && s.Cmd != "" {
					_, _ = fmt.Fprintf(w, "    $ %s\n", s.Cmd)
					if err = runShellCommandTo(e, s.Cmd, w, w); err != nil {
						err = fmt.Errorf("provision command failed: %w", err)
					}
				}
			}
			if s.Echo != "" {
				_, _ = fmt.Fprintf(w, "    %s\n", s.Echo)
			}
			// buf is empty on this path; the step's lines have already been written. It is
			// still read so the two branches return the same shape.
			results[idx] = result{index: idx, output: buf.String(), err: err}
		}(i, step)
	}

	wg.Wait()

	// Print output in order and collect errors
	var errs []error
	for _, r := range results {
		if r.output != "" {
			fmt.Print(r.output)
		}
		if r.err != nil {
			errs = append(errs, r.err)
		}
	}

	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return fmt.Errorf("parallel provision failed:\n  %s", strings.Join(msgs, "\n  "))
	}
	return nil
}

// resolveProvisionProfile resolves which provision profile to use.
// Priority: exact match → default_profile alias → single-profile auto → error.
func resolveProvisionProfile(provision map[string][]config.ProvisionItem, defaultProfile string, requested string, explicit bool) (string, []config.ProvisionItem, error) {
	// Direct lookup
	if steps, ok := provision[requested]; ok {
		return requested, steps, nil
	}

	// No provision defined at all
	if len(provision) == 0 {
		return "", nil, fmt.Errorf("no provision commands defined in dva.yml")
	}

	// Implicit fallbacks only (user did not specify a profile name)
	if requested == "default" && !explicit {
		// default_profile alias (explicit config, works with any number of profiles)
		if defaultProfile != "" {
			if steps, ok := provision[defaultProfile]; ok {
				fmt.Fprintf(os.Stderr, "⚠ Profile 'default' not found, using '%s' (default_profile)\n\n", defaultProfile)
				return defaultProfile, steps, nil
			}
			fmt.Fprintf(os.Stderr, "⚠ default_profile '%s' not found in provision profiles — ignoring\n\n", defaultProfile)
		}

		// Auto-fallback: exactly 1 profile
		if len(provision) == 1 {
			for k, v := range provision {
				fmt.Fprintf(os.Stderr, "⚠ Profile 'default' not found, using '%s' (only available profile)\n\n", k)
				return k, v, nil
			}
		}
	}

	// Build available list
	available := make([]string, 0, len(provision))
	for k := range provision {
		available = append(available, k)
	}
	sort.Strings(available)

	// "Did you mean?" suggestion
	var suggestions []string
	for _, name := range available {
		if levenshtein(requested, name) <= 2 {
			suggestions = append(suggestions, name)
		}
	}

	var msg strings.Builder
	fmt.Fprintf(&msg, "provision profile '%s' not found. Available: %s", requested, strings.Join(available, ", "))
	if len(suggestions) > 0 {
		msg.WriteString("\n\nDid you mean?")
		for _, s := range suggestions {
			fmt.Fprintf(&msg, "\n  dva provision %s", s)
		}
	}

	return "", nil, fmt.Errorf("%s", msg.String())
}

// listProvisionProfiles prints provision profiles in the requested format.
func listProvisionProfiles(c *config.Config) error {
	if len(c.Provision.Profiles) == 0 {
		fmt.Println("No provision profiles defined.")
		return nil
	}

	keys := make([]string, 0, len(c.Provision.Profiles))
	for k := range c.Provision.Profiles {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if jsonOutput {
		return printProvisionJSON(c.Provision.Profiles, c.Provision.DefaultProfile, keys)
	}
	return printProvisionTable(c.Provision.Profiles, c.Provision.DefaultProfile, keys)
}

func printProvisionTable(provision map[string][]config.ProvisionItem, defaultProfile string, keys []string) error {
	maxName := len("PROFILE")
	for _, k := range keys {
		if len(k) > maxName {
			maxName = len(k)
		}
	}
	if defaultProfile != "" {
		maxName += 2 // room for " *" suffix
	}

	fmt.Printf("%-*s  %5s  %s\n", maxName, "PROFILE", "STEPS", "FIRST STEP")
	for _, k := range keys {
		steps := provision[k]
		desc := firstStepDescription(steps)
		display := k
		if k == defaultProfile {
			display = k + " *"
		}
		fmt.Printf("%-*s  %5d  %s\n", maxName, display, len(steps), desc)
	}
	if defaultProfile != "" {
		fmt.Printf("\n* default profile\n")
	}
	return nil
}

func printProvisionJSON(provision map[string][]config.ProvisionItem, defaultProfile string, keys []string) error {
	profiles := make(map[string]any, len(keys))
	for _, k := range keys {
		steps := provision[k]
		profiles[k] = map[string]any{
			"steps":      len(steps),
			"first_step": firstStepDescription(steps),
		}
	}
	result := map[string]any{"profiles": profiles}
	if defaultProfile != "" {
		result["default_profile"] = defaultProfile
	}
	return output.PrintJSON(result)
}

func firstStepDescription(steps []config.ProvisionItem) string {
	if len(steps) == 0 {
		return ""
	}
	s := steps[0]
	if s.Step != "" {
		return s.Step
	}
	if s.Raw != "" {
		return s.Raw
	}
	if s.Echo != "" {
		return s.Echo
	}
	return ""
}

// runProvisionCompose builds and runs a compose command for a provision step.
// Uses exec.Command directly instead of shell to avoid command injection.
func runProvisionCompose(e *config.Environment, c *config.Config, stepName string, args []string) error {
	return runProvisionComposeTo(e, c, stepName, args, os.Stdout, os.Stdout, os.Stderr)
}

// runProvisionComposeTo is runProvisionCompose with its three output streams named: where the
// `$ …` echo goes, and where the child's stdout and stderr go. Only the parallel batch passes
// anything but os.Stdout/os.Stderr, which is why the plain form above still exists — the eight
// sequential callers have nothing to say about streams and are not made to say it (TASK-168).
func runProvisionComposeTo(e *config.Environment, c *config.Config, stepName string, args []string, echo, stdout, stderr io.Writer) error {
	composeCmd, composeArgs, err := buildComposeArgs(e, c, args)
	if err != nil {
		return fmt.Errorf("provision step '%s': %w", stepName, err)
	}
	_, _ = fmt.Fprintf(echo, "    $ %s %s\n", composeCmd, strings.Join(composeArgs, " "))

	cmd := exec.Command(composeCmd, composeArgs...)
	// Stdin stays os.Stdin even under a parallel batch. Two concurrent children sharing the
	// terminal's input is its own problem and not this one's; taking stdin away here would
	// change which commands can run, not just how their output is labelled.
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = e.EnvSlice()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("provision step '%s' failed: %w", stepName, err)
	}
	return nil
}

func runShellCommand(e *config.Environment, cmdStr string) error {
	return runShellCommandTo(e, cmdStr, os.Stdout, os.Stderr)
}

// runShellCommandTo is runShellCommand with the child's streams injectable. See
// runProvisionComposeTo for why the undecorated form is kept.
func runShellCommandTo(e *config.Environment, cmdStr string, stdout, stderr io.Writer) error {
	var c *exec.Cmd

	// Platform-specific shell selection
	switch runtime.GOOS {
	case "windows":
		// Use PowerShell for better compatibility
		c = exec.Command("powershell", "-Command", cmdStr)
	default:
		// Unix-like systems (Linux, macOS, BSD, etc.)
		c = exec.Command("sh", "-c", cmdStr)
	}

	c.Stdin = os.Stdin
	c.Stdout = stdout
	c.Stderr = stderr
	if e != nil {
		c.Dir = e.WorkDir()
		c.Env = e.EnvSlice()
	}
	return c.Run()
}

// clearProvisionMarkers removes all provision marker files from .sb/dva/.
// Called by `dva clean --volumes` so that provision suggestions reappear after a reset.
func clearProvisionMarkers(configDir string) {
	for _, m := range provisionMarkers(configDir) {
		_ = os.Remove(m)
	}
}

// provisionMarkers lists what clearProvisionMarkers would delete, as full paths.
//
// The probe-only half of the same walk, in the shape of portOwnerPIDs against reclaimPort:
// `dva clean --dry-run` needs to name the files without removing them, and a preview that
// re-derived the "provisioned-" prefix itself would be free to drift from the deletion it
// claims to describe. Returns nil for an unreadable directory, which is the same silence
// clearProvisionMarkers has always kept — a missing .sb/dva is the ordinary case on a
// project that has never provisioned. TASK-166.
// provisionMarkerName is the file name recording that a profile has been provisioned.
//
// The marker stays in the *invoked* project's dot-directory even for an imported profile:
// it answers "has this project been provisioned", which is the question `dva up` asks of
// the config it was run against, not of the child that owns the steps.
//
// The slash replacement is what makes an imported name usable as a file name at all.
// Imports register canonically as `child/profile`, and filepath.Join then read that slash
// as a directory component: MkdirAll had created only the dot-directory, so the write
// failed with ENOENT and every imported provision run ended on a warning. The literal "/"
// is the separator applySubprojectImports writes, on every platform. Shared with the
// reader in compose.go so the two cannot spell the same profile differently (TASK-264).
func provisionMarkerName(profile string) string {
	return "provisioned-" + strings.ReplaceAll(profile, "/", "__")
}

func provisionMarkers(configDir string) []string {
	markerDir := filepath.Join(configDir, config.DotDirName)
	entries, err := os.ReadDir(markerDir)
	if err != nil {
		return nil
	}
	var found []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "provisioned-") {
			found = append(found, filepath.Join(markerDir, e.Name()))
		}
	}
	return found
}

// writeProvisionMarker creates a marker file indicating that a provision
// profile has been run. Used by `dva up` to skip provision suggestions.
func writeProvisionMarker(configDir, profile string) {
	markerDir := filepath.Join(configDir, config.DotDirName)
	if err := os.MkdirAll(markerDir, 0755); err != nil {
		return
	}
	markerFile := filepath.Join(markerDir, provisionMarkerName(profile))
	if err := os.WriteFile(markerFile, []byte(""), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "[warn] could not write provision marker: %v\n", err)
	}
}
