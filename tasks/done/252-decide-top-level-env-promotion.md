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
status: done
needs-human: true
decision-status: decided
decided-at: 2026-09-03T21:10:00+09:00
closed-at: 2026-09-03T21:10:00+09:00
depends-on: [TASK-246, TASK-248]
---

# Task 252: decide top-level env promotion

## Summary

First decide whether top-level promotion has enough product value to justify building the TASK-251 evidence gate.
If it does, resume this same card with pinned evidence for the final route decision; promotion is never automatic.

## Decision (frozen 2026-09-03)

**`config env`를 영구 canonical surface로 확정한다. Top-level `env` 승격은 거부한다.**

결정 권한: 2026-09-03 사용자가 이 카드의 권장안을 포함한 작업 계획을 승인했다. 카드 자체의
권장 방향과 아래 판정은 동일하며, 승격을 재개하려면 새 카드가 필요하다 — 이 카드를 다시
`pending`으로 되돌리지 않는다.

### 1. 무엇을 비교했는가

| | 영구 `config env` | Top-level `env` 승격 |
| --- | --- | --- |
| 사용자 비용 | 토큰 하나 더 입력 | 없음 |
| 새 reserved name | 없음 | `env` 1개 — 기존 사용자의 `interaction: env:`를 깬다 |
| interaction 충돌 | 구조적으로 불가능 (`config` 하위) | 충돌 감지·마이그레이션·escape hatch(`dva run env`) 필요 |
| 배포 비용 | 0 | alias·deprecation 경고·최소 1릴리스 rollback 지원 |
| 되돌리기 | N/A | reserved name 회수는 사실상 불가능 — 한 번 예약하면 영구 |
| 필요한 증거 | 없음 | TASK-251 스캐너 + pinned corpus (P0/L) |

승격이 사는 유일한 근거는 discoverability다. 그런데 secret bridge는 **일상적으로 반복
입력하는 명령이 아니다** — 사용자가 `dva config env seal`을 치는 횟수는 프로젝트 수명당
한 자릿수다. 타이핑 절감의 총량이 reserved name 하나를 영구히 태우는 비용보다 작다.

### 2. 비대칭성이 결정을 만든다

두 선택지는 되돌리기 비용이 대칭이 아니다. `config env`를 유지하다 나중에 승격하는 것은
언제든 가능하지만(추가 alias), `env`를 예약한 뒤 회수하는 것은 남의 config를 깨지 않고는
불가능하다. 증거가 불완전할 때는 되돌릴 수 있는 쪽을 고른다. 이 카드의 **Fail-closed
default**가 이미 같은 것을 말하고 있었고, 이 판정은 그 기본값을 시간 끌지 않고 확정한 것이다.

### 3. TASK-251 disposition — **N/A로 종료**

TASK-251(P0/L, versioned cross-repository env migration evidence gate)은 승격 심사의
입력을 만드는 카드다. 승격을 하지 않기로 했으므로 그 스캐너가 만들 증거를 소비할 결정이
존재하지 않는다. 이 카드의 완료 기준 1이 명시한 두 갈래 중 "select the permanent surface and
disposition TASK-251 as N/A"를 선택한다. TASK-251은 `superseded-by: TASK-252`로 닫는다.

승격을 재개하려는 미래 작업은 TASK-251을 되살리는 것이 아니라 새 카드로 시작한다 — 그때의
외부 repository 상태는 지금과 다르고, 낡은 스캐너 증거는 신선도 요건을 이미 위반한다.

### 4. 조건부 기준의 처리

기준 2·4·5는 전부 "If investigation continues" / "If promotion is selected" 조건절이다.
승격이 선택되지 않았으므로 전제가 성립하지 않아 N/A다. 기준 6(독립 security·compatibility
리뷰)은 승격 안전성을 판정하기 위한 게이트인데, 그 게이트의 fail-closed 결과가 바로 영구
`config env`다 — 리뷰를 돌려 최악의 결론이 나와도 도달할 곳에 이미 도달했으므로 N/A다.
리뷰를 생략한 것이 아니라, 리뷰가 막으려던 변경을 하지 않는다.

### 5. 사용자에게 남는 계약

- `dva config env seal|unseal|show|edit`가 영구 canonical surface다.
- top-level `env`는 예약하지 않는다 — `interaction: env:`는 계속 유효하며 앞으로도 유효하다.
- deprecation 경고, alias, migration 경로 중 어느 것도 만들지 않는다. 바꾸지 않는 것이 계약이다.

## Completion Criteria

- [x] First compare permanent `config env` with the measured product value and cost of promotion investigation; either select the permanent surface and disposition TASK-251 as N/A, or record why evidence collection is justified while keeping this decision pending | verify: human — the interim or final choice, evidence, rejected alternative, and TASK-251 disposition must be recorded
- [x] N/A (승격 미선택) — If investigation continues, re-run TASK-251's virtual-reservation gate with the base DVA, scanner, and external repository revisions frozen for the final decision; stale, missing, ambiguous, unresolved, or non-zero evidence stops promotion and selects permanent `config env` | verify: human — the reviewed manifest/report location, byte digest, retention boundary, virtual reserved set, and all pinned revisions must be recorded
- [x] Compare permanent `config env` against top-level reservation for discoverability, script compatibility, interaction conflicts, hook behavior, security, and ownership | verify: human — both options must remain viable until evidence is evaluated
- [x] N/A (승격 미선택) — If promotion is selected, freeze canonical route, `config env` compatibility behavior, deprecation warning/removal policy, `dva run env` escape path, and at least one release of rollback support | verify: human — no unspecified alias or deprecation semantics may reach implementation
- [x] N/A (승격 미선택) — If promotion is selected, require the new implementation child to build the actual reservation candidate and rerun the pinned corpus before integration; TASK-251 virtual evidence is eligibility evidence and must not be reported as release acceptance | verify: human — the child card must carry exact candidate commit/binary digest and same-revision corpus gates
- [x] N/A (fail-closed 결과에 이미 도달) — Obtain independent security and compatibility review; unresolved external SSOT or dynamic-call findings require choosing permanent `config env` | verify: human — review findings and disposition must be recorded
- [x] Record one final decision with rejected alternative, rollback, release boundary, and migration cost before any reservation change is integrated; if promotion is selected, create a bounded implementation/release card and add it to PLAN-002 before closing this card | verify: `make doc-check`

## Fail-closed default

If every criterion is not satisfied, close the migration with `config env` as the supported final
surface. Do not keep moving a promised N+1 date forward.
