---
id: TASK-255
title: "Decide the kubectl canonical route and ktl compatibility"
type: chore
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-02T10:07:00+09:00
source: "PLAN-003 public route compatibility decision"
scope: "usage evidence, route naming, alias and reservation behavior, deprecation, rollback, and independent review"
status: todo
needs-human: true
decision-status: pending
---

# Task 255: decide kubectl route compatibility

## Summary

Choose whether `ktl` remains canonical, `kubectl` becomes canonical with `ktl` compatibility, or the route
remains unchanged for lack of sufficient evidence. Do not register a new top-level name until this card is
approved.

## Recommended direction

충돌 corpus가 green일 때만 `kubectl`을 canonical로 추가하고 `ktl`은 visible compatibility route로
유지하는 방향을 권장한다. 제거 날짜는 미리 약속하지 않고, 실제 사용이 0이며 rollback support가 끝난
별도 release decision에서만 검토한다. Evidence가 불완전하면 현재 `ktl` 하나를 유지한다.

## Completion Criteria

- [ ] Build a secret-free invocation corpus across tracked DVA documentation, skills, scripts and pinned canonical consumer repositories; record repository IDs, revisions, scanned paths, literal matches, unresolved dynamic calls, and scanner limitations | verify: human — missing canonical repositories, unpinned revisions, or unexplained dynamic invocations stop a rename decision
- [ ] Compare `ktl` canonical, `kubectl` canonical with compatibility, and no-change options for discoverability, typing cost, script compatibility, interaction collisions, completion, and support burden | verify: human — all three options and rejected reasons must be recorded
- [ ] If names coexist, freeze which name is canonical, whether the other is a hidden or visible compatibility route, how both names remain reserved, and parity across root flags, entry selection, passthrough argv, help, manifest, completion, debug output, exit status, signals, and process replacement | verify: human — no unspecified alias behavior may reach implementation
- [ ] Preserve the current collision matrix unless a separate approved contract changes it: config load warning, `config validate` error, bare-name built-in precedence, exact interaction reachability through `dva run <name>`, and reserved-prefix namespace rejection must be explicit for every coexisting name | verify: human — fail closed must not be interpreted as removing the explicit `run` escape route
- [ ] Decide whether manifest represents one canonical command with compatibility routes or coequal routes, including schema versioning and legacy-field meaning; if current schema cannot express the decision, require the bounded child produced from TASK-254 before implementation | verify: human — TASK-256 must not invent route-identity fields ad hoc
- [ ] Freeze deprecation warning channel, minimum compatibility releases, removal evidence gate, rollback route, and documentation migration; absence of sufficient evidence selects the current `ktl` route | verify: human — deprecation and removal must be separate decisions
- [ ] Obtain independent compatibility review, append an approved `## Decision Record` to this card, and change `decision-status` from `pending` to `decided` before TASK-256 begins | verify: `make doc-check`

## Non-goals

- No route registration or reserved-name change.
- No kubectl runner behavior change.
- No compatibility removal in the same release that introduces a new canonical name.
