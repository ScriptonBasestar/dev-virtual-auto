// Package cli — regression tests for TASK-279.
//
// Three plan-route lifecycle flags used to be parsed and then thrown away: restart
// hardcoded Force regardless of what --force said, stop/down accepted --no-wait into a
// struct with no field to hold it, and build's own parseDvaFlags call bound --env/--tag/
// --exclude-tag to _. Every one of them answered exit 0, which reads as "your flag was
// honoured". This file pins the repaired behaviour so a later refactor cannot quietly
// restore any of the three discards.
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// composeRestartFixture is a one-entry compose plan, load-bearing for
// TestPlanRestartHonoursForce: it needs a compose entry so composeUpArgs' Force handling is
// reachable, and --dry-run so the argv is logged instead of requiring a real docker.
func composeRestartFixture(t *testing.T) (*config.Config, *config.Environment) {
	t.Helper()
	c := loadTestConfig(t, `version: "0.1.44"
stack:
  demo:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
plans:
  demo:
    entries:
      - name: demo
`)
	if err := os.WriteFile(filepath.Join(c.FileDir(), "compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}
	return c, config.NewEnvironment(nil, c.FileDir(), c.FileDir())
}

// TestPlanRestartHonoursForce is criterion 1/2: `dva restart <plan> --force` and `dva
// restart <plan>` must be distinguishable, and a user who did not type --force must not get
// --force-recreate anyway.
//
// Measured before the fix: runPlanRestart built lifecycle.UpOptions{Force: true, ...}
// unconditionally, so both cases below logged --force-recreate and this test would have
// failed on the "not passed" case.
func TestPlanRestartHonoursForce(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		wantForce bool
	}{
		{"force passed", []string{"--force", "--dry-run"}, true},
		{"force not passed", []string{"--dry-run"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, e := composeRestartFixture(t)
			logs := useBufferedSlog(t)

			if err := runPlanRestart(c, planEnv(e), "demo", tc.args); err != nil {
				t.Fatalf("runPlanRestart failed: %v", err)
			}

			out := logs.String()
			got := strings.Contains(out, "--force-recreate")
			if got != tc.wantForce {
				t.Errorf("--force-recreate present=%v, want %v (restart %v):\n%s", got, tc.wantForce, tc.args, out)
			}
		})
	}
}

// TestPlanStopDownRejectNoWait is criterion 3: stop/down accept --no-wait into
// StopOptions/DownOptions, which declare no Wait field to receive it. Rather than adding a
// field neither plugin layer ever reads on teardown (confirmed: composeDownArgs and
// composeStopArgs never consult pctx.Wait), the flag is now rejected the same way an
// unsupported plan flag already is.
//
// up/restart are the non-defective control: --no-wait is meaningful there (UpOptions.Wait
// is read by composeUpArgs) and must keep working.
func TestPlanStopDownRejectNoWait(t *testing.T) {
	c, e, _ := purgeFixture(t)

	for _, tc := range []struct {
		verb string
		run  func(*config.Config, *config.Environment, []string) error
	}{
		{"stop", func(c *config.Config, e *config.Environment, a []string) error {
			return runPlanStop(c, planEnv(e), "demo", a)
		}},
		{"down", func(c *config.Config, e *config.Environment, a []string) error {
			return runPlanDown(c, planEnv(e), "demo", a)
		}},
	} {
		t.Run(tc.verb, func(t *testing.T) {
			err := tc.run(c, e, []string{"--no-wait", "--dry-run"})
			if err == nil {
				t.Fatalf("%s --no-wait was accepted and silently absorbed, not rejected", tc.verb)
			}
			if !strings.Contains(err.Error(), "--no-wait") {
				t.Errorf("error must name the flag it rejected, got: %v", err)
			}
		})
	}
}

// TestPlanUpRestartAcceptNoWait is the control half of the above: --no-wait must keep
// working on the two routes where UpOptions.Wait is read.
func TestPlanUpRestartAcceptNoWait(t *testing.T) {
	c, e, _ := purgeFixture(t)

	for _, tc := range []struct {
		verb string
		run  func(*config.Config, *config.Environment, []string) error
	}{
		{"up", func(c *config.Config, e *config.Environment, a []string) error {
			return runPlanUp(c, planEnv(e), "demo", a)
		}},
		{"restart", func(c *config.Config, e *config.Environment, a []string) error {
			return runPlanRestart(c, planEnv(e), "demo", a)
		}},
	} {
		t.Run(tc.verb, func(t *testing.T) {
			if err := tc.run(c, e, []string{"--no-wait", "--dry-run"}); err != nil {
				t.Errorf("%s --no-wait --dry-run failed: %v", tc.verb, err)
			}
		})
	}
}

// TestBuildRejectsSelectorsInsteadOfSilence is criterion 4: --env/--tag/--exclude-tag used
// to be parsed by parseDvaFlags and bound to _, so none of them leaked into docker's argv
// but none of them did anything either — `dva build --exclude-tag app` still built the
// entry tagged app, and `dva build --env prod` stayed silent against a config with no
// environments: where `dva up --env prod` fails with "env 'prod' not found" on the same
// config. The fix rejects all three on this route instead.
func TestBuildRejectsSelectorsInsteadOfSilence(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"env", []string{"--env", "prod"}, "--env"},
		{"tag", []string{"--tag", "app"}, "--tag"},
		{"exclude-tag", []string{"--exclude-tag", "app"}, "--exclude-tag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			argv := composePassthroughFixtureWith(t, buildFixtureYAML)

			err := buildCmd.RunE(buildCmd, tc.args)
			if err == nil {
				t.Fatalf("dva build %v returned nil — the selector was accepted and silently absorbed", tc.args)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error must name the flag it rejected, got: %v", err)
			}
			if !strings.Contains(err.Error(), "does not support") {
				t.Errorf("error must say build does not support the selector, got: %v", err)
			}

			// docker must never be invoked: a rejection that still ran the build would be
			// the same silent-absorption bug wearing a different symptom.
			if got := argv(); len(got) != 0 {
				t.Fatalf("docker was invoked %d time(s) despite the rejected selector: %v", len(got), got)
			}
		})
	}
}
