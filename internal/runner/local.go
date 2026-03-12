package runner

import (
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

// LocalRunner executes commands directly on the host.
type LocalRunner struct {
	Cmd  *ResolvedCommand
	Opts RunOptions
}

// Execute runs the command locally via exec (replaces current process).
func (r *LocalRunner) Execute(env *config.Environment) error {
	cmd := strings.TrimSpace(r.Cmd.Command)
	args := commandArgs(r.Cmd)
	return dvaexec.ExecReplace(env, cmd, args, r.Cmd.Shell)
}
