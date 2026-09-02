---
id: TASK-261
title: "Decide vNext vocabulary and migration commitment"
type: chore
priority: P0
effort: M
exec-tier: strong
created-at: 2026-09-02T10:13:00+09:00
source: "PLAN-003 final vocabulary and migration decision"
scope: "public nouns and namespaces, compatibility strategy, migration tooling, corpus gate, rollback, and follow-up plan"
status: todo
needs-human: true
decision-status: pending
depends-on: [TASK-254, TASK-256, TASK-258, TASK-260]
---

# Task 261: decide vNext vocabulary and migration

## Summary

After incremental route work and composition semantics are known, decide whether DVA keeps its current
vocabulary, introduces compatibility-first aliases, or performs a versioned hard break. This decision does
not implement a vNext schema or namespace.

## Recommended direction

Current-compatible evolution을 권장한다. `stack`, `plans`, `interaction`, `subprojects`는 이미 제품 문서와
schema에서 서로 다른 책임을 가지므로 측정된 이해도 문제 없이 rename하지 않는다. `run`과 top-level
lifecycle verbs도 유지하며 `plan`/`exec`/`tool` namespace를 추가하지 않는다.

필요한 개선은 stable machine command identity, explicit route와 additive alias metadata로 해결하고,
configuration noun이나 route를 hard break하지 않는다. Evidence가 rename 이익을 증명하지 못하면 이
권장안을 최종 결정으로 채택한다.

## Completion Criteria

- [ ] Compare current vocabulary with proposed alternatives for subprojects/projects, plans/targets, interaction/tasks, stack/components, `run`, and any proposed `plan`/`exec`/`tool` namespace using TASK-254, TASK-256, TASK-258, and TASK-260 evidence | verify: human — no noun may be renamed without a mapped product concept, ambiguity analysis, and measured migration cost
- [ ] Choose current-compatible evolution, alias-first migration, or versioned hard break; freeze canonical terms, route examples, configuration keys, compatibility duration, warning channels, and removal gates | verify: human — unspecified terms retain their current contract
- [ ] Define migration tooling, version detection, machine-readable report, pinned consumer corpus, generated documentation ownership, release sequencing, rollback, and support horizon | verify: human — dynamic calls, ignored files, and unavailable repositories must remain explicit findings rather than assumed compatibility
- [ ] Keep reserved-name collisions as hard errors and the current hook ownership model unless a separate approved decision with equivalent safety evidence changes them | verify: human — vocabulary work must not smuggle in collision or execution-hook policy changes
- [ ] Obtain independent product, architecture, and compatibility review, append an approved `## Decision Record` to this card, and change `decision-status` from `pending` to `decided`; if migration is selected, create a new plan with bounded implementation and release cards before closing this task | verify: `make doc-check`

## Fail-closed default

If evidence, compatibility contract, rollback, or human approval is incomplete, retain the current vocabulary
and routes. Do not leave a promised hard-break release date open-ended.
