package runner

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// childModeEnv selects a scenario when this test binary re-executes itself. See
// TestComposeStepsRunToCompletion for why a child process is required.
const childModeEnv = "DVA_COMPOSE_STEPS_CHILD"

// composeRunnerWith builds a DockerComposeRunner whose compose command is the given binary.
// `echo` makes each invocation print the argv it was handed, so a step that ran is visible.
// Nothing contacts docker.
func composeRunnerWith(binary string) *DockerComposeRunner {
	return &DockerComposeRunner{
		Cmd: &ResolvedCommand{Service: "app"},
		Opts: RunOptions{Config: &config.Config{
			Stack: map[string]*config.LifecycleEntry{
				"infra": {Compose: &config.ComposePluginConfig{Command: binary}},
			},
		}},
	}
}

// TestComposeStepsRunToCompletion covers TASK-091. execCompose ends in ExecReplace —
// syscall.Exec — which replaces the running process and never returns. executeSteps called it
// from inside two nested loops, so neither the second command of a step nor the second step
// itself was reachable: dva ran the first command and exited 0 without even printing the
// remaining labels.
//
// This test MUST run the runner in a child process, and that is not incidental. An in-process
// version of it is worse than no test at all: when the regression is present, syscall.Exec
// replaces the *test binary*, which then exits with the exec'd command's status 0 — and
// `go test` reports `ok`. Verified: with ExecReplace restored, the in-process form printed
// `ok` while its subtests never reported a single PASS or FAIL line. A test that a regression
// silently converts into a pass is exactly the failure mode this task is about.
//
// Re-executing this binary moves the replacement into a child, so the parent survives to count
// what the child actually managed to print.
func TestComposeStepsRunToCompletion(t *testing.T) {
	if mode := os.Getenv(childModeEnv); mode != "" {
		runComposeStepsChild(mode)
		return
	}

	t.Run("every step runs, not just the first", func(t *testing.T) {
		out := runChild(t, "steps")
		for _, marker := range []string{"STEP-ONE-MARKER", "STEP-TWO-MARKER"} {
			if !strings.Contains(out, marker) {
				t.Errorf("%s never ran; child output was:\n%s", marker, out)
			}
		}
		// The missing label is what made the old behaviour undetectable: the output gave no
		// sign that a second step had ever been configured.
		if !strings.Contains(out, "step two") {
			t.Errorf("the second step's label was not printed; child output was:\n%s", out)
		}
	})

	t.Run("every command of a step runs, not just the first", func(t *testing.T) {
		out := runChild(t, "commands")
		for _, marker := range []string{"FIRST-MARKER", "SECOND-MARKER"} {
			if !strings.Contains(out, marker) {
				t.Errorf("%s never ran; child output was:\n%s", marker, out)
			}
		}
	})

	t.Run("a failing step aborts the sequence", func(t *testing.T) {
		// Safe to run in-process: the binary cannot be resolved, so exec.LookPath fails
		// before any replacement could happen. Running to completion must not mean
		// swallowing failures.
		var err error
		out := captureStdout(t, func() {
			err = composeRunnerWith("dva-absent-compose-binary").executeSteps(
				&config.Environment{},
				[]config.ProvisionItem{
					{Step: "failing step", Run: "does-not-matter"},
					{Step: "must not start", Run: "unreachable"},
				})
		})
		if err == nil {
			t.Fatal("a failing compose step must return an error")
		}
		if !strings.Contains(err.Error(), "failing step") {
			t.Errorf("the error must name the step that failed; got %v", err)
		}
		if strings.Contains(out, "must not start") {
			t.Errorf("the sequence continued past a failure; got %q", out)
		}
	})
}

// runChild re-executes this test binary in the given mode and returns everything it printed.
func runChild(t *testing.T, mode string) string {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestComposeStepsRunToCompletion$")
	cmd.Env = append(os.Environ(), childModeEnv+"="+mode)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child %q exited with %v; output:\n%s", mode, err, out)
	}
	return string(out)
}

// runComposeStepsChild is the child half: it drives executeSteps against `echo` and lets the
// output go to the real stdout, where the parent collects it. If the steps path replaces the
// process, this function never returns and the later markers never print.
func runComposeStepsChild(mode string) {
	env := &config.Environment{}
	r := composeRunnerWith("echo")

	var steps []config.ProvisionItem
	switch mode {
	case "steps":
		steps = []config.ProvisionItem{
			{Step: "step one", Run: "STEP-ONE-MARKER"},
			{Step: "step two", Run: "STEP-TWO-MARKER"},
		}
	case "commands":
		steps = []config.ProvisionItem{
			{Step: "one step, two commands", Run: []any{"FIRST-MARKER", "SECOND-MARKER"}},
		}
	default:
		return
	}
	// The error is the parent's problem to infer from missing markers; failing here would
	// only muddy the output it parses.
	_ = r.executeSteps(env, steps)
}
