---
id: TASK-257
title: "Decide the canonical validate route and compatibility"
type: chore
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-02T10:09:00+09:00
source: "PLAN-003 public route compatibility decision"
scope: "validate usage evidence, canonical route, parity, deprecation, rollback, and independent review"
status: todo
needs-human: true
decision-status: pending
depends-on: [TASK-254]
---

# Task 257: decide validate route compatibility

## Summary

Choose whether `config validate`, top-level `validate`, or both are canonical public routes. Current code
shares an implementation while current documentation and skills primarily teach `config validate`; neither
route may be hidden or removed without evidence and an approved migration contract.

## Recommended direction

Documentation은 conceptual owner인 `dva config validate`를 canonical example로 사용하고 top-level
`dva validate`는 visible, behavior-identical shortcut으로 계속 지원하는 방향을 권장한다. 두 route 모두
제거·deprecation 일정은 두지 않는다. 이 선택은 기존 skill과 automation을 보존하면서 frequent command의
discoverability도 유지한다.

## Completion Criteria

- [ ] Build a secret-free invocation corpus from tracked DVA documentation, canonical skills, scripts and pinned consumer repositories; record repository IDs, revisions, scanned paths, literal matches, dynamic-call limitations, and text/JSON automation usage | verify: human — missing or stale evidence stops route removal or hiding
- [ ] Compare `config validate` canonical with top-level compatibility, top-level canonical with `config validate` compatibility, and coequal routes for discoverability, script stability, conceptual grouping, completion, and support cost | verify: human — current implementation sharing is evidence but not by itself a product decision
- [ ] Freeze parity for config discovery, `--strict`, `--fix`, root persistent flags including `--json`, errors, stdout/stderr, exit codes, help, manifest, completion, and any route-specific warnings | verify: human — every allowed difference must be explicit and no nonexistent route-specific flag may be invented
- [ ] Decide whether manifest represents one canonical command with a compatibility route or two coequal routes, including schema versioning and legacy-field meaning; if current schema cannot express the decision, require the bounded child produced from TASK-254 before implementation | verify: human — TASK-258 must not invent route-identity fields ad hoc
- [ ] Freeze canonical documentation route, compatibility visibility, warning channel, minimum support releases, removal evidence gate, and rollback; absence of sufficient evidence keeps both current routes visible and functional | verify: human — deprecation and removal must be separate decisions
- [ ] Obtain independent compatibility review, append an approved `## Decision Record` to this card, and change `decision-status` from `pending` to `decided` before TASK-258 begins | verify: `make doc-check`

## Non-goals

- No route, flag, or validation behavior change.
- No schema or semantic-warning change.
- No evidence-free alias removal or help hiding.
