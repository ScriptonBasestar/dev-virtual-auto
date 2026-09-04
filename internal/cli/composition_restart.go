package cli

import (
	"context"
	"fmt"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
)

// runCompositionRestart restarts every composed child in place, in wave order, per TASK-260
// §4.3's frozen contract: "Wave 순서대로 각 child에 restart — child 내부는 child 자신의
// restart 의미(개별 entry 재시작)를 그대로 쓰고, root는 child 단위로만 순서를 준다." That is
// deliberately NOT a whole-composition Stop followed by a whole-composition Up: this walks
// comp.Entries directly (already resolved in wave, order, name order — the same order
// compositionChildNames documents) and, for each child in turn, stops that one child and
// brings that one child back up, mirroring lifecycle.Orchestrator.Restart's own stop-then-up
// short-circuit but scoped to a single child. lifecycle.CompositionOrchestrator has no
// Restart of its own — TASK-291's frozen surface stays Up/Down/Stop/Status only
// (composition_orchestrator.go) — restart is composed here from the same per-child Up/Stop
// the executor already exposes, not a fifth orchestrator method.
//
// Because each child's restart is independent, one child's failure never rolls back another
// child that already came back up: there is no whole-composition rollback to opt out of, so
// --no-rollback keeps being rejected for this verb by validateCompositionFlagScope the same
// way it always has (TASK-298 gap A).
func runCompositionRestart(c *config.Config, el *envLoad, planName string, extraArgs []string) error {
	comp, err := lifecycle.ResolveCompositionPlan(c, planName)
	if err != nil {
		return err
	}
	flags, err := validateCompositionFlagScope(comp, planName, "restart", extraArgs)
	if err != nil {
		return err
	}

	exec := newCompositionExecutor(c, el, scopedChildName(flags))
	report, runErr := restartCompositionChildren(context.Background(), comp, exec, flags.noWait)
	return renderCompositionReport(report, runErr)
}

// restartCompositionChildren implements runCompositionRestart's per-child loop. Each child
// gets exec.Stop then exec.Up, in that order. A failure at either step marks that child
// "failed" in the report, records a diagnostic sentence explaining the child's resulting
// state (mirroring composition_orchestrator.go's rollback-failure diagnostics, TASK-298 gap
// B), and halts — every not-yet-restarted child (the rest of this wave, and every later
// wave) is left "not_started" rather than attempted. TASK-260 §5.2 step 1 requires this for
// `up`'s wave processing ("Wave N의 child가 실패하면, Wave N 안에서 그 실패한 child 이후의
// child는 시작하지 않는다"), and restart has no per-verb exemption from it: unlike `up`,
// restart has no rollback to unwind an already-succeeded child, so a later wave's child
// would otherwise be restarted against a dependency this call just found broken. A stop
// failure leaves the child's own state ambiguous (it may still be up, or partially torn
// down); an up failure after a successful stop leaves it definitely down, where it was up
// before this call started. Both are exactly the kind of state an operator cannot infer from
// the structured report's per-child State/Error alone, which is why they get a Diagnostics
// sentence instead of only a report row.
func restartCompositionChildren(ctx context.Context, comp *lifecycle.CompositionPlan, exec lifecycle.CompositionChildExecutor, noWait bool) (*lifecycle.CompositionReport, error) {
	report := lifecycle.NewCompositionReportSkeleton(comp, lifecycle.CompositionOutcomeUp)

	fail := func(i int, wrapped error) error {
		report.Children[i].State = lifecycle.ChildStateFailed
		report.Children[i].Error = wrapped.Error()
		report.Outcome = lifecycle.CompositionOutcomeFailed
		report.Error = wrapped.Error()
		return wrapped
	}

	for i, entry := range comp.Entries {
		child := entry.ChildPlan
		label := child.Name

		if err := exec.Stop(ctx, child); err != nil {
			wrapped := fail(i, fmt.Errorf("composed child %q restart: stop failed: %w", label, err))
			diagnostic := fmt.Sprintf("restart of %s: stop failed: %v — %s's state is uncertain (it may still be up or only partially stopped), manual verification required", label, err, label)
			return report, &lifecycle.CompositionError{Err: wrapped, Diagnostics: []string{diagnostic}, Report: report}
		}
		if err := exec.Up(ctx, child); err != nil {
			wrapped := fail(i, fmt.Errorf("composed child %q restart: up failed: %w", label, err))
			diagnostic := fmt.Sprintf("restart of %s: stop succeeded but up failed: %v — %s is now down (it was up before this restart), manual verification required", label, err, label)
			return report, &lifecycle.CompositionError{Err: wrapped, Diagnostics: []string{diagnostic}, Report: report}
		}
		if !noWait {
			if err := exec.WaitReady(ctx, child); err != nil {
				wrapped := fail(i, fmt.Errorf("composed child %q restart: not ready: %w", label, err))
				diagnostic := fmt.Sprintf("restart of %s: up succeeded but its readiness check failed: %v — %s may not be serving yet, manual verification required", label, err, label)
				return report, &lifecycle.CompositionError{Err: wrapped, Diagnostics: []string{diagnostic}, Report: report}
			}
		}
		report.Children[i].State = lifecycle.ChildStateUp
	}

	return report, nil
}
