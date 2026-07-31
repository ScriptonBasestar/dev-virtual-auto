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
	// steps: first, as in LocalRunner and DockerComposeRunner. Without this branch a config with
	// `pod:` and `steps:` but no `command:` fell straight through to the ExecReplace below, which
	// appended nothing after `--` and replaced the process with `kubectl exec <pod> --`. Every
	// declared step was discarded, dva printed nothing about them, and `dva validate` had already
	// exited 0 — warnUnreachableCommands counts HasSteps() as reachable without asking which
	// runner would run it. TASK-094.
	if len(r.Cmd.Steps) > 0 {
		return r.executeSteps(env, r.Cmd.Steps)
	}

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

// executeSteps runs each step as a separate kubectl exec.
// The loop is runStepLoop, shared with the local and compose runners; only the exec differs.
func (r *KubectlRunner) executeSteps(env *config.Environment, steps []config.ProvisionItem) error {
	return runStepLoop(env, r.Opts.Config, steps, func(cmds []string) error {
		for _, c := range cmds {
			c = strings.TrimSpace(c)
			if c == "" {
				continue
			}
			// ExecSubprocess, not the ExecReplace used by the single-command path above:
			// syscall.Exec does not return, so calling it here would leave every step after
			// the first unreachable — silently, with exit 0. Same reason as TASK-091.
			if err := dvaexec.ExecSubprocess(env, "kubectl", r.buildStepArgs(c), false); err != nil {
				return err
			}
		}
		return nil
	})
}

// buildStepArgs builds kubectl exec args for a single command string.
// Does NOT mutate r.Cmd; constructs args independently per command.
func (r *KubectlRunner) buildStepArgs(cmd string) []string {
	pod, container := parsePod(r.Cmd.Pod)

	args := []string{"exec"}
	if container != "" {
		args = append(args, "--container", container)
	}
	// No --tty/--stdin here, unlike the interactive path: a step is a scripted command with no
	// terminal attached, and kubectl fails outright when asked for a TTY it cannot get.
	// DockerComposeRunner.buildStepArgs omits the equivalent flags for the same reason.
	args = append(args, pod, "--")

	if r.Cmd.Shell {
		args = append(args, "sh", "-c", cmd)
	} else {
		args = append(args, dvaexec.SplitCommand(cmd)...)
	}
	return args
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
