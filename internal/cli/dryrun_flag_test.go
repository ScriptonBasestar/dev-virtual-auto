package cli

import "testing"

// TASK-047: upCmd/stackUpCmd/appUpCmd set DisableFlagParsing, so cobra never
// parses the root persistent --dry-run (root.go). parseDvaFlags is the shared
// seam every one of those commands routes through, so it must consume the flag
// and set the global. Before the fix, --dry-run fell through to `filtered` and
// dryRun stayed false, so `dva up --dry-run` executed for real.
func TestParseDvaFlagsConsumesDryRun(t *testing.T) {
	orig := dryRun
	t.Cleanup(func() { dryRun = orig })

	dryRun = false
	_, _, _, _, filtered := parseDvaFlags([]string{"--dry-run"})

	if !dryRun {
		t.Error("parseDvaFlags did not set dryRun; --dry-run is silently dropped and `dva up --dry-run` executes for real")
	}
	for _, a := range filtered {
		if a == "--dry-run" {
			t.Error("--dry-run leaked into filtered args; it would be read as an entry/service name")
		}
	}
}

// The flag must not be swallowed at the cost of real positional args, and must
// still be recognized alongside the other hand-parsed flags.
func TestParseDvaFlagsDryRunKeepsPositionalArgs(t *testing.T) {
	orig := dryRun
	t.Cleanup(func() { dryRun = orig })

	dryRun = false
	mode, _, _, _, filtered := parseDvaFlags([]string{"-M", "native", "--dry-run", "postgres"})

	if !dryRun {
		t.Error("dryRun not set when mixed with other flags")
	}
	if mode != "native" {
		t.Errorf("mode = %q, want %q", mode, "native")
	}
	if len(filtered) != 1 || filtered[0] != "postgres" {
		t.Errorf("filtered = %v, want [postgres]", filtered)
	}
}

// Negative control: absent the flag, parseDvaFlags must not set dryRun. Without
// this, the test above would pass on a function that hardcoded dryRun = true.
func TestParseDvaFlagsLeavesDryRunUnsetWhenAbsent(t *testing.T) {
	orig := dryRun
	t.Cleanup(func() { dryRun = orig })

	dryRun = false
	if _, _, _, _, _ = parseDvaFlags([]string{"postgres"}); dryRun {
		t.Error("dryRun set to true without --dry-run present")
	}
}
