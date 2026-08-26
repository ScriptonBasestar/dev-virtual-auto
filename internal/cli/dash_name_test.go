package cli

import (
	"strings"
	"testing"
)

// TestDashPredicatesDisagreeOnPurpose keeps the historical name that made the former
// disagreement visible, and now pins the product ruling that removed it.
//
// isFlag (root.go) classifies the COMMAND slot: Execute asks it whether os.Args[1]
// should be resolved as an interaction before prepending run without reordering argv.
// isFlagToken classifies the PLAN-NAME and ENTRY-NAME slots, where the guards decide
// whether to report a token or step aside.
//
// A lone dash is a supported interaction and entry name. It must therefore be a non-flag in
// every name-bearing slot. TASK-223 removed the flags-first rewrite that turned `dva greet -`
// into `dva run - greet` and adopted isFlagToken's len>1 rule for isFlag; this test ensures a
// later local rewrite cannot reintroduce the predicate disagreement behind that defect.
func TestDashPredicatesDisagreeOnPurpose(t *testing.T) {
	if isFlag("-") {
		t.Errorf("isFlag(\"-\") = true; a lone dash is a supported name, so Execute must not sort it ahead of an interaction")
	}
	if isFlagToken("-") {
		t.Errorf(`isFlagToken("-") = true; a lone dash names nothing, and calling it a flag stands the name guards down -- the TASK-218 defect`)
	}

	// Every token the two predicates ever see must get the same answer from both. Without
	// this the historical disagreement could spread again token by token.
	for _, tok := range []string{"", "--", "-v", "-M", "--debug", "--var", "-x=1", "up", "s1", "KEY=value"} {
		if isFlag(tok) != isFlagToken(tok) {
			t.Errorf("isFlag(%q)=%v but isFlagToken(%q)=%v; name-bearing slots must share one flag rule", tok, isFlag(tok), tok, isFlagToken(tok))
		}
	}
}

// TestDefaultPlanGuardDoesNotCallALoneDashAFlag binds rejectSuppressedDefaultPlan.
//
// Its message asserts something about the invocation -- that the user wrote a flag, and
// that the flag is why the default plan did not resolve. With a lone dash both halves are
// false, and the guard emitted it only where a default plan existed, so the same token got
// two different accounts depending on a config key the user may never have written.
//
// The fixture must have a resolvable default plan: without one the guard returns before
// the dash test and this test would pass on an untouched code path.
func TestDefaultPlanGuardDoesNotCallALoneDashAFlag(t *testing.T) {
	writeRestartDefaultPlanProbeConfig(t)
	err := statusCmd.RunE(statusCmd, []string{"-"})

	if err == nil {
		t.Fatalf(`status -: exited 0; a lone dash names no plan and must be reported, not run`)
	}
	if strings.Contains(err.Error(), "flags suppress the default plan") {
		t.Fatalf("status -: %q; the user wrote no flag, so this account of the invocation is false", err)
	}
	if !strings.Contains(err.Error(), "plan '-' not found") {
		t.Fatalf("status -: %q; with this guard standing aside the name guard must be the one that answers", err)
	}
}

// TestUnknownPlanGuardReportsALoneDash binds rejectUnknownPlanArg, in the config shape
// where it is the only guard that can fire: plans exist, so the no-plans early return is
// not taken, and no default plan resolves, so rejectSuppressedDefaultPlan returns first.
//
// detectPlanRoute has never had a dash test -- it looks args[0] up in c.Plans and finds
// nothing -- so before this change the router and the guard reading the same slot gave a
// lone dash opposite readings. This pins them to one.
func TestUnknownPlanGuardReportsALoneDash(t *testing.T) {
	writeRestartPlanProbeConfig(t)
	err := statusCmd.RunE(statusCmd, []string{"-"})

	if err == nil {
		t.Fatalf(`status -: exited 0 and reported the whole stack; detectPlanRoute already read "-" as a name matching no plan, and the guard must agree`)
	}
	if !strings.Contains(err.Error(), "plan '-' not found") {
		t.Fatalf("status -: %q; want the unmatched-plan-name refusal", err)
	}
}

// TestUpPositionalGuardReportsALoneDashWithNoPlans binds rejectUpPositionalArg, and is
// the row where the defect was an ACTION rather than a wording.
//
// With no plans configured `dva up -` took the whole-stack path and started every declared
// entry, reporting success. The marker assertion is the point of the test: an error string
// alone would not distinguish "refused" from "started everything and then failed".
func TestUpPositionalGuardReportsALoneDashWithNoPlans(t *testing.T) {
	dir := writeRestartProbeConfig(t)
	err := upCmd.RunE(upCmd, []string{"-"})

	if err == nil {
		t.Fatalf(`up -: exited 0; a lone dash is a name that matches no entry, and up takes no positional argument at all here`)
	}
	if !strings.Contains(err.Error(), "unexpected argument '-'") {
		t.Fatalf("up -: %q; want the positional-argument refusal, which is the only guard that can fire in a plan-less config", err)
	}
	if got := ranMarkers(t, dir); len(got) != 0 {
		t.Fatalf("up -: refused with %v but still ran %v; the guard must stop the command before any entry is touched", err, got)
	}
}
