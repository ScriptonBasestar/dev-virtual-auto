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
	return r.runForm(env, classifyForm(r.Cmd))
}

// runForm runs one execution form.
//
// This switch is the one that kept coming up short. Before TASK-094 it was a single ExecReplace
// with no branch at all, so `steps:` fell through it and `kubectl exec <pod> --` ran with
// nothing after the separator; before TASK-175 `script:` and `script_file:` did the same, or
// worse, ran the parent's inherited command; before TASK-178 a list `command:` ran only its
// first line. Three tasks, three forms, one if-chain that ended in an unguarded fall-through.
// The forms now come from classifyForm and the fall-through is an error — see execform.go.
func (r *KubectlRunner) runForm(env *config.Environment, form execForm) error {
	switch form {
	case formSteps:
		return r.executeSteps(env, r.Cmd.Steps)

	case formCommandList:
		return r.execEach(env, r.Cmd.CommandLines)

	case formScriptFile, formScript, formCommand:
		// The one-shot forms: exactly one kubectl process, so this path may replace dva with
		// it. The two above must not — see execEach.
		args, err := r.execArgs()
		if err != nil {
			return err
		}
		return dvaexec.ExecReplace(env, "kubectl", args, false)

	default:
		return unhandledFormError("kubectl", form)
	}
}

// execEach runs one `kubectl exec` per command, in order, stopping at the first failure.
//
// This is what a list `command:` gets, and it is deliberately the same machinery a step's run:
// lines already use: a list is a sequence of commands, which is what `steps:` is minus the
// labels. Reusing buildStepArgs rather than execArgs carries two properties that a list needs
// and the one-shot path cannot give it — ExecSubprocess instead of ExecReplace, because
// syscall.Exec does not return and would make every line after the first unreachable at exit 0
// (TASK-091), and no --tty --stdin, because a command that is one of several cannot be handed
// the terminal and kubectl fails outright when asked for a TTY it cannot get.
//
// No arguments are appended, matching LocalRunner: its list branch passes CommandLines to
// ExecSequential and never consults commandArgs, so default_args and argv reach a list on no
// runner. Appending them here would make the same dva.yml behave differently under kubectl.
func (r *KubectlRunner) execEach(env *config.Environment, cmds []string) error {
	for _, args := range r.eachArgs(cmds) {
		if err := dvaexec.ExecSubprocess(env, "kubectl", args, false); err != nil {
			return err
		}
	}
	return nil
}

// eachArgs builds the argv for every command execEach will run, in order.
//
// Separated from the loop for the same reason execArgs was separated from Execute: what went
// wrong here was the *count* of invocations, not the shape of one, and a test that can only see
// the shape of one cannot tell a two-line list from a one-line list. Blank lines are dropped
// here, so a test sees the same skipping the exec does.
func (r *KubectlRunner) eachArgs(cmds []string) [][]string {
	var argvs [][]string
	for _, c := range cmds {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		argvs = append(argvs, r.buildStepArgs(c))
	}
	return argvs
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
// The loop is runStepLoop, shared with the local and compose runners; only the exec differs —
// and that difference is execEach, which a list `command:` runs through too. Steps and a list
// are the same sequence with and without labels, so they had better not drift apart.
func (r *KubectlRunner) executeSteps(env *config.Environment, steps []config.ProvisionItem) error {
	return runStepLoop(env, r.Opts.Config, steps, func(cmds []string) error {
		return r.execEach(env, cmds)
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
