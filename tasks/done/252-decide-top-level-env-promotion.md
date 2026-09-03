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
depends-on: [TASK-246, TASK-248]
---

# Task 252: decide top-level env promotion

## Summary

First decide whether top-level promotion has enough product value to justify building the TASK-251 evidence gate.
If it does, resume this same card with pinned evidence for the final route decision; promotion is never automatic.

## Decision required

The permanent surface can be selected without first building a repository scanner. Promotion review requires
TASK-251 because passing its migration gate only makes reservation review possible; it does not make promotion mandatory.

## Recommended direction

`config env`를 영구 canonical surface로 유지하는 것을 권장한다. Top-level `env`는 typing을 조금 줄이지만
새 reserved name, interaction migration, hook/collision 설명과 rollback 비용을 만든다. Secret bridge는
자주 입력하는 일상 명령이 아니므로 이 비용을 상쇄할 제품 가치가 아직 없다.

실제 사용 빈도·discoverability 가치가 gate와 migration 비용을 정당화하지 못하면 TASK-251을 N/A로
종료한다. 조사를 계속하기로 선택한 경우 이 카드는 `pending`으로 남기고 TASK-251 뒤 재개한다.
TASK-251이 완전한 green evidence를 만들더라도 그것은 promotion 가능성만 증명한다. 실제 사용자
discoverability 측정과 routing candidate 검증이 별도 이익을 증명하지 못하면 현재 group 안에서 끝낸다.

## Completion Criteria

- [ ] First compare permanent `config env` with the measured product value and cost of promotion investigation; either select the permanent surface and disposition TASK-251 as N/A, or record why evidence collection is justified while keeping this decision pending | verify: human — the interim or final choice, evidence, rejected alternative, and TASK-251 disposition must be recorded
- [ ] If investigation continues, re-run TASK-251's virtual-reservation gate with the base DVA, scanner, and external repository revisions frozen for the final decision; stale, missing, ambiguous, unresolved, or non-zero evidence stops promotion and selects permanent `config env` | verify: human — the reviewed manifest/report location, byte digest, retention boundary, virtual reserved set, and all pinned revisions must be recorded
- [ ] Compare permanent `config env` against top-level reservation for discoverability, script compatibility, interaction conflicts, hook behavior, security, and ownership | verify: human — both options must remain viable until evidence is evaluated
- [ ] If promotion is selected, freeze canonical route, `config env` compatibility behavior, deprecation warning/removal policy, `dva run env` escape path, and at least one release of rollback support | verify: human — no unspecified alias or deprecation semantics may reach implementation
- [ ] If promotion is selected, require the new implementation child to build the actual reservation candidate and rerun the pinned corpus before integration; TASK-251 virtual evidence is eligibility evidence and must not be reported as release acceptance | verify: human — the child card must carry exact candidate commit/binary digest and same-revision corpus gates
- [ ] Obtain independent security and compatibility review; unresolved external SSOT or dynamic-call findings require choosing permanent `config env` | verify: human — review findings and disposition must be recorded
- [ ] Record one final decision with rejected alternative, rollback, release boundary, and migration cost before any reservation change is integrated; if promotion is selected, create a bounded implementation/release card and add it to PLAN-002 before closing this card | verify: `make doc-check`

## Fail-closed default

If every criterion is not satisfied, close the migration with `config env` as the supported final
surface. Do not keep moving a promised N+1 date forward.
