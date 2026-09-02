package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

const planResolutionConfig = `version: "0.1.0"
vars:
  G1: a
env_file:
  - .env
stack:
  s1:
    default_runner: script
    runners:
      script:
        up: echo MARKERS1
plans:
  p1:
    vars:
      LOG_LEVEL: debug
    entries:
      - name: s1
`

// captureStreams records stdout and stderr separately, which the shared captureOutput
// helper cannot do — the point of these tests is which stream the resolution lands on,
// so merging them would assert nothing.
func captureStreams(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	oldStdout, oldStderr := os.Stdout, os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout, os.Stderr = wOut, wErr

	done := make(chan struct{})
	var outBuf, errBuf bytes.Buffer
	go func() {
		outBuf.ReadFrom(rOut)
		errBuf.ReadFrom(rErr)
		close(done)
	}()

	fn()

	wOut.Close()
	wErr.Close()
	os.Stdout, os.Stderr = oldStdout, oldStderr
	<-done

	return outBuf.String(), errBuf.String()
}

func planResolutionEnv(t *testing.T, c *config.Config) *config.Environment {
	t.Helper()
	return config.NewEnvironment(nil, c.FileDir(), c.FileDir())
}

// TestPlanUpDryRunPrintsResolution covers the surface decision behind this feature: rather
// than adding an --explain flag, the resolution rides on --dry-run, which already means
// "tell me what would happen instead of doing it". The resolution is that answer.
func TestPlanUpDryRunPrintsResolution(t *testing.T) {
	c := loadTestConfig(t, planResolutionConfig)
	e := planResolutionEnv(t, c)

	_, stderr := captureStreams(t, func() {
		if err := runPlanUp(c, planEnv(e), "p1", []string{"--dry-run"}); err != nil {
			t.Errorf("plan up dry-run failed: %v", err)
		}
	})

	if !strings.Contains(stderr, "Resolution:") {
		t.Fatalf("dry-run should print the resolution, got:\n%s", stderr)
	}
	for _, want := range []string{
		"vars: env_file",
		"vars: global vars",
		`vars: plans."p1".vars`,
		"vars: OS environment",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("resolution missing %q in:\n%s", want, stderr)
		}
	}
}

// TestPlanUpWithoutDryRunStaysQuiet is the other half of that decision. A real 'dva up' is
// not a question about resolution, and printing ten extra lines on every start would be the
// cost that made folding this into --dry-run preferable to printing it unconditionally.
func TestPlanUpWithoutDryRunStaysQuiet(t *testing.T) {
	c := loadTestConfig(t, planResolutionConfig)
	e := planResolutionEnv(t, c)

	stdout, stderr := captureStreams(t, func() {
		if err := runPlanUp(c, planEnv(e), "p1", nil); err != nil {
			t.Errorf("plan up failed: %v", err)
		}
	})

	if strings.Contains(stdout+stderr, "Resolution:") {
		t.Fatalf("a real run should not print the resolution, got:\n%s%s", stdout, stderr)
	}
}

// TestPlanResolutionGoesToStderr keeps --json parseable. TASK-116 fixed the same defect for
// the stack override warning: advisory text on stdout corrupts the machine-readable stream,
// and a reader piping 'dva up p1 --dry-run --json' into a parser would hit it first.
func TestPlanResolutionGoesToStderr(t *testing.T) {
	c := loadTestConfig(t, planResolutionConfig)
	e := planResolutionEnv(t, c)

	oldJSON := jsonOutput
	jsonOutput = true
	t.Cleanup(func() { jsonOutput = oldJSON })

	stdout, stderr := captureStreams(t, func() {
		if err := runPlanUp(c, planEnv(e), "p1", []string{"--dry-run"}); err != nil {
			t.Errorf("plan up dry-run --json failed: %v", err)
		}
	})

	if strings.Contains(stdout, "Resolution:") {
		t.Fatalf("resolution must not reach stdout under --json, got:\n%s", stdout)
	}
	if !strings.Contains(stderr, "Resolution:") {
		t.Fatalf("resolution should still reach stderr under --json, got:\n%s", stderr)
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout), "{") {
		t.Errorf("stdout should carry only the JSON document, got:\n%s", stdout)
	}
}

// TestPlanDownDryRunPrintsResolution checks the wiring reached every plan verb that accepts
// --dry-run, not just 'up'. down/stop/restart resolve the same plan through the same
// function, so a user asking any of them what would happen gets the same answer.
func TestPlanDownDryRunPrintsResolution(t *testing.T) {
	c := loadTestConfig(t, planResolutionConfig)
	e := planResolutionEnv(t, c)

	_, stderr := captureStreams(t, func() {
		if err := runPlanDown(c, planEnv(e), "p1", []string{"--dry-run"}); err != nil {
			t.Errorf("plan down dry-run failed: %v", err)
		}
	})

	if !strings.Contains(stderr, "Resolution:") {
		t.Fatalf("plan down dry-run should print the resolution, got:\n%s", stderr)
	}
}

func TestPlanStopDryRunPrintsResolution(t *testing.T) {
	c := loadTestConfig(t, planResolutionConfig)
	e := planResolutionEnv(t, c)

	_, stderr := captureStreams(t, func() {
		if err := runPlanStop(c, planEnv(e), "p1", []string{"--dry-run"}); err != nil {
			t.Errorf("plan stop dry-run failed: %v", err)
		}
	})

	if !strings.Contains(stderr, "Resolution:") {
		t.Fatalf("plan stop dry-run should print the resolution, got:\n%s", stderr)
	}
}

func TestPlanRestartDryRunPrintsResolution(t *testing.T) {
	c := loadTestConfig(t, planResolutionConfig)
	e := planResolutionEnv(t, c)

	_, stderr := captureStreams(t, func() {
		if err := runPlanRestart(c, planEnv(e), "p1", []string{"--dry-run"}); err != nil {
			t.Errorf("plan restart dry-run failed: %v", err)
		}
	})

	if !strings.Contains(stderr, "Resolution:") {
		t.Fatalf("plan restart dry-run should print the resolution, got:\n%s", stderr)
	}
}
