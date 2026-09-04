---
id: TASK-297
title: "fix dva status divergent composition implementation and wrong exit code"
type: bug
priority: P2
effort: S
exec-tier: standard
created-at: 2026-09-04T00:00:00+09:00
source: "Independent review of TASK-260 composition status reporting — found while auditing the frozen §5.3/§5.5/§5.6 contract against production code"
scope: "internal/cli/composition_flags.go runCompositionStatus and internal/lifecycle/composition_orchestrator.go CompositionOrchestrator.Status only"
status: done
depends-on: []
---

# Task 297: fix dva status divergent composition implementation and wrong exit code

## Summary

Two independent implementations of the same composition status report (TASK-260 §5.3/§5.5)
exist in this codebase, and they already disagree.

`runCompositionStatus` (`internal/cli/composition_flags.go:535-596`, wired from
`internal/cli/status.go:35` — this is the only path a real `dva status <plan>` invocation
runs) builds its own `compositionStatusReport` and, when a child query succeeds but nothing
in that child is running, sets `cs.State = "not_started"` and — if no child outright failed —
`report.Outcome = "not_started"` (`composition_flags.go:569`, `582`). `"not_started"` is not
a value TASK-260's frozen contract documents anywhere for `Outcome`; the contract only ever
specifies `"up"` and `"failed"` there (see the doc comment at `composition_flags.go:503`,
which itself only lists `"not_started"`/`"up"`/`"failed"` for child *state*, not composition
*outcome*). Worse, `runCompositionStatus` only returns a non-nil error in the `failed` branch
(`composition_flags.go:594`) — the `not_started`-outcome branch falls through to `return nil`
(`composition_flags.go:596`). TASK-260 §5.6 froze "full success is exit 0, everything else is
exit 1" — a composition that is entirely down is not a success, yet this path exits 0 for it.

`CompositionOrchestrator.Status` (`internal/lifecycle/composition_orchestrator.go:325-349`)
is the other implementation of the same report. It seeds `CompositionOutcomeUp` and sets
`report.Outcome = CompositionOutcomeFailed` whenever `!allUp` (`composition_orchestrator.go:347-348`),
which matches the frozen contract's `"up"`/`"failed"` vocabulary and the flat exit-code rule —
its own doc comment even says so: "Outcome is up only when every child is up, matching §5.6's
flat anything-short-of-full-success-is-exit-1" (`composition_orchestrator.go:326-327`). But this
implementation is dead code in production: the only caller found in the tree is its own unit
test, `TestCompositionPartialStateReportShape` at
`internal/lifecycle/composition_orchestrator_test.go:613` (which exercises the success/"up"
case only — it does not cover a fully-down composition). It is never reached from
`internal/cli/status.go` or any other CLI path — `runCompositionStatus` never calls it and has
its own parallel, non-conforming logic instead.

**Concrete failure scenario**: a CI gate written as `dva status release --json` checking
`outcome == "up"`, or simply keying off the process exit status, silently passes on a fully-down
composition — the CLI path returns an out-of-contract `"not_started"` value with exit code 0
instead of the frozen `"failed"`/exit-1 contract that the (unused) lifecycle-package
implementation already gets right.

## Recommended direction

Having two independent implementations of the same reporting shape is itself the root cause
worth fixing here, not just the exit code — whichever fix is chosen should leave exactly one
implementation of this report in production code:

- Either have `runCompositionStatus` delegate to `CompositionOrchestrator.Status` directly
  (reusing its already-correct, already-tested outcome logic instead of re-deriving it), or
  correct `runCompositionStatus` in place to use only the frozen `"up"`/`"failed"` outcome
  vocabulary and the frozen exit-code rule (a composition is `"failed"`/exit-1 whenever it is
  not fully up, whether that's because a query errored or because children are simply not
  running).
- Whichever direction is taken, decide what happens to per-child `"not_started"` state — that
  value is valid at the child-state granularity per the existing doc comment
  (`composition_flags.go:503`) and should likely stay there; this card is only about the
  composition-level `Outcome` field and the resulting process exit code, not about removing
  `"not_started"` from child rows.
- If `CompositionOrchestrator.Status` becomes the live path, confirm its error handling for a
  child query failure (`composition_orchestrator.go:334-338`, `ChildStateFailed`) still surfaces
  the same operator-facing detail (child name, underlying error) that `runCompositionStatus`
  currently prints, so this fix doesn't regress diagnostic output while fixing the exit code.

## Completion Criteria

- [x] `dva status <plan>` on a composition where every child is down (queries succeed, nothing
      running) reports outcome `"failed"` and exits non-zero, not `"not_started"`/exit 0
      | verify: `/usr/bin/grep -Eq '^func TestCompositionStatusExitsNonzeroWhenDown\(' internal/cli/composition_flags_test.go && go test ./internal/cli -count=1`
- [x] Only one implementation of the composition status report remains reachable from
      production code — either `runCompositionStatus` delegates to
      `CompositionOrchestrator.Status`, or `CompositionOrchestrator.Status` is removed/merged
      if the CLI-side fix supersedes it; no two parallel outcome-computation code paths for the
      same report survive | verify: `human — confirm by reading the diff that only one Outcome-computation implementation remains wired to internal/cli/status.go`
- [x] Existing composition status coverage (`TestCompositionStatusReportsFailedChild` and the
      `CompositionOrchestrator.Status` unit tests) still passes after the fix
      | verify: `go test ./internal/cli ./internal/lifecycle -count=1`
- [x] Repository gates pass | verify: `make lint && make test && make test-integration && make commit-check`

## Completion evidence

Took the delegate direction, not the in-place fix: `runCompositionStatus`
(`internal/cli/composition_flags.go`) no longer builds its own report — it now calls
`lifecycle.NewCompositionOrchestrator(comp, newCompositionExecutor(c, el, "")).Status(ctx)` and
renders the result through `renderCompositionReport`, the exact same helper `runCompositionUp`/
`runCompositionDown`/`runCompositionStop`/`runCompositionRestart` already use. This removed the
second implementation entirely: `compositionStatusReport`, `compositionChildStatus`,
`compositionRollbackReport`, `queryCompositionChildStatus`, and `printCompositionStatusText` are
deleted from `composition_flags.go` (also dropped the now-dead `compositionChildPlanPart`
helper). `newCompositionExecutor`'s `PlanChildExecutor.IsUp` already performs the identical
resolve-runtime + "environment inputs incomplete" check `queryCompositionChildStatus` used to do
by hand (`compositionChildEnvironment`, `composition_flags.go`), so the unrunnable-child
diagnostic detail is unchanged.

`CompositionOrchestrator.Status` (`internal/lifecycle/composition_orchestrator.go`) was the
other half of the fix: it previously always returned a nil error, so nothing forced exit 1 even
when it correctly set `Outcome = CompositionOutcomeFailed`. It now returns a non-nil
`*CompositionError` whenever not every child is up — mirroring how `Up`/`teardown` already
signal failure to their caller — carrying either a synthesized "not fully up" message or the
joined per-child query errors when any child was unrunnable.

One intentional behavior change surfaced by unifying the two paths: a local (non-subproject)
composed child's JSON `project` field is now `""` with the full name in `plan` (matching
`lifecycle.splitChildLabel`, the same convention `up`/`down`/`stop`/`restart` already reported),
whereas the old CLI-only status path duplicated the plan name into `project` too. This makes
`status` consistent with every other composition verb instead of being the outlier.

New test: `TestCompositionStatusExitsNonzeroWhenDown`
(`internal/cli/composition_flags_test.go`) pins the exit-code contract directly. Updated the two
`TestCompositionAggregateLogsStatusBuild` subtests that asserted the old `"not_started"`/exit-0
outcome to expect `"failed"`/non-zero instead, and `TestCompositionStatusReportsFailedChild`
(`internal/cli/composition_flags_exec_test.go`) to unmarshal into `lifecycle.CompositionReport`
now that the CLI-local report type is gone.

`internal/cli/composition_flags_test.go` was split into `composition_flags_test.go` (flag
validation, propagation, status/logs/build) and `composition_flags_exec_test.go` (real-execution
up/down/rollback/dry-run tests, `compositionProcessFixtureConfig`) — the repo's file-size gate
started blocking the single file after this change's net line growth; the split carries no
behavior change, same package, tests unchanged apart from the type-reference fixes above.

Worktree: `/Users/archmagece/worktrees/dva/dva/claude__mst__fix__task297-status-divergent-implementation`
Branch: `dev/claude/mst/fix/task297-status-divergent-implementation`

Gates run clean: `make build`, `make lint`, `make test` (race, all packages), `make
test-integration`, `make doc-check`. `go test ./internal/cli ./internal/lifecycle -count=1`
individually green.

**Post-review fix (independent reviewer, verified against a real binary before this fix)**:
`runCompositionStatus` was inheriting `newCompositionExecutor`'s `DryRun` wiring, which exists for
`up`/`down`/`stop`/`restart`'s teardown gate (TASK-291) but made `--dry-run dva status` on a
fully-down composition fabricate `"up"` for every child and outcome `"up"` — the exact false-green
this card exists to eliminate, worse than master's pre-fix `"not_started"`/exit-0 (master at least
reported the true child state, just the wrong outcome/exit code). Fixed by forcing
`DryRun: false` on both `compositionExecutor` instances built for the status path only —
`runPlanStatus`, the single-plan equivalent, never consults dry-run either, so this also removes a
divergence between the two status paths rather than introducing one. Also cleaned up three stale
comments left referencing symbols this same commit deleted
(`compositionStatusReport`/`queryCompositionChildStatus`), in `composition_flags.go` and
`composition_flags_exec_test.go`. Re-ran the full gate suite after — still clean.

## Non-goals

- Not touching `tasks/todo/260-freeze-cross-project-plan-composition.md` — that card is being
  closed separately.
- Not changing per-child `State` vocabulary (`"not_started"`/`"up"`/`"failed"` at the child
  level) — this card is scoped to the composition-level `Outcome` field and process exit code.
- Not changing behavior for `dva status <plan>` on a non-composition (single-project) plan —
  that path is unrelated to this divergence.
