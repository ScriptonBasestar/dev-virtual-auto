package runner

import (
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

	switch form {
	case formSteps:
		return r.executeSteps(env, cmd.Steps)

	case formScriptFile:
		path := cmd.ScriptFile
		if !filepath.IsAbs(path) && r.Opts.Config != nil {
			path = filepath.Join(r.Opts.Config.FileDir(), path)
		}
		return dvaexec.ExecScriptFile(env, path)

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
