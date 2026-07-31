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
// The loop lives in runStepLoop, shared with the compose and kubectl runners; this supplies only
// the part that differs — how a step's run: commands reach the host.
func (r *LocalRunner) executeSteps(env *config.Environment, steps []config.ProvisionItem) error {
	return runStepLoop(env, r.Opts.Config, steps, func(cmds []string) error {
		// Shell mode is unconditionally true here, as it has always been on this path: a step's
		// run: string is written as a shell line, not as an argv.
		return dvaexec.ExecSequential(env, cmds, true)
	})
}
