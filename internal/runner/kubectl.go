package runner

import (
	"fmt"
	"os"
	"path/filepath"
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

	args, err := r.execArgs()
	if err != nil {
		return err
	}

	return dvaexec.ExecReplace(env, "kubectl", args, false)
}

// execArgs builds the argv for the one-shot paths — every execution form except steps, which
// loops and belongs to executeSteps.
//
// Split out of Execute so the argv is observable without a cluster. Execute ends in
// syscall.Exec, so until this seam existed there was nothing a test could assert on; it is the
// same split DockerComposeRunner.executeArgs opened for compose (TASK-132).
func (r *KubectlRunner) execArgs() ([]string, error) {
	args := r.execPrefix(true)

	// script:/script_file: ahead of the command form, matching LocalRunner's documented
	// precedence (steps > script_file > script > command). Before TASK-175 neither field was
	// read anywhere in this file, so a `pod:` interaction declaring one fell straight through
	// to the command form below: `kubectl exec <pod> --` with nothing after it at the top
	// level, or — for a subcommand — the *parent's* inherited command. That is TASK-094's
	// defect in the same function, two execution forms short.
	if r.Cmd.ScriptFile != "" || r.Cmd.Script != "" {
		body, err := r.scriptBody()
		if err != nil {
			return nil, err
		}

		// Run it in the pod, rather than falling back to LocalRunner the way
		// DockerComposeRunner does for this case. That precedent does not carry: a compose
		// container shares the host the CLI runs on, while a pod is in a cluster that usually
		// is not this machine. Running the script here would point a `pod:`-declared script at
		// the developer's own filesystem and database — silently at the wrong target, which is
		// the defect being fixed, relocated rather than removed.
		//
		// sh, not the interpreter named by a shebang, even though the local path honours one by
		// writing a temp file and exec'ing it. schema.json documents both fields as *shell*
		// scripts and no document mentions a shebang at all, so sh is the promised contract —
		// and it is the choice with no silent-wrong mode. A `#!/usr/bin/perl` body under
		// `sh -c` dies on its first line, loudly. The same body under a shebang-honouring
		// `perl -c` would run a syntax check, print nothing and exit 0: the more faithful
		// looking design is the one that fails silently.
		//
		// No arguments are appended. A script consumes none locally either, and since TASK-149
		// a subcommand declaring script: no longer inherits the parent's default_args.
		return append(args, "sh", "-c", body), nil
	}

	if r.Cmd.Entrypoint != "" {
		args = append(args, r.Cmd.Entrypoint)
	}

	cmd := strings.TrimSpace(r.Cmd.Command)
	if cmd != "" {
		args = append(args, dvaexec.SplitCommand(cmd)...)
	}

	return append(args, commandArgs(r.Cmd)...), nil
}

// scriptBody returns the shell source to run in the pod: script_file's contents when set,
// otherwise script. Precedence matches LocalRunner.
//
// The file is read here rather than named to kubectl because it lives on the host — the pod has
// no copy of it, so there is no path that would mean the same thing on the other side. Relative
// paths resolve against the directory holding dva.yml, as they do locally.
func (r *KubectlRunner) scriptBody() (string, error) {
	if r.Cmd.ScriptFile == "" {
		return r.Cmd.Script, nil
	}

	path := r.Cmd.ScriptFile
	if !filepath.IsAbs(path) && r.Opts.Config != nil {
		path = filepath.Join(r.Opts.Config.FileDir(), path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("script_file %q: %w", path, err)
	}
	return string(data), nil
}

// execPrefix builds `exec [--tty --stdin] [--container c] <pod> --`, the head every kubectl
// invocation this runner makes shares.
//
// tty splits the one-shot paths from the step loop, which is the distinction that already
// existed rather than a new policy: a single `dva run` is a foreground command and gets the
// terminal, while a step is a scripted line with no terminal attached and kubectl fails outright
// when asked for a TTY it cannot get. DockerComposeRunner.buildStepArgs omits the equivalent
// flags for the same reason.
func (r *KubectlRunner) execPrefix(tty bool) []string {
	pod, container := parsePod(r.Cmd.Pod)

	args := []string{"exec"}
	if tty {
		args = append(args, "--tty", "--stdin")
	}
	if container != "" {
		args = append(args, "--container", container)
	}
	return append(args, pod, "--")
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
//
// Takes no Environment, unlike its DockerComposeRunner counterpart, and that asymmetry is
// not a pending decision. TASK-129 made compose forward the declared environment as -e;
// kubectl cannot do the same, because `kubectl exec` has no env flag to forward it with
// (measured against the installed client: six flags — container, filename,
// pod-running-timeout, quiet, stdin, tty — and no occurrence of "env" in its help at all;
// `kubectl run` does have --env=[], which is what makes the absence meaningful). Both of
// this runner's paths build `exec`, the single-command one at Execute and steps here, so
// there is no kubectl path that could carry it. A pod's environment comes from its spec.
func (r *KubectlRunner) buildStepArgs(cmd string) []string {
	// tty=false, unlike the one-shot paths: a step is a scripted command with no terminal
	// attached. execPrefix carries that reasoning and the container handling both paths need.
	args := r.execPrefix(false)

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
