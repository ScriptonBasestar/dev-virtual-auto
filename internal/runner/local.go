package runner

import (
	"fmt"
	"os"
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
func (r *LocalRunner) Execute(env *config.Environment) error {
	return r.runForm(env, classifyForm(r.Cmd))
}

// runForm runs one execution form. Split from Execute so a test can hand it a form no case
// covers and see what happens; the answer used to be "the last if runs" and is now an error.
//
// The precedence this switch used to spell out — steps > script_file > script > list > command
// — moved to classifyForm, which is the only copy of it now. This runner is the one that had
// it right, so nothing here changes what runs; what changes is that the other two runners read
// their form from the same place rather than from a paraphrase.
func (r *LocalRunner) runForm(env *config.Environment, form execForm) error {
	cmd := r.Cmd

	// script_file is resolved before the chdir below so a relative path with no config in
	// hand keeps meaning "relative to where dva was invoked", as it did before TASK-313.
	scriptPath := cmd.ScriptFile
	if form == formScriptFile && !filepath.IsAbs(scriptPath) {
		if r.Opts.Config != nil {
			scriptPath = filepath.Join(r.Opts.Config.FileDir(), scriptPath)
		} else if abs, err := filepath.Abs(scriptPath); err == nil {
			scriptPath = abs
		}
	}

	if err := r.enterWorkdir(); err != nil {
		return err
	}

	switch form {
	case formSteps:
		return r.executeSteps(env, cmd.Steps)

	case formScriptFile:
		return dvaexec.ExecScriptFile(env, scriptPath)

	case formScript:
		return dvaexec.ExecScriptInline(env, cmd.Script)

	case formCommandList:
		// One subprocess per line, stopping at the first failure, and no arguments appended —
		// this is the behaviour the other two runners are being brought to, so it is worth
		// naming rather than leaving as whatever ExecSequential happens to do.
		return dvaexec.ExecSequential(env, cmd.CommandLines, cmd.Shell)

	case formCommand:
		single := strings.TrimSpace(cmd.Command)
		args := commandArgs(cmd)
		return dvaexec.ExecReplace(env, single, args, cmd.Shell)

	default:
		return unhandledFormError("local", form)
	}
}

// executeSteps runs ProvisionItems sequentially (reuses provision runner logic).
// The loop lives in runStepLoop, shared with the compose and kubectl runners; this supplies only
// the part that differs — how a step's run: commands reach the host.
func (r *LocalRunner) executeSteps(env *config.Environment, steps []config.ProvisionItem) error {
	return runStepLoop(env, r.Opts.Config, steps, func(cmds []string) error {
		// Shell mode is unconditionally true here, as it has always been on this path: a step's
		// run: string is written as a shell line, not as an argv.
		return dvaexec.ExecSequential(env, cmds, true)
	})
}

// enterWorkdir applies interaction.workdir for host execution (TASK-313).
//
// Every form ends in a subprocess or a process replacement, and the replacement (formCommand
// via ExecReplace) cannot take a directory of its own, so the process's cwd is the one place
// all five forms read it from. A relative workdir is anchored at the dva.yml directory, not at
// the caller's cwd: the same `dva run x` must land in the same directory from any subfolder,
// which is what the `cd sub && …` chains this replaces were approximating by hand.
//
// A missing directory is an error here rather than a chdir failure from deep inside exec: the
// message names the declared value and where it resolved to, so a typo is visible as a typo.
func (r *LocalRunner) enterWorkdir() error {
	dir := strings.TrimSpace(r.Cmd.Workdir)
	if dir == "" {
		return nil
	}
	resolved := dir
	if !filepath.IsAbs(resolved) {
		base := ""
		if r.Opts.Config != nil {
			base = r.Opts.Config.FileDir()
		}
		if base == "" || base == "." {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("workdir %q: resolving current directory: %w", dir, err)
			}
			base = cwd
		}
		resolved = filepath.Join(base, resolved)
	}
	info, err := os.Stat(resolved)
	switch {
	case err != nil && os.IsNotExist(err):
		return fmt.Errorf("workdir %q: directory not found (resolved to %s)", dir, resolved)
	case err != nil:
		return fmt.Errorf("workdir %q: %w", dir, err)
	case !info.IsDir():
		return fmt.Errorf("workdir %q: not a directory (resolved to %s)", dir, resolved)
	}
	if err := os.Chdir(resolved); err != nil {
		return fmt.Errorf("workdir %q: %w", dir, err)
	}
	return nil
}
