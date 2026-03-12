package runner

import (
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

// KubectlRunner executes commands via kubectl exec.
type KubectlRunner struct {
	Cmd  *ResolvedCommand
	Opts RunOptions
}

// Execute builds and runs the kubectl exec command.
func (r *KubectlRunner) Execute(env *config.Environment) error {
	pod, container := parsePod(r.Cmd.Pod)

	var args []string
	args = append(args, "exec")
	args = append(args, "--tty", "--stdin")

	if container != "" {
		args = append(args, "--container", container)
	}

	args = append(args, pod, "--")

	if r.Cmd.Entrypoint != "" {
		args = append(args, r.Cmd.Entrypoint)
	}

	cmd := strings.TrimSpace(r.Cmd.Command)
	if cmd != "" {
		args = append(args, dvaexec.SplitCommand(cmd)...)
	}

	args = append(args, commandArgs(r.Cmd)...)

	return dvaexec.ExecReplace(env, "kubectl", args, false)
}

// parsePod splits "pod:container" into pod and container.
func parsePod(spec string) (pod, container string) {
	parts := strings.SplitN(spec, ":", 2)
	pod = parts[0]
	if len(parts) > 1 {
		container = parts[1]
	}
	return
}
