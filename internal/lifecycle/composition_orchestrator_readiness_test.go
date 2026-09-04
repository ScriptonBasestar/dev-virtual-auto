package lifecycle

import (
	"context"
	"errors"
	"testing"
)

// TestCompositionReadinessFailureRollsBackSucceededSiblings covers TASK-296: a
// WaitReady failure lands on a child whose exec.Up already succeeded, so it must stay in
// the rollback list alongside the rest of its wave's succeeded siblings instead of being
// dropped as if it were an exec.Up failure.
func TestCompositionReadinessFailureRollsBackSucceededSiblings(t *testing.T) {
	plan := resolveTestComposition(t)

	exec := newFakeChildExecutor()
	boom := errors.New(`entry "b" readiness check failed: timed out waiting for healthy`)
	exec.readyErr["b-plan"] = boom
	orch := newTestCompositionOrchestrator(t, plan, exec)

	report, err := orch.Up(context.Background(), CompositionUpOptions{})
	if err == nil {
		t.Fatal("Up succeeded, want the readiness failure to surface")
	}
	if !errors.Is(err, boom) {
		t.Fatalf("returned error = %v, want the original readiness failure", err)
	}

	// Both wave-0 children's Up calls succeeded; a-plan's wait passes, b-plan's wait
	// fails. Wave 1 (c-plan) never starts. b-plan — the readiness-failed child itself —
	// must still be torn down alongside a-plan, LIFO.
	requireCalls(t, exec.recorded(), []string{
		"up:a-plan", "up:b-plan",
		"wait:a-plan", "wait:b-plan",
		"down:b-plan", "down:a-plan",
	})

	if report.Outcome != CompositionOutcomeFailed {
		t.Fatalf("outcome = %q, want %q", report.Outcome, CompositionOutcomeFailed)
	}
	wantAttempted := []string{"b-plan", "a-plan"}
	requireCalls(t, report.Rollback.Attempted, wantAttempted)
	requireCalls(t, report.Rollback.Succeeded, wantAttempted)
	if len(report.Rollback.Failed) != 0 {
		t.Fatalf("rollback.failed = %v, want empty", report.Rollback.Failed)
	}

	// b-plan transitions from "failed" (set on the readiness failure) to "rolled_back"
	// once its own down succeeds — the same transition any other rolled-back sibling
	// gets. c-plan never started at all.
	wantStates := map[string]string{
		"a-plan": ChildStateRolledBack,
		"b-plan": ChildStateRolledBack,
		"c-plan": ChildStateNotStarted,
	}
	for _, child := range report.Children {
		if got := child.State; got != wantStates[child.Plan] {
			t.Fatalf("child %q state = %q, want %q", child.Plan, got, wantStates[child.Plan])
		}
		if child.Plan == "b-plan" && child.Error != boom.Error() {
			t.Fatalf("b-plan error = %q, want the readiness failure preserved (%q)", child.Error, boom.Error())
		}
	}
}
