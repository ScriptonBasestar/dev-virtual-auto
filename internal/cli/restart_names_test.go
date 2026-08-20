// Package cli — regression tests for TASK-033.
// restartCmd advertises "[PLAN | SERVICE...]" in its Use string; these tests
// assert that on the legacy (no-plans) path the names actually reach
// lifecycle.UpOptions.Names instead of being discarded.
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeRestartProbeConfig creates a two-entry script stack in a temp dir and
// chdirs into it. Each hook touches a marker file, so "which entries ran" is
// observable without parsing subprocess output.
func writeRestartProbeConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	body := `version: "0.1.44"
environments:
  dev:
    environment:
      PROBE_ENV: dev-value
stack:
  s1:
    order: 1
    script:
      up: touch s1_up
      stop: touch s1_stop
  s2:
    order: 2
    script:
      up: touch s2_up
      stop: touch s2_stop
`
	if err := os.WriteFile(filepath.Join(dir, "dva.yml"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// loadConfig/loadEnv memoize into package globals; without a reset each test
	// would reuse the previous test's (already-removed) temp dir.
	cfg, env = nil, nil
	t.Cleanup(func() {
		os.Chdir(oldWd)
		cfg, env = nil, nil
	})
	return dir
}

func ranMarkers(t *testing.T, dir string) map[string]bool {
	t.Helper()
	got := map[string]bool{}
	for _, m := range []string{"s1_up", "s1_stop", "s2_up", "s2_stop"} {
		if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
			got[m] = true
		}
	}
	return got
}

// TestRestart_ScopesToNamedEntry is the core regression guard: with the names
// discarded into "_", restart bounces every entry, so s2 markers appear here.
func TestRestart_ScopesToNamedEntry(t *testing.T) {
	dir := writeRestartProbeConfig(t)

	if err := restartCmd.RunE(restartCmd, []string{"s1"}); err != nil {
		t.Fatalf("restart s1: %v", err)
	}

	got := ranMarkers(t, dir)
	for _, want := range []string{"s1_stop", "s1_up"} {
		if !got[want] {
			t.Errorf("restart s1: %s did not run, want it to", want)
		}
	}
	for _, unwanted := range []string{"s2_stop", "s2_up"} {
		if got[unwanted] {
			t.Errorf("restart s1: %s ran, but s2 was not named", unwanted)
		}
	}
}

// TestRestart_NoArgsRestartsAll pins the legitimate whole-stack path so the
// scoping fix cannot regress it into a no-op.
func TestRestart_NoArgsRestartsAll(t *testing.T) {
	dir := writeRestartProbeConfig(t)

	if err := restartCmd.RunE(restartCmd, []string{}); err != nil {
		t.Fatalf("restart: %v", err)
	}

	got := ranMarkers(t, dir)
	for _, want := range []string{"s1_stop", "s1_up", "s2_stop", "s2_up"} {
		if !got[want] {
			t.Errorf("restart (no args): %s did not run, want all entries restarted", want)
		}
	}
}

// TestRestart_UnknownNameTouchesNothing matches the 'dva stack up bogus-name'
// reference path: warn, change nothing, exit 0.
func TestRestart_UnknownNameTouchesNothing(t *testing.T) {
	dir := writeRestartProbeConfig(t)

	if err := restartCmd.RunE(restartCmd, []string{"definitely-not-an-entry"}); err != nil {
		t.Fatalf("restart unknown: %v", err)
	}

	if got := ranMarkers(t, dir); len(got) != 0 {
		t.Errorf("restart <unknown>: %v ran, want no entry touched", got)
	}
}

// TestRestart_NameNotConfusedWithFlagValue guards the TASK-027 trap: the value
// of -E must not be treated as an entry name, and naming s1 must still scope.
func TestRestart_NameNotConfusedWithFlagValue(t *testing.T) {
	dir := writeRestartProbeConfig(t)

	if err := restartCmd.RunE(restartCmd, []string{"s1", "-E", "dev"}); err != nil {
		t.Fatalf("restart s1 -E dev: %v", err)
	}

	got := ranMarkers(t, dir)
	if !got["s1_up"] {
		t.Error("restart s1 -E dev: s1_up did not run")
	}
	for _, unwanted := range []string{"s2_stop", "s2_up"} {
		if got[unwanted] {
			t.Errorf("restart s1 -E dev: %s ran; 'dev' must not be read as an entry name", unwanted)
		}
	}
}

// TestRestartRejectsUnknownFlag is the TASK-198 guard. Before it, an unrecognised
// token fell through parseDvaFlags into the service-name list, matched no entry,
// and the empty selection was reported as success: `dva restart --no-wat` exited 0
// having restarted nothing. Measured against the built binary at 8762d15, up/down/
// stop all exited 1 on the same argument — restart was the only one of the four
// lifecycle verbs that did not.
//
// Both halves are asserted. An error alone would also be produced by a command that
// rejected everything, so the run must fail AND leave the stack untouched, and the
// message must name the offending flag rather than merely refusing.
func TestRestartRejectsUnknownFlag(t *testing.T) {
	// --zzznonsense is the nonsense control from the card; the rest are the plausible
	// typos measured beside it, each of which exited 0 before this guard. --no-wait
	// and --var are real flags of this command's PLAN path, and reaching the stack
	// path means the config declares no plans at all, so they are unknown here too.
	for _, flag := range []string{"--zzznonsense", "--no-wat", "--dev", "--docker", "--force", "--no-wait", "--var"} {
		t.Run(flag, func(t *testing.T) {
			dir := writeRestartProbeConfig(t)

			err := restartCmd.RunE(restartCmd, []string{flag})
			if err == nil {
				t.Fatalf("restart %s: exited 0; an unrecognised flag must not be read as a service name", flag)
			}
			if !strings.Contains(err.Error(), "unknown flag") {
				t.Errorf("restart %s: message %q does not say \"unknown flag\"", flag, err)
			}
			if !strings.Contains(err.Error(), flag) {
				t.Errorf("restart %s: message %q does not name the flag the user has to fix", flag, err)
			}
			if got := ranMarkers(t, dir); len(got) != 0 {
				t.Errorf("restart %s: %v ran; a rejected command must touch nothing", flag, got)
			}
		})
	}
}

// TestRestartAcceptsKnownFlagsAfterGuard pins the other direction. rejectUnknownFlags
// fires on ANY dash-prefixed leftover, so the guard is only correct while every flag
// restart honours is consumed by parseDvaFlags before it. A future flag added to the
// command but not to parseDvaFlags would start being refused, and the table above
// cannot see that — it only proves the guard fires.
func TestRestartAcceptsKnownFlagsAfterGuard(t *testing.T) {
	for _, args := range [][]string{
		{"-E", "dev"},
		{"--env", "dev"},
		{"--env=dev"},
		{"--dry-run"},
		{"s1", "--dry-run"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			// parseDvaFlags writes --dry-run into the package global, which outlives the
			// subtest and would silently turn a later test's restart into a no-op.
			saved := dryRun
			t.Cleanup(func() { dryRun = saved })

			writeRestartProbeConfig(t)
			if err := restartCmd.RunE(restartCmd, args); err != nil {
				t.Fatalf("restart %v: %v; this flag is honoured here and must survive the guard", args, err)
			}
		})
	}
}

// TestRestartTerminatorStillNamesEntries is the half of TASK-198 the guard got
// wrong on its first pass. parseDvaFlags keeps `--` on purpose so each command
// can rule on it, and `dva up` rejects a stray one because it takes no positional
// names at all. restart does take them, so `--` is the ordinary way to say the
// next word is a name; checking it unconditionally turned a working invocation
// into `unknown flag "--"`. Measured against the built binary: rc=0 restarting s1
// before the guard, rc=1 with the guard applied to the terminator, rc=0 again
// with it exempt.
//
// Asserting the markers rather than the exit code is deliberate — `restart --`
// with the terminator swallowed as a name also exits 0, having done nothing.
func TestRestartTerminatorStillNamesEntries(t *testing.T) {
	dir := writeRestartProbeConfig(t)

	if err := restartCmd.RunE(restartCmd, []string{"--", "s1"}); err != nil {
		t.Fatalf("restart -- s1: %v", err)
	}

	got := ranMarkers(t, dir)
	for _, want := range []string{"s1_stop", "s1_up"} {
		if !got[want] {
			t.Errorf("restart -- s1: %s did not run; the terminator must not swallow the name", want)
		}
	}
	for _, unwanted := range []string{"s2_stop", "s2_up"} {
		if got[unwanted] {
			t.Errorf("restart -- s1: %s ran, but s2 was not named", unwanted)
		}
	}
}

// TestRestartTerminatorDoesNotDisarmTheGuard pins the other edge of the exemption.
// Only what precedes `--` is checked, so a typo before it must still be caught;
// without this, `dva restart --no-wat --` would be a way to opt out of the guard.
func TestRestartTerminatorDoesNotDisarmTheGuard(t *testing.T) {
	dir := writeRestartProbeConfig(t)

	err := restartCmd.RunE(restartCmd, []string{"--no-wat", "--", "s1"})
	if err == nil {
		t.Fatal("restart --no-wat -- s1: exited 0; a terminator must not disarm the guard for flags before it")
	}
	if !strings.Contains(err.Error(), "unknown flag") || !strings.Contains(err.Error(), "--no-wat") {
		t.Errorf("restart --no-wat -- s1: message %q does not name the flag", err)
	}
	if got := ranMarkers(t, dir); len(got) != 0 {
		t.Errorf("restart --no-wat -- s1: %v ran; a rejected command must touch nothing", got)
	}
}
