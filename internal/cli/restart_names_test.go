// Package cli — regression tests for TASK-033.
// restartCmd advertises "[PLAN | SERVICE...]" in its Use string; these tests
// assert that on the legacy (no-plans) path the names actually reach
// lifecycle.UpOptions.Names instead of being discarded.
package cli

import (
	"os"
	"path/filepath"
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
