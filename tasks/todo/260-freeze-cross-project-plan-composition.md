---
id: TASK-260
title: "Freeze the cross-project plan-composition contract"
type: chore
priority: P0
effort: L
exec-tier: strong
created-at: 2026-09-02T10:12:00+09:00
source: "PLAN-003 composition architecture decision"
scope: "project identity, plan composition semantics, execution and failure contract, compatibility, fixtures, and implementation boundary"
status: todo
needs-human: true
decision-status: pending
depends-on: [TASK-262, TASK-263]
---

# Task 260: freeze cross-project plan composition

## Summary

Use the restored imported-plan contract and approved TASK-263 address/exposure decision to decide whether DVA
should support cross-project plan composition and, if so, freeze a complete contract before any foundational
schema or runtime implementation begins.

## Recommended direction

V1은 root plan이 명시적으로 exposed child plans를 aggregate하는 단방향 모델을 권장한다. Child plan이
parent나 sibling plan을 recursive include하게 하지 않고, root가 전체 DAG와 rollback을 한 번 해석한다.
이는 subproject ownership을 보존하면서 cycle, duplicate ownership과 teardown ambiguity를 줄인다.

Destructive flag는 root에서 명시한 scope 안에서만 child로 전달하고 지원 여부가 다르면 전체 실행 전에
거부한다. Partial failure는 resolved plan과 completed/failed/rolled-back state를 machine-readable하게 남긴다.

## Completion Criteria

- [ ] Compare no composition, declarative plan include, and explicit root aggregation; record the selected model and why the rejected models fail product, operability, or compatibility constraints | verify: human — convenience alone is insufficient to add a second orchestration layer
- [ ] Apply TASK-263's frozen address and exposure contract, then freeze root/child identity, cycle detection, duplicate inclusion, default selection, environment/site/vars merge, entry overrides, `depends_on`, `order`, and resolved-plan immutability | verify: human — every ambiguity must have a fail-closed rule and an accepted/rejected YAML fixture
- [ ] Freeze execution waves, working directories, every lifecycle verb, per-project scope and propagation or rejection of `--no-wait`, `--var`, tag selectors, `--force`, `--volumes`, and `--purge`, readiness, LIFO rollback, cancellation, retry and idempotence | verify: human — destructive flags require explicit scope and confirmation behavior; no child may receive an unsupported flag silently
- [ ] Freeze partial failure, rollback failure with original-error preservation, partial-state reporting, recovery and retry, aggregate status/logs/build behavior, text/JSON output, diagnostics, and exit codes | verify: human — success and failure fixtures must cover at least two projects, a dependency cycle, a failed rollback, and a resumable partial state
- [ ] Define compatibility and migration for existing local plans and imported item names, plus rollback after a failed rollout; do not silently reinterpret an existing valid configuration | verify: human — before/after configuration and invocation examples must be recorded
- [ ] Obtain independent architecture and operability review, append an approved `## Decision Record` to this card, and change `decision-status` from `pending` to `decided`; if composition is selected, create a separate implementation plan with bounded schema, resolver, runtime, CLI, migration, and fixture cards | verify: `make doc-check`

## Non-goals

- No schema, resolver, orchestrator, CLI, or migration implementation in this card.
- No automatic reachability without an approved identity contract.
- No vNext vocabulary decision beyond terminology needed to make this contract unambiguous.
