---
id: TASK-296
title: "Fix composition readiness-gate failure leaving a started child running with no rollback"
type: bug
priority: P1
effort: S
exec-tier: standard
created-at: 2026-09-04T00:00:00+09:00
source: "Independent review of TASK-260 composition rollback implementation (reviewer finding, rated MEDIUM)"
scope: "internal/lifecycle/composition_orchestrator.go CompositionOrchestrator.Up wave-boundary readiness gate only"
status: done
depends-on: []
---

# Task 296: fix composition readiness-gate failure leaving a started child running with no rollback

## Summary

`internal/lifecycle/composition_orchestrator.go`'s `CompositionOrchestrator.Up` deliberately excludes
the readiness-failed child from its own rollback list:

```go
// The failing child keeps state "failed" and is not itself rolled back: TASK-260 §5.2
// rolls back the children that succeeded, and a child's own partial state after a
// failed up stays the child's business (§5.1).
report.Outcome = CompositionOutcomeFailed
report.Error = primary.Error()
report.Children[failedAt].State = ChildStateFailed
report.Children[failedAt].Error = primary.Error()
// A readiness failure lands on a child whose up already succeeded; it is the failure,
// not a rollback target.
succeeded = dropIndex(succeeded, failedAt)
```

Confirmed directly (composition_orchestrator.go:193–235). The wave loop runs two passes per wave: an
`exec.Up` pass that appends `i` to `succeeded` only when `Up` returns nil, then (unless `NoWait`) a
`WaitReady` pass over the same wave. When `WaitReady` fails, `failedAt = i` is set for a child that was
already appended to `succeeded` in the first pass — its `up` genuinely succeeded, only its readiness
check timed out. `dropIndex(succeeded, failedAt)` then removes that index from the rollback list, so the
LIFO rollback loop at line 248 never calls `exec.Down` on it.

For the sibling case — a child whose `exec.Up` itself fails — `failedAt` is set *before* the append to
`succeeded` happens (the loop `break`s first), so that index was never in `succeeded` to begin with;
`dropIndex` is a no-op there. The comment's justification ("not a rollback target") only actually holds
for that case. It is being applied uniformly to a second, distinct case — readiness failure on an
already-started child — where it silently drops a live resource from rollback instead of being a no-op.

TASK-260 §5.2 step 2 requires: "Wave 0..N에서 이미 성공적으로 `up`된 모든 child를 LIFO 순서로 `down`한다"
— every child whose `up` succeeded in the affected waves, with no carve-out for a child whose subsequent
readiness check is what triggered the rollback.

**Concrete failure scenario**: wave 0 = `[api/deploy, web/deploy]`. Both `Up` calls succeed. `web/deploy`'s
`WaitReady` times out. `api/deploy` is torn down (rolled back) via the LIFO loop, but `web/deploy` is left
running with its containers/ports still held — even though `report.Children[web].State` reads `"failed"`,
which everywhere else in the composition status contract (§5.3) means "did not come up," not "came up
and is still running." The operator sees a `"failed"` child in the JSON report with no indication that
live resources exist behind it, and must separately know to run `dva down release` by hand to reclaim
them. This is exactly the "some children may still be up" situation the contract already has a documented
opt-out for — `CompositionUpOptions.NoRollback` (§4.4) exists so an operator can *choose* to preserve
failure state for inspection — except here it happens implicitly, only for readiness failures, regardless
of `NoRollback`.

Also note: the test fake already carries a `readyErr` hook for exactly this
(`internal/lifecycle/composition_orchestrator_test.go:29`, `WaitReady` at line 86), but no existing test
in the file ever sets `f.readyErr[...]` — this path is currently untested.

## Recommended direction

`dropIndex(succeeded, failedAt)` should not run unconditionally for every failure. Per §5.2, rollback's
job is to tear down every child whose `up` succeeded in the affected waves — and a readiness-failed child
qualifies, since it is only in `succeeded` when its `up` returned nil in the first place. Two ways to get
there, either acceptable:

- Stop dropping `failedAt` from `succeeded` at all. For the `exec.Up`-failure case this is already a
  no-op (the index was never appended), so removing the `dropIndex` call changes behavior only for the
  readiness-failure case — which is the one that needs to change. The LIFO rollback loop then naturally
  calls `exec.Down` on the readiness-failed child alongside its wave-mates, and on success its
  `report.Children[idx].State` transitions from `ChildStateFailed` to `ChildStateRolledBack` the same way
  any other rolled-back child's does (the `Error` field the rollback loop leaves untouched on success
  still shows why the rollback happened).
- Or keep `failedAt` distinguished from ordinary `succeeded` entries but still call `exec.Down` on it as
  part of the rollback pass, if the report wants to keep its state visibly different from a plain
  rolled-back sibling (e.g. a fifth `ChildState` for "rolled back after readiness failure"). This is more
  invasive for what looks like a should-be-simple fix — only worth it if reusing `ChildStateRolledBack`
  turns out to erase information a caller relies on.

Either way, update or remove the "not a rollback target" comment — it's currently wrong for the readiness
path and should either be scoped explicitly to the `exec.Up`-failure case or dropped once the code no
longer needs it to justify anything.

## Completion Criteria

- [x] A wave-boundary readiness (`WaitReady`) failure on a child whose `exec.Up` succeeded results in that
      child being included in the LIFO rollback alongside the rest of its wave's succeeded siblings — not
      dropped from the rollback list | verify: `/usr/bin/grep -Eq '^func TestCompositionReadinessFailureRollsBackSucceededSiblings\(' internal/lifecycle/composition_orchestrator_readiness_test.go && go test ./internal/lifecycle -count=1`
      (note: the test landed in a new sibling file `composition_orchestrator_readiness_test.go` rather
      than the existing `composition_orchestrator_test.go` — the repo's file-size hook blocks that file at
      600 code lines and it was already at the limit)
- [x] The existing `exec.Up`-failure rollback behavior is unchanged — a child whose `up` itself failed is
      still not treated as a rollback target (it was never in `succeeded`), confirmed by the pre-existing
      up-failure rollback tests in `composition_orchestrator_test.go` continuing to pass unmodified |
      verify: `go test ./internal/lifecycle -run TestCompositionRollback -count=1`
- [x] The `readyErr` hook on `fakeChildExecutor` (composition_orchestrator_test.go:29) is exercised by at
      least one test, closing the gap noted above | verify: `/usr/bin/grep -Eq 'readyErr\[' internal/lifecycle/composition_orchestrator_test.go`
- [x] Repository gates pass | verify: `make lint && make test && make test-integration && make commit-check`

## Non-goals

- Not changing how an `exec.Up`-failure itself is reported or rolled back — only the readiness-failure
  path, which currently diverges from it incorrectly.
- Not introducing a new `ChildState` unless the simpler fix (reusing `ChildStateRolledBack`) turns out to
  lose information a caller needs — see "Recommended direction" above.
- Not changing `CompositionUpOptions.NoRollback` semantics — a readiness-failed child should still be
  preserved un-rolled-back when the operator explicitly passes `--no-rollback`, same as any other
  succeeded child today.
- Not touching `Orchestrator.Down`'s per-entry error swallowing — that's TASK-295, a separate defect in a
  different function.
