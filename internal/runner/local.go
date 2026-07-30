package runner

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

// LocalRunner executes commands directly on the host.
type LocalRunner struct {
	Cmd  *ResolvedCommand
	Opts RunOptions
}

// Execute runs the command locally via exec.
// Priority: steps > script_file > script > command list > single command
func (r *LocalRunner) Execute(env *config.Environment) error {
	cmd := r.Cmd

	// 1. steps: named steps executed sequentially
	if len(cmd.Steps) > 0 {
		return r.executeSteps(env, cmd.Steps)
	}

	// 2. script_file: external shell script
	if cmd.ScriptFile != "" {
		path := cmd.ScriptFile
		if !filepath.IsAbs(path) && r.Opts.Config != nil {
			path = filepath.Join(r.Opts.Config.FileDir(), path)
		}
		return dvaexec.ExecScriptFile(env, path)
	}

	// 3. script: inline shell script block
	if cmd.Script != "" {
		return dvaexec.ExecScriptInline(env, cmd.Script)
	}

	// 4. command list (command: [a, b, c])
	if len(cmd.CommandLines) > 0 {
		return dvaexec.ExecSequential(env, cmd.CommandLines, cmd.Shell)
	}

	// 5. single command (original behavior)
	single := strings.TrimSpace(cmd.Command)
	args := commandArgs(cmd)
	return dvaexec.ExecReplace(env, single, args, cmd.Shell)
}

// executeSteps runs ProvisionItems sequentially (reuses provision runner logic).
func (r *LocalRunner) executeSteps(env *config.Environment, steps []config.ProvisionItem) error {
	for i, step := range steps {
		label := step.Step
		if label == "" {
			label = fmt.Sprintf("step %d", i+1)
		}
		// Before the note check, though the order does not matter: an item with a note is
		// not inert. This runner used to reach the emptiness test below and `continue`
		// without ever printing the label, so an inert step left no trace at all — the
		// hook path at least printed its label. Now both say the same thing.
		if step.IsInert() {
			fmt.Printf("  → %s\n", label)
			fmt.Printf("    ⚠ %s\n", config.InertStepMessage)
			continue
		}
		if step.Note != "" {
			fmt.Printf("  → %s: %s\n", label, step.Note)
			continue
		}
		cmds := step.RunCommands()
		if len(cmds) == 0 && step.Raw != "" {
			cmds = []string{step.Raw}
		}
		if len(cmds) == 0 {
			continue
		}
		fmt.Printf("  → %s\n", label)
		if err := dvaexec.ExecSequential(env, cmds, true); err != nil {
			return fmt.Errorf("step %q failed: %w", label, err)
		}
	}
	return nil
}
