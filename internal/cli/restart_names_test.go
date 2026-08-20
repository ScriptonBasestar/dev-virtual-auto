// Package cli — regression tests for TASK-033.
// restartCmd advertises "[PLAN | ENTRY...]" in its Use string; these tests
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

// TestRestart_UnknownNameTouchesNothing pins the half of the old behaviour that
// TASK-207 kept: an unmatchable name still touches nothing. What changed is the
// exit code, so both halves are asserted here — "nothing ran" alone passed on
// master too, and by itself cannot tell the new ruling from the old one.
//
// This test used to justify itself by conformance to the `stack` command family's
// bogus-name path — warn, change nothing, exit 0. That family was deleted with the
// applications: section (6710766) and the verb that replaced it rejects the name
// instead, so the rationale outlived its own premise while the test kept passing.
// TASK-207 replaces it: restart now agrees with up/down/stop. The card binds this
// on the deleted command's name being absent from this file, so it is described
// here rather than quoted.
func TestRestart_UnknownNameTouchesNothing(t *testing.T) {
	dir := writeRestartProbeConfig(t)

	err := restartCmd.RunE(restartCmd, []string{"definitely-not-an-entry"})
	if err == nil {
		t.Fatal("restart <unknown>: exited 0; TASK-207 rules an unmatchable name an error")
	}
	if !strings.Contains(err.Error(), "definitely-not-an-entry") {
		t.Errorf("restart <unknown>: message %q does not name the argument", err)
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
			// --dry-run reaches the package global through wrapWithHooks'
			// consumeDryRunFlag (root.go), which strips it before RunE's body runs --
			// not through parseDvaFlags, so those two rows never reach the guard at
			// all. The save/restore is still required either way: the global outlives
			// the subtest and would silently turn a later test's restart into a no-op.
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

// writeRestartPlanProbeConfig is writeRestartProbeConfig plus two plans and no
// default_plan. That combination is the one that disproves the tempting reading
// of the guard above it: it is NOT true that reaching parseDvaFlags means the
// config declares no plans. requirePlanSelection returns nil as soon as
// planRoutingArgs leaves any token behind, and planRoutingArgs strips only
// --debug and --json, so a flag counts as a token left behind. DefaultPlan() is
// "" here (two plans, none named default), so rejectSuppressedDefaultPlan
// returns nil too, and the stack path runs with plans configured.
func writeRestartPlanProbeConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	body := `version: "0.1.44"
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
plans:
  p1:
    entries:
      - name: s1
  p2:
    entries:
      - name: s2
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
	cfg, env = nil, nil
	t.Cleanup(func() {
		os.Chdir(oldWd)
		cfg, env = nil, nil
	})
	return dir
}

// TestRestartRejectsUnknownFlagBesideANameWithPlansConfigured pins the worse
// half of TASK-198, which the card did not record and which only appears once
// a name precedes the typo. `dva restart s1 --zzznonsense` on master exited 0
// having restarted s1 and silently discarded the argument -- not the "does
// nothing" the card describes, but "does something while ignoring what you
// asked for", which no warning reports. The plans-present fixture is deliberate:
// it is the shape that shows the defect is not confined to plan-less configs.
func TestRestartRejectsUnknownFlagBesideANameWithPlansConfigured(t *testing.T) {
	for _, args := range [][]string{
		{"--zzznonsense"},
		{"s1", "--zzznonsense"},
		{"s1", "--no-wait"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			dir := writeRestartPlanProbeConfig(t)
			err := restartCmd.RunE(restartCmd, args)
			if err == nil {
				t.Fatalf("restart %v: exited 0; an unrecognised flag must not be read as a service name, plans or no plans", args)
			}
			if !strings.Contains(err.Error(), "unknown flag") {
				t.Fatalf("restart %v: want an unknown-flag error, got %v", args, err)
			}
			if got := ranMarkers(t, dir); len(got) != 0 {
				t.Fatalf("restart %v: rejected but still ran %v; the guard must refuse before any entry is touched", args, got)
			}
		})
	}
}

// TestRestartPlanRoutingSurvivesTheGuard is the over-rejection axis for the
// plans-present config. The guard sits after detectPlanRoute, so a plan name
// must still route, and a bare stack name must still restart only that entry.
func TestRestartPlanRoutingSurvivesTheGuard(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want []string
	}{
		{args: []string{"p1"}, want: []string{"s1_stop", "s1_up"}},
		{args: []string{"s1"}, want: []string{"s1_stop", "s1_up"}},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			dir := writeRestartPlanProbeConfig(t)
			if err := restartCmd.RunE(restartCmd, tc.args); err != nil {
				t.Fatalf("restart %v: %v", tc.args, err)
			}
			got := ranMarkers(t, dir)
			for _, m := range tc.want {
				if !got[m] {
					t.Fatalf("restart %v: %s did not run; got %v", tc.args, m, got)
				}
			}
			for _, m := range []string{"s2_stop", "s2_up"} {
				if got[m] {
					t.Fatalf("restart %v: %s ran; the selection must not widen to the whole stack", tc.args, m)
				}
			}
		})
	}
}

// writeRestartDefaultPlanProbeConfig is the shape TASK-210 is about: a plan is
// resolvable with no name given. The key is written out rather than implied,
// because the two routes into DefaultPlan() are not interchangeable evidence --
// see writeRestartLonePlanProbeConfig for the other one.
func writeRestartDefaultPlanProbeConfig(t *testing.T) string {
	t.Helper()
	return writeRestartConfigBody(t, `version: "0.1.44"
default_plan: p1
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
plans:
  p1:
    entries:
      - name: s1
  p2:
    entries:
      - name: s2
`)
}

// writeRestartLonePlanProbeConfig reaches the same resolvable default without
// declaring one: Config.DefaultPlan() promotes a lone plan. It is the shape a
// real dva.yml is most likely to have, and the one where a user is least likely
// to expect a "default plan" refusal, having never written the words. Kept
// separate from the explicit fixture so a regression in either promotion path
// fails on its own subtest instead of hiding behind the other.
func writeRestartLonePlanProbeConfig(t *testing.T) string {
	t.Helper()
	return writeRestartConfigBody(t, `version: "0.1.44"
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
plans:
  p1:
    entries:
      - name: s1
`)
}

// writeRestartConfigBody is the chdir-and-reset half of a fixture writer. The
// three writers above it predate this helper and are deliberately left alone:
// each carries extra content of its own, so folding them in would be a rewrite
// rather than a move, and their bodies are what other tests are pinned to.
func writeRestartConfigBody(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
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

// TestRestartTerminatorNamesEntriesUnderAResolvableDefaultPlan covers the half of
// TASK-210 that the bare-terminator test cannot reach. With a resolvable default
// plan, `dva restart -- s1` never reaches detectPlanRoute's name branch as a plan
// name; it falls through to rejectSuppressedDefaultPlan, which on master read
// args[0] == "--", called it a flag, and refused an invocation that names one
// entry as explicitly as any. Differential against the form without the
// terminator, so it pins the identity rather than today's output.
func TestRestartTerminatorNamesEntriesUnderAResolvableDefaultPlan(t *testing.T) {
	fixtures := []struct {
		shape string
		write func(*testing.T) string
	}{
		{"explicit default_plan", writeRestartDefaultPlanProbeConfig},
		{"lone plan promoted to default", writeRestartLonePlanProbeConfig},
	}

	for _, fx := range fixtures {
		t.Run(fx.shape, func(t *testing.T) {
			plainDir := fx.write(t)
			plainErr := restartCmd.RunE(restartCmd, []string{"s1"})
			plainRan := ranMarkers(t, plainDir)

			termDir := fx.write(t)
			termErr := restartCmd.RunE(restartCmd, []string{"--", "s1"})
			termRan := ranMarkers(t, termDir)

			if (plainErr == nil) != (termErr == nil) {
				t.Fatalf("restart -- s1: %v; restart s1: %v; the terminator separates the name, it does not change what the name means", termErr, plainErr)
			}
			for _, m := range []string{"s1_stop", "s1_up", "s2_stop", "s2_up"} {
				if plainRan[m] != termRan[m] {
					t.Errorf("restart -- s1: %s ran=%v; restart s1: ran=%v", m, termRan[m], plainRan[m])
				}
			}
		})
	}
}

// TestRestartBareTerminatorMeansABareRestart is the `--` row of TASK-207's ruling,
// and it inverts what this test asserted under TASK-198 — where it was
// TestRestartBareTerminatorChangesNothing and required rc=0 with nothing run.
// The inversion is the point of having had the test: the behaviour moved because
// a card ruled, and the diff says so out loud.
//
// Under 198 a first draft removed `--` from the name list, and the escalation was
// measured, with this file byte-identical and only compose.go varying:
//
//	restart --   master:      rc=0, nothing restarted
//	restart --   that draft:  rc=0, s1 and s2 both stopped and started
//	restart --   198 shipped: rc=0, nothing restarted
//	restart --   207 (here):  whatever a bare `dva restart` does, per config shape
//
// 207 lands on that draft's behaviour, from the opposite direction. 198's
// objection was sound on its own terms — an empty Names means "every entry" to
// lifecycle, and silently doing MORE is the one direction a guard-tightening
// change must not drift in. What dissolves it is the guard 207 adds beside it:
// with unmatchable names rejected, leaving the token in no longer means "a no-op",
// it means `dva restart -- "$@"` with an empty "$@" exits 1. That is not the
// conservative option, it is a different regression — the idiom `--` exists for,
// broken. Consuming the terminator makes the empty case mean "no names given",
// which is what a bare `dva restart` already means, so the two agree.
//
// The last row used to read "rc=0, s1 and s2 both stopped and started", hardcoded,
// and an adversarial review measured it false: with several plans and no
// default_plan a bare `dva restart` is REFUSED as ambiguous, while `restart --`
// bounced the whole stack — 198's escalation, arriving from the other side. The
// test had measured that divergence and read it as agreement, because it asserted
// an outcome instead of the claim. So it now asserts the claim: same error text or
// same success, and the same markers, as a bare invocation in the same fixture.
// A differential assertion cannot be satisfied by a coincidence the way a literal
// expectation can.
//
// Comparing markers rather than only the exit code is deliberate and inherited:
// swallowing `--` as an unmatchable name would also exit 0, having done nothing.
func TestRestartBareTerminatorMeansABareRestart(t *testing.T) {
	fixtures := []struct {
		shape string
		write func(*testing.T) string
	}{
		{"no plans", writeRestartProbeConfig},
		{"two plans, no default_plan", writeRestartPlanProbeConfig},
		{"explicit default_plan", writeRestartDefaultPlanProbeConfig},
		{"lone plan promoted to default", writeRestartLonePlanProbeConfig},
	}

	for _, fx := range fixtures {
		t.Run(fx.shape, func(t *testing.T) {
			bareDir := fx.write(t)
			bareErr := restartCmd.RunE(restartCmd, nil)
			bareRan := ranMarkers(t, bareDir)

			termDir := fx.write(t)
			termErr := restartCmd.RunE(restartCmd, []string{"--"})
			termRan := ranMarkers(t, termDir)

			switch {
			case bareErr == nil && termErr != nil:
				t.Fatalf("restart --: %v; a bare restart succeeds in this config, so the terminator form must too", termErr)
			case bareErr != nil && termErr == nil:
				t.Fatalf("restart -- exited 0 having run %v, but a bare restart here is refused with %v; `--` must never do more than the bare form is permitted to", termRan, bareErr)
			case bareErr != nil && termErr != nil && bareErr.Error() != termErr.Error():
				t.Fatalf("restart --: %q; a bare restart says %q, and \"no names given\" must be the same refusal", termErr, bareErr)
			}
			for _, m := range []string{"s1_stop", "s1_up", "s2_stop", "s2_up"} {
				if bareRan[m] != termRan[m] {
					t.Errorf("restart --: %s ran=%v; a bare restart ran=%v", m, termRan[m], bareRan[m])
				}
			}
		})
	}
}

// TestRestartUnknownNameRuling is TASK-207's ruling, in one table, for all four
// tokens the card enumerates. They reach the same rc=0-nothing-done outcome by
// different routes, so ruling on them separately would have been four chances to
// answer the same question four ways.
//
// Both fixtures are exercised because the stack path is NOT the plan-less path:
// detectPlanRoute returns ok=false for "no plans" and for "several plans, none
// selected" alike, and requirePlanSelection lets the second through as soon as
// planRoutingArgs leaves any token behind. Measuring only the plain fixture would
// leave the ruling's reach unmeasured on the config shape most users have.
func TestRestartUnknownNameRuling(t *testing.T) {
	fixtures := []struct {
		shape          string
		write          func(*testing.T) string
		hasDefaultPlan bool
	}{
		{"no plans", writeRestartProbeConfig, false},
		{"two plans, no default_plan", writeRestartPlanProbeConfig, false},
		{"explicit default_plan", writeRestartDefaultPlanProbeConfig, true},
		{"lone plan promoted to default", writeRestartLonePlanProbeConfig, true},
	}

	// hint pins the explanation, not just the quoted token, wherever the token alone
	// would not discriminate. `strings.Contains(err, "-")` is true of most messages
	// this command can emit — including rejectSuppressedDefaultPlan's — so the dash
	// rows would have passed on the wrong error the moment a fixture with a
	// default_plan is added. The row asserts the guard's own words instead.
	//
	// The `--` row is not here: `restart --` is empty-name-list, whose whole meaning
	// is "the same as a bare restart", and that is a claim about two invocations
	// rather than about one. TestRestartBareTerminatorMeansABareRestart owns it.
	//
	// The two default-plan fixtures were added by TASK-210, and they are what makes the
	// hint column earn its keep: before that fix the two terminator rows were answered by
	// rejectSuppressedDefaultPlan, whose message ECHOES the args and so quotes `--no-wat`
	// just as the name guard does. The token alone said the rows passed. Only the hint
	// separates "this argument names no entry" from "you wrote a flag, which suppressed
	// the default plan" — and the user wrote no flag.
	//
	// hintUnderDefaultPlan is the one row where the ruling is still config-shape-dependent,
	// pinned rather than skipped so it fails loudly when that is settled. `-` is too short
	// for rejectUnknownFlags (len < 2) but not for rejectSuppressedDefaultPlan's dash test,
	// so where a default plan resolves it is refused as a flag instead of reported as an
	// unmatchable name. TASK-218 owns it; aligning the two guards here instead would have
	// re-opened TASK-087's hole, where an unrecognised token loses its effect and the command
	// runs anyway — in the two default-plan fixtures, which are the only ones where that guard
	// fires. What is measured is the selection: `dva up -` reaches `[lifecycle] <entry>` with
	// no plan line. The exit code is not, because the fixtures run without a reachable docker
	// and every row ends rc=1 there; rc=0 is TASK-087's report, not this harness's. The hole is
	// not merely hypothetical elsewhere: with no default plan to resolve, nothing catches a
	// lone dash, and `dva up -` already takes the whole-stack path in all four remaining
	// fixtures — in one of them a bare `dva up` refuses outright. TASK-218 carries the
	// measurement; the point here is only that copying one guard's rule into the other
	// trades a wrong message for a wrong action.
	cases := []struct {
		what                 string
		args                 []string
		names                string // the token the message must quote; "" when the call succeeds
		hint                 string // wording the message must carry; "" when the token suffices
		hintUnderDefaultPlan string // replaces hint where a default plan resolves; "" means no divergence
		wantRan              []string
	}{
		{"a typo'd entry name", []string{"zzznosuchservice"}, "zzznosuchservice", "", "", nil},
		{"a typo'd name beside a real one", []string{"s1", "zzznosuchservice"}, "zzznosuchservice", "", "", nil},
		{"a bare dash, too short for the flag guard", []string{"-"}, "-", "too short to be a flag", "flags suppress the default plan", nil},
		{"a flag after the terminator", []string{"--", "--no-wat", "s1"}, "--no-wat", "every argument is a name", "", nil},
		{"a second terminator, an ordinary word", []string{"--", "--", "s1"}, "--", "only the first", "", nil},
		{"a real name after the terminator", []string{"--", "s1"}, "", "", "", []string{"s1_stop", "s1_up"}},
	}

	for _, fx := range fixtures {
		for _, tc := range cases {
			t.Run(fx.shape+"/"+tc.what, func(t *testing.T) {
				dir := fx.write(t)
				err := restartCmd.RunE(restartCmd, tc.args)

				wantHint := tc.hint
				if fx.hasDefaultPlan && tc.hintUnderDefaultPlan != "" {
					wantHint = tc.hintUnderDefaultPlan
				}

				if tc.names != "" {
					if err == nil {
						t.Fatalf("restart %v: exited 0; TASK-207 rules an unmatchable name an error", tc.args)
					}
					if !strings.Contains(err.Error(), tc.names) {
						t.Errorf("restart %v: message %q does not quote %q, so it cannot tell the user which argument was wrong", tc.args, err, tc.names)
					}
					if wantHint != "" && !strings.Contains(err.Error(), wantHint) {
						t.Errorf("restart %v: message %q lacks %q, so this row cannot tell the guard's error from any other error mentioning the same token", tc.args, err, wantHint)
					}
				} else if err != nil {
					t.Fatalf("restart %v: %v; this selection is legitimate and must survive the guard", tc.args, err)
				}

				got := ranMarkers(t, dir)
				want := map[string]bool{}
				for _, m := range tc.wantRan {
					want[m] = true
				}
				// Asserted as an exact set, not a subset. "s1 ran" is true of the
				// defect the card calls its worst row (`restart -- --no-wat s1`
				// restarted s1 and discarded the typo), so a test that only checks
				// what SHOULD run would pass on the behaviour being fixed.
				for _, m := range []string{"s1_stop", "s1_up", "s2_stop", "s2_up"} {
					if want[m] && !got[m] {
						t.Errorf("restart %v: %s did not run, want it to (ran %v)", tc.args, m, got)
					}
					if !want[m] && got[m] {
						t.Errorf("restart %v: %s ran, want it untouched (ran %v)", tc.args, m, got)
					}
				}
			})
		}
	}
}

func writeRestartTaggedPlanProbeConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	body := `version: "0.1.44"
stack:
  s1:
    order: 1
    tags: [web]
    script:
      up: touch s1_up
      stop: touch s1_stop
  s2:
    order: 2
    tags: [db]
    script:
      up: touch s2_up
      stop: touch s2_stop
plans:
  p1:
    entries:
      - name: s1
  p2:
    entries:
      - name: s2
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
	cfg, env = nil, nil
	t.Cleanup(func() {
		os.Chdir(oldWd)
		cfg, env = nil, nil
	})
	return dir
}

// TestRestartStackFlagsDoNotNeedAPlanName separates the two invocations that both
// arrive at an empty name list, in one config where the difference is visible:
// several plans, no default_plan, so the plan gate has something to refuse.
//
// `dva restart --` means "no names given" and is refused, exactly as a bare
// `dva restart` is. `dva restart --tag web` also leaves zero names behind, and is
// NOT the same statement — a selector was given, and the raw-args gate has already
// ruled that a surviving token means "do not ask for a plan". A first version of
// the terminator fix gated on the empty list alone and refused both: measured,
// `restart --tag web` went from rc=0 bouncing s1 to rc=1 "multiple plans
// configured", while `up --tag web` and `stop --tag web` were untouched. This test
// is what makes that regression visible, so it asserts the two shapes in the same
// fixture rather than checking either alone.
func TestRestartStackFlagsDoNotNeedAPlanName(t *testing.T) {
	selectors := []struct {
		what string
		args []string
		ran  []string
	}{
		{"a tag selector", []string{"--tag", "web"}, []string{"s1_stop", "s1_up"}},
		{"its short form", []string{"-T", "web"}, []string{"s1_stop", "s1_up"}},
		{"an attached value", []string{"--tag=web"}, []string{"s1_stop", "s1_up"}},
		{"an exclusion", []string{"--exclude-tag", "db"}, []string{"s1_stop", "s1_up"}},
	}
	for _, tc := range selectors {
		t.Run(tc.what, func(t *testing.T) {
			dir := writeRestartTaggedPlanProbeConfig(t)
			if err := restartCmd.RunE(restartCmd, tc.args); err != nil {
				t.Fatalf("restart %v: %v; a selector is a token, and the raw-args gate lets a token through — refusing here would make restart the only lifecycle verb whose tag filter needs a plan name", tc.args, err)
			}
			got := ranMarkers(t, dir)
			want := map[string]bool{}
			for _, m := range tc.ran {
				want[m] = true
			}
			for _, m := range []string{"s1_stop", "s1_up", "s2_stop", "s2_up"} {
				if want[m] != got[m] {
					t.Errorf("restart %v: %s ran=%v, want %v (ran %v)", tc.args, m, got[m], want[m], got)
				}
			}
		})
	}

	// The control. Same fixture, same empty name list, opposite ruling — without
	// this row the test above would also pass on a build with no gate at all.
	t.Run("but the bare terminator still is a missing name", func(t *testing.T) {
		dir := writeRestartTaggedPlanProbeConfig(t)
		err := restartCmd.RunE(restartCmd, []string{"--"})
		if err == nil {
			t.Fatalf("restart --: exited 0 having run %v; with several plans and no default_plan a bare restart is refused, and `--` says the same thing", ranMarkers(t, dir))
		}
		if ran := ranMarkers(t, dir); len(ran) != 0 {
			t.Errorf("restart --: refused with %v but ran %v; a refusal must not act first", err, ran)
		}
	})
}
