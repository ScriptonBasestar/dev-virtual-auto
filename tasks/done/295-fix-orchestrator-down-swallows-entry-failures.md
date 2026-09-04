---
id: TASK-295
title: "Fix Orchestrator.Down swallowing per-entry teardown failures"
type: bug
priority: P2
effort: S
exec-tier: standard
created-at: 2026-09-04T00:00:00+09:00
source: "TASK-293 implementation and independent review (impl-task293, review-task293) — found while building composition rollback-failure fixtures"
scope: "internal/lifecycle/orchestrator.go Orchestrator.Down only — both the plain single-project down path and the composition rollback path that calls through it"
status: done
depends-on: []
---

# Task 295: fix Orchestrator.Down swallowing per-entry teardown failures

## Summary

`internal/lifecycle/orchestrator.go`'s `Orchestrator.Down` logs and swallows every per-entry
`plugin.Down` failure, then unconditionally returns `nil`:

```go
if err := plugin.Down(ctx, pctx); err != nil {
    fmt.Fprintf(os.Stderr, "[warn] entry %q down failed: %v\n", entry.Name, err)
    // Continue with other entries — don't abort on single failure during teardown
}
```

Confirmed directly (line numbers approximate, `Down` starts ~line 170): `filterEntries` and
`stopModeProcesses` are the only calls in this function that can return a non-nil error;
`requireSource`'s failure is also warn-and-continue, not returned; the per-entry `plugin.Down` loop
never contributes to the return value at all. So `Orchestrator.Down` cannot report "one or more entries
failed to tear down" to any caller, ever — for any plugin (compose, kubectl, helm, script, process), not
composition-specific.

**Concrete consequence discovered by TASK-293**: `lifecycle.PlanChildExecutor.Down`
(`internal/lifecycle/composition_orchestrator.go`) does nothing but call `orch.Down(...)` and return its
error. `CompositionOrchestrator.Up`'s rollback loop depends entirely on `exec.Down` returning a non-nil
error to mark a child `ChildStateRollbackFailed` (TASK-260 §5.2). That branch is fully implemented and
already unit-tested against a *fake* executor
(`internal/lifecycle/composition_orchestrator_test.go:TestCompositionRollbackFailurePreservesOriginalError`
et al.), but is unreachable through the real production executor: a real `dva up`'s automatic rollback
can never actually report `rollback_failed` for a script/process child today, even though every layer
above `Orchestrator.Down` is correct. TASK-293's fixtures for this scenario had to work around it with a
test-local executor that bypasses only the swallow (`internal/integration/composition_fixture_test.go`,
`realDownExecutor`), documented in-line as a workaround, not a design choice.

The same swallow also means a plain `dva down <plan>` today always exits 0 even when a service
genuinely failed to tear down — silently misreporting teardown success for the single-project path too.

## Recommended direction

Make `Orchestrator.Down` aggregate and return per-entry failures instead of only warning, while
preserving its "don't abort teardown on one entry's failure — keep going" intent (that part is correct
and should stay):

- Keep iterating all filtered entries even when one's `plugin.Down` fails (unchanged).
- Collect the per-entry errors (e.g. `errors.Join`, or a small aggregate error type consistent with how
  `CompositionError` is structured, if a caller needs to inspect *which* entries failed rather than just
  that some did) and return non-nil from `Down` when at least one occurred, instead of always returning
  `nil`.
- Check `Stop` (`internal/lifecycle/orchestrator.go`) for the same pattern — TASK-293's finding was
  scoped to `Down`, but `Stop` also warn-and-continues on `plugin.Stop` failure per the pattern seen
  at the top of `orchestrator.go`; confirm whether it has the same unconditional-nil-return defect before
  deciding whether this card's fix needs to cover both or `Stop` is already correct.

## Completion Criteria

- [x] `Orchestrator.Down` returns a non-nil error when at least one entry's `plugin.Down` fails, while
      still attempting teardown of every other filtered entry (no early abort) | verify: `/usr/bin/grep -Eq '^func TestOrchestratorDownReturnsErrorOnEntryFailure\(' internal/lifecycle/orchestrator_test.go && go test ./internal/lifecycle -count=1`
- [x] `lifecycle.PlanChildExecutor.Down` now genuinely surfaces a real per-entry down failure (no more
      need for `realDownExecutor`-style workarounds) — confirm by pointing
      `internal/integration/composition_fixture_test.go`'s `TestCompositionFixtureRollbackFailurePreservesError`
      and `TestCompositionFixtureResumesAfterRollbackFailure` fixtures at the real `PlanChildExecutor`
      instead of `realDownExecutor` and confirming they still pass | verify: `go test ./internal/integration -tags=integration -count=1`
- [x] `dva down <plan>` exits non-zero when a real entry teardown fails, verified against the real
      binary, not just unit-level | verify: `go test ./internal/cli -run TestPlanDown -count=1 -v`
- [x] Repository gates pass | verify: `make lint && make test && make test-integration && make commit-check`

## Non-goals

- Not changing `Orchestrator.Down`'s "keep tearing down other entries after one fails" behavior — only
  its silent-success return value.
- Not touching `compose`/`kubectl`/`helm` plugin `Down` implementations themselves — this is about the
  orchestrator loop that calls them, not the plugins' own error handling.
- Does not itself require removing `realDownExecutor` from TASK-293's fixture file — that's a natural
  follow-up once this lands, worth doing for cleanliness, but not a blocking requirement of this card.

## Completion evidence

`Orchestrator.Down` and `Orchestrator.Stop` (`internal/lifecycle/orchestrator.go`) now collect each
per-entry `plugin.Down`/`plugin.Stop` failure into a slice and return `errors.Join(...)` at the end,
instead of only warning and unconditionally returning `nil`. The "keep tearing down every other entry"
behavior is unchanged — the failure is appended, not returned early. `Stop` was fixed alongside `Down`
because it carried the identical unconditional-nil-return defect (confirmed by reading it, per this
card's "Recommended direction"), not a scope expansion beyond what this card already asked to check.

`internal/integration/composition_fixture_test.go`'s `realDownExecutor` workaround (documented at
length in TASK-293) is removed: `TestCompositionFixtureRollbackFailurePreservesError` and
`TestCompositionFixtureResumesAfterRollbackFailure` now construct a plain `&lifecycle.PlanChildExecutor{...}`
and observe the real rollback-down failure through production code, confirmed by re-running both under
`-tags=integration`.

New tests: `TestOrchestratorDownReturnsErrorOnEntryFailure` and
`TestOrchestratorStopReturnsErrorOnEntryFailure` (`internal/lifecycle/orchestrator_test.go`),
`TestPlanDownExitsNonZeroOnEntryDownFailure` (`internal/cli/plan_resolution_test.go`, calls
`runPlanDown` directly — the CLI layer whose returned error becomes the process exit code — rather than
spawning a subprocess).

Gates run clean: `make lint && make test && make test-integration` and `go test ./internal/lifecycle`,
`go test ./internal/cli -run TestPlanDown -v`, `go test ./internal/integration -tags=integration -run
TestCompositionFixture -v` individually.
