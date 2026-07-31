package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// kubectlChildModeEnv selects a scenario when this test binary re-executes itself. See
// TestKubectlStepsRunToCompletion for why a child process is required.
const kubectlChildModeEnv = "DVA_KUBECTL_STEPS_CHILD"

// kubectlShim puts a fake `kubectl` first on PATH and returns an accessor for the argv of every
// invocation it received.
//
// This is not a convenience. `kubectl` on a developer machine points at a real cluster, so a test
// that reached the real binary would run the fixture's commands against it. Two guards make that
// impossible rather than unlikely:
//
//   - PATH is rebuilt from scratch as shim:/bin:/usr/bin rather than prepended to the caller's,
//     so the directories that actually hold kubectl are not on it at all.
//   - exec.LookPath is then asked where `kubectl` resolves, and the test aborts unless the answer
//     is the shim. A shim that failed to install stops the test instead of falling through.
//
// /bin and /usr/bin stay on PATH because the local and compose runners need `sh`, `echo` and
// `true`; neither directory contains a kubectl.
func kubectlShim(t *testing.T) func() []string {
	t.Helper()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "invocations.log")
	shimPath := filepath.Join(dir, "kubectl")

	// Echoes what it was called with so the step output is observable, and appends the same argv
	// to a log so the *number* of invocations can be counted — one per step is the property
	// TASK-094 turns on.
	//
	// The exit 97 is load-bearing. The single-command path ends in ExecReplace → syscall.Exec, so
	// a KubectlRunner that fails to dispatch to steps REPLACES whatever process called it; the
	// shim's exit status then becomes that process's. Failing on the exact argv the defect
	// produced — a trailing bare `--`, meaning nothing was passed to exec — is what lets the
	// parent in TestKubectlStepsRunToCompletion see that its child was substituted.
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> " + logPath + "\n" +
		"case \"$*\" in *' --') " +
		"echo 'shim: kubectl exec with no command after --' >&2; exit 97 ;; esac\n" +
		"printf 'kubectl %s\\n' \"$*\"\n"
	if err := os.WriteFile(shimPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write kubectl shim: %v", err)
	}

	t.Setenv("PATH", dir+":/bin:/usr/bin")

	resolved, err := exec.LookPath("kubectl")
	if err != nil {
		t.Fatalf("kubectl does not resolve under the test PATH: %v", err)
	}
	if resolved != shimPath {
		t.Fatalf("kubectl resolves to %q, not the shim %q — refusing to run against a real cluster", resolved, shimPath)
	}

	return func() []string {
		data, err := os.ReadFile(logPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			t.Fatalf("read shim log: %v", err)
		}
		return strings.FieldsFunc(strings.TrimRight(string(data), "\n"), func(r rune) bool { return r == '\n' })
	}
}

// TestKubectlStepsRunToCompletion covers TASK-094. KubectlRunner.Execute had no `Steps` branch at
// all — the whole file contained zero references to the field. A config with `pod:` and `steps:`
// but no `command:` fell through to ExecReplace with nothing appended after `--`, so the process
// became `kubectl exec <pod> --` and every declared step was discarded. dva printed nothing about
// them and `dva validate` had already exited 0.
//
// This test MUST run the runner in a child process, for the same reason
// TestComposeStepsRunToCompletion must: both regressions it guards against end in syscall.Exec,
// which replaces the *test binary* rather than returning. Measured — an in-process version of
// these two scenarios reported `ok` against a deliberately reverted fix, because the substituted
// shim exited 0 and `go test` read that as the test binary's own success. A test a regression
// converts into a pass is worse than no test.
//
// Re-executing this binary moves the replacement into a child, so the parent survives to judge
// what the child actually managed to print and what status it exited with.
func TestKubectlStepsRunToCompletion(t *testing.T) {
	if mode := os.Getenv(kubectlChildModeEnv); mode != "" {
		// The child installs its own shim; the parent's PATH is not inherited through the
		// re-exec in a form that would help, and the guards in kubectlShim must apply on both
		// sides of the fork.
		kubectlShim(t)
		runKubectlStepsChild(mode)
		return
	}

	t.Run("every step runs, not just the first", func(t *testing.T) {
		out := runKubectlChild(t, "steps")
		for _, marker := range []string{"STEP-ONE-MARKER", "STEP-TWO-MARKER", "STEP-THREE-MARKER"} {
			if !strings.Contains(out, marker) {
				t.Errorf("%s never ran; child output was:\n%s", marker, out)
			}
		}
		// One kubectl exec per step, not one exec carrying all of them and not one exec total.
		// The count is logged rather than only asserted: a bare pass cannot distinguish "three
		// steps ran" from "the assertion never executed", and this whole test exists because
		// that distinction was invisible once before.
		got := strings.Count(out, "kubectl exec")
		t.Logf("kubectl invocations observed in the child: %d", got)
		if got != 3 {
			t.Errorf("kubectl invoked %d times, want 3 — one per step; child output was:\n%s", got, out)
		}
		// The missing label is what makes a dropped step undetectable in the field: the output
		// gives no sign that a later step was ever configured.
		for _, label := range []string{"step one", "step two", "step three"} {
			if !strings.Contains(out, label) {
				t.Errorf("the label %q was not printed; child output was:\n%s", label, out)
			}
		}
	})

	t.Run("Execute dispatches to steps rather than exec'ing an empty command", func(t *testing.T) {
		// Through Execute, not executeSteps: the missing branch was in Execute, so entering by
		// the front door is what proves the dispatch happens. Without the branch the child is
		// replaced by `kubectl exec <pod> --`, the shim exits 97, and runKubectlChild fails on
		// the status before any assertion below is reached.
		out := runKubectlChild(t, "execute")
		if !strings.Contains(out, "STEP-ONE-MARKER") {
			t.Errorf("the step did not run through Execute; child output was:\n%s", out)
		}
		for _, line := range strings.Split(out, "\n") {
			if strings.HasSuffix(strings.TrimSpace(line), "--") {
				t.Errorf("argv ends in a bare separator: %q", line)
			}
		}
		// The interactive path's flags have no place on a scripted step: there is no terminal,
		// and kubectl fails outright when asked for a TTY it cannot get.
		if strings.Contains(out, "--tty") || strings.Contains(out, "--stdin") {
			t.Errorf("a step must not request a TTY; child output was:\n%s", out)
		}
	})
}

// runKubectlChild re-executes this test binary in the given mode and returns everything it
// printed. A non-zero exit is a failure in its own right: it is how the shim reports that it was
// handed an empty command, which only happens when the process was replaced.
func runKubectlChild(t *testing.T, mode string) string {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestKubectlStepsRunToCompletion$")
	cmd.Env = append(os.Environ(), kubectlChildModeEnv+"="+mode)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child %q exited with %v — the steps path replaced the process instead of returning; output:\n%s", mode, err, out)
	}
	return string(out)
}

// runKubectlStepsChild is the child half: it drives the kubectl runner against the shim and lets
// the output go to the real stdout, where the parent collects it. If the steps path replaces the
// process, this function never returns and the later markers never print.
func runKubectlStepsChild(mode string) {
	env := &config.Environment{}
	r := &KubectlRunner{
		Cmd:  &ResolvedCommand{Pod: "myapp-0"},
		Opts: RunOptions{Config: composeConfig("echo")},
	}

	switch mode {
	case "steps":
		// The error is the parent's problem to infer from missing markers; failing here would
		// only muddy the output it parses.
		_ = r.executeSteps(env, []config.ProvisionItem{
			{Step: "step one", Run: "STEP-ONE-MARKER"},
			{Step: "step two", Run: "STEP-TWO-MARKER"},
			{Step: "step three", Run: "STEP-THREE-MARKER"},
		})
	case "execute":
		r.Cmd.Steps = []config.ProvisionItem{{Step: "step one", Run: "STEP-ONE-MARKER"}}
		_ = r.Execute(env)
	}
}

// TestKubectlStepsAddressTheContainer covers the `pod:container` form, which parsePod already
// understood on the single-command path and which the new step path must not drop.
//
// Safe in-process: it asserts on argv shape, and the regressions it would catch do not replace
// the process the way the two scenarios above do.
func TestKubectlStepsAddressTheContainer(t *testing.T) {
	invocations := kubectlShim(t)

	r := &KubectlRunner{
		Cmd:  &ResolvedCommand{Pod: "myapp-0:sidecar"},
		Opts: RunOptions{Config: composeConfig("echo")},
	}
	_ = captureStdout(t, func() {
		if err := r.executeSteps(&config.Environment{}, []config.ProvisionItem{{Step: "s", Run: "true"}}); err != nil {
			t.Fatalf("executeSteps: %v", err)
		}
	})

	got := invocations()
	t.Logf("shim invocations (%d): %v", len(got), got)
	if len(got) != 1 {
		t.Fatalf("shim invoked %d times, want 1", len(got))
	}
	if !strings.Contains(got[0], "exec --container sidecar myapp-0 --") {
		t.Errorf("argv = %q, want --container sidecar with the container suffix stripped from the pod", got[0])
	}
}
