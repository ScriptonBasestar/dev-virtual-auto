---
id: TASK-291
title: "Implement composition runtime and LIFO rollback"
type: feature
priority: P1
effort: L
exec-tier: strong
created-at: 2026-09-04T10:00:00+09:00
source: "PLAN-005 implementation of TASK-260's frozen composition contract"
scope: "wave-sequential cross-project execution for every lifecycle verb, automatic LIFO rollback with original-error preservation, the --no-rollback opt-out, and partial-state reporting"
status: todo
depends-on: [TASK-290]
---

# Task 291: implement composition runtime and LIFO rollback

## Summary

Execute a resolved `CompositionPlan`'s children sequentially by wave for every lifecycle verb, and
implement the automatic LIFO rollback-on-failure behavior TASK-260 §5.1 explicitly introduces as new
(single-project `Up` at `internal/lifecycle/orchestrator.go:75` has no such rollback today, and this task
must not silently change that single-project behavior while adding it for composition).

## Recommended direction

Add a composition-aware execution path that, for `up`, calls each wave's children in declaration order,
sequentially (TASK-260 §4.1 — wave is an ordering concept, not a concurrency model; do not introduce
parallel execution). Between waves, wait for each just-started child's own readiness/health check unless
`--no-wait` is set (§4.5).

On any child's `up` failure: stop starting further children, then tear down every already-succeeded child
in strict LIFO order via that child's own `down` (plain teardown — never with `--volumes`/`--purge`, per
§5.2). If a rollback `down` itself fails, preserve the original failure as the primary error unchanged
("original-error preservation", §5.2) and record the rollback failure as a secondary diagnostic naming
the still-possibly-up child.

Add the `--no-rollback` flag (TASK-260's "Open question — resolved 2026-09-04": the user approved this
opt-out after the initial draft; TASK-260 §4.4 and §5.1 already specify its contract — propagate-to-all,
skips the automatic rollback entirely when set, leaving whatever succeeded in place for inspection).

`down`/`stop` on a composition plan tear down children in reverse-wave (LIFO) order regardless of how they
got there (§4.3); a manual `down` after a failed automatic rollback re-queries each child's actual state
rather than trusting any cached record (§5.4/§6.3 — "child execution state is the source of truth").

Produce the partial-state report shape TASK-260 §5.3 specifies (`outcome`, `children[].state`,
`rollback.{attempted,succeeded,failed}`, `error`) for both the success and failure paths; reuse it for
`status` on a composition plan (§5.5). Do not add a persisted state file or a `--retry` flag (§5.4, non-goal).

## Completion Criteria

- [ ] `up` on a composition plan executes children sequentially by wave, waits for per-child readiness
  between waves unless `--no-wait`, and never runs two children concurrently even within one wave
  (TASK-260 §4.1, §4.5) | verify: `/usr/bin/grep -Eq '^func TestCompositionUpExecutesWavesSequentially\(' internal/lifecycle/composition_orchestrator_test.go && go test ./internal/lifecycle -count=1`
- [ ] On a mid-wave child failure, every already-succeeded child is torn down via plain `down` in strict
  LIFO order before the command returns; no further child is started (TASK-260 §5.2) | verify: `/usr/bin/grep -Eq '^func TestCompositionUpRollsBackSucceededChildrenOnFailure\(' internal/lifecycle/composition_orchestrator_test.go && go test ./internal/lifecycle -count=1`
- [ ] When a rollback `down` itself fails, the reported primary error is the original failure unchanged,
  and the rollback failure appears only as a secondary diagnostic naming the affected child (TASK-260
  §5.2's "original-error preservation") | verify: `/usr/bin/grep -Eq '^func TestCompositionRollbackFailurePreservesOriginalError\(' internal/lifecycle/composition_orchestrator_test.go && go test ./internal/lifecycle -count=1`
- [ ] `--no-rollback` skips the automatic teardown entirely, leaving succeeded children running, and is
  accepted on every lifecycle verb per the propagate-to-all rule (TASK-260 §4.4, "Open question —
  resolved 2026-09-04") | verify: `/usr/bin/grep -Eq '^func TestCompositionNoRollbackFlagSkipsTeardown\(' internal/lifecycle/composition_orchestrator_test.go && go test ./internal/lifecycle -count=1`
- [ ] `down`/`stop` on a composition plan always tear down in reverse-wave order, and re-invoking `down`
  after a failed automatic rollback re-queries live child state rather than relying on a cached record
  (TASK-260 §4.3, §5.4, §6.3) | verify: `/usr/bin/grep -Eq '^func TestCompositionDownIsLIFOAndReentrant\(' internal/lifecycle/composition_orchestrator_test.go && go test ./internal/lifecycle -count=1`
- [ ] The partial-state report (`outcome`/`children[].state`/`rollback.*`/`error`) matches TASK-260 §5.3's
  shape on both the success and failure paths, and single-project `Up`'s existing no-rollback behavior is
  unchanged by this task (regression fixture required) | verify: `/usr/bin/grep -Eq '^func TestCompositionPartialStateReportShape\(' internal/lifecycle/composition_orchestrator_test.go && go test ./internal/lifecycle -count=1`
- [ ] Repository gates pass | verify: `make lint && make test && make test-integration && make commit-check`

## Non-goals

- No CLI flag parsing/wiring or text/JSON command-level output — that belongs to TASK-292 (this task
  implements the orchestrator-level behavior and report data those layers will call).
- No persisted partial-state file and no `--retry` flag (TASK-260 §5.4).
- No change to single-project `Up`'s existing no-automatic-rollback behavior.
