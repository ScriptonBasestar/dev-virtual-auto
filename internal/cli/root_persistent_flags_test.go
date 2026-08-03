package cli

import "testing"

// TASK-048: DisableFlagParsing commands never let cobra parse root persistent
// --debug / --json. logger.Init runs in PersistentPreRun before RunE, so the
// only fix is a pre-parse of os.Args that sets the globals before Init.

func TestApplyRootPersistentFlagsFromArgs_SetsDebugAndJSON(t *testing.T) {
	origDebug, origJSON := debug, jsonOutput
	t.Cleanup(func() {
		debug = origDebug
		jsonOutput = origJSON
	})

	debug = false
	jsonOutput = false
	applyRootPersistentFlagsFromArgs([]string{"up", "--debug", "--json", "postgres"})

	if !debug {
		t.Error("applyRootPersistentFlagsFromArgs did not set debug from --debug")
	}
	if !jsonOutput {
		t.Error("applyRootPersistentFlagsFromArgs did not set jsonOutput from --json")
	}
}

func TestApplyRootPersistentFlagsFromArgs_AbsentLeavesFalse(t *testing.T) {
	origDebug, origJSON := debug, jsonOutput
	t.Cleanup(func() {
		debug = origDebug
		jsonOutput = origJSON
	})

	debug = false
	jsonOutput = false
	applyRootPersistentFlagsFromArgs([]string{"up", "postgres", "--dry-run"})

	if debug {
		t.Error("debug set without --debug present")
	}
	if jsonOutput {
		t.Error("jsonOutput set without --json present")
	}
}

func TestApplyRootPersistentFlagsFromArgs_DoesNotTouchDryRun(t *testing.T) {
	origDebug, origJSON, origDry := debug, jsonOutput, dryRun
	t.Cleanup(func() {
		debug = origDebug
		jsonOutput = origJSON
		dryRun = origDry
	})

	dryRun = false
	applyRootPersistentFlagsFromArgs([]string{"compose", "up", "--dry-run"})

	if dryRun {
		t.Error("applyRootPersistentFlagsFromArgs must not consume --dry-run (compose passthrough)")
	}
}

// parseDvaFlags must strip --debug/--json so lifecycle commands do not treat
// them as entry/service names (e.g. `dva up --debug`).

func TestParseDvaFlagsConsumesDebugAndJSON(t *testing.T) {
	origDebug, origJSON, origDry := debug, jsonOutput, dryRun
	t.Cleanup(func() {
		debug = origDebug
		jsonOutput = origJSON
		dryRun = origDry
	})

	debug = false
	jsonOutput = false
	dryRun = false
	_, _, _, _, filtered, _ := parseDvaFlags([]string{"--debug", "--json", "postgres"})

	for _, a := range filtered {
		if a == "--debug" || a == "--json" {
			t.Errorf("%s leaked into filtered args; would be read as an entry/service name", a)
		}
	}
	if len(filtered) != 1 || filtered[0] != "postgres" {
		t.Errorf("filtered = %v, want [postgres]", filtered)
	}
}
