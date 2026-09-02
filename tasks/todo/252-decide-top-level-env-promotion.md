---
id: TASK-252
title: "Decide whether top-level env promotion is safer than keeping config env"
type: chore
priority: P0
effort: M
exec-tier: strong
created-at: 2026-09-01T19:28:00+09:00
source: "PLAN-002 optional promotion decision gate"
scope: "security and compatibility review, candidate corpus evidence, route/alias/deprecation contract, release decision"
status: todo
needs-human: true
decision-status: pending
depends-on: [TASK-251]
---

# Task 252: decide top-level env promotion

## Summary

Use same-revision virtual-reservation evidence and independent review to choose between permanent `config env` and a new
top-level reservation; promotion is optional, never automatic.

## Decision required

Passing a migration gate makes reservation review possible; it does not make promotion mandatory.
This card compares a permanent `config env` surface with a new top-level `env` reservation using the
same-revision evidence produced by TASK-251.

## Recommended direction

`config env`를 영구 canonical surface로 유지하는 것을 권장한다. Top-level `env`는 typing을 조금 줄이지만
새 reserved name, interaction migration, hook/collision 설명과 rollback 비용을 만든다. Secret bridge는
자주 입력하는 일상 명령이 아니므로 이 비용을 상쇄할 제품 가치가 아직 없다.

TASK-251이 완전한 green evidence를 만들더라도 그것은 promotion 가능성만 증명한다. 실제 사용자
discoverability 측정과 routing candidate 검증이 별도 이익을 증명하지 못하면 현재 group 안에서 끝낸다.

## Completion Criteria

- [ ] Re-run TASK-251's virtual-reservation gate with the base DVA, scanner, and external repository revisions frozen for this decision; stale, missing, ambiguous, unresolved, or non-zero evidence stops promotion and selects permanent `config env` | verify: human — the reviewed manifest/report location, byte digest, retention boundary, virtual reserved set, and all pinned revisions must be recorded
- [ ] Compare permanent `config env` against top-level reservation for discoverability, script compatibility, interaction conflicts, hook behavior, security, and ownership | verify: human — both options must remain viable until evidence is evaluated
- [ ] If promotion is selected, freeze canonical route, `config env` compatibility behavior, deprecation warning/removal policy, `dva run env` escape path, and at least one release of rollback support | verify: human — no unspecified alias or deprecation semantics may reach implementation
- [ ] If promotion is selected, require the new implementation child to build the actual reservation candidate and rerun the pinned corpus before integration; TASK-251 virtual evidence is eligibility evidence and must not be reported as release acceptance | verify: human — the child card must carry exact candidate commit/binary digest and same-revision corpus gates
- [ ] Obtain independent security and compatibility review; unresolved external SSOT or dynamic-call findings require choosing permanent `config env` | verify: human — review findings and disposition must be recorded
- [ ] Record one final decision with rejected alternative, rollback, release boundary, and migration cost before any reservation change is integrated; if promotion is selected, create a bounded implementation/release card and add it to PLAN-002 before closing this card | verify: `make doc-check`

## Fail-closed default

If every criterion is not satisfied, close the migration with `config env` as the supported final
surface. Do not keep moving a promised N+1 date forward.
