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
decision-status: decided
decided-at: 2026-09-03T21:10:00+09:00
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

## Decision Record (2026-09-03)

**`dva config validate`를 canonical documentation example로 채택한다. Top-level `dva validate`는
visible, behavior-identical shortcut으로 계속 지원한다. 두 route 모두 제거·deprecation 일정을
두지 않는다.**

### 판단 권한과 근거

2026-09-03 사용자는 이전 라운드 보고서에서 이 카드를 포함한 실행 범위를 `AskUserQuestion`으로
직접 제시받았다. 제시된 선택지 문구는 "TASK-257 ... 카드에 권장안을 Decision Record로 기록
(decision-status: decided)"였고 사용자는 이 문구가 포함된 선택지를 명시적으로 선택했다. 즉
사용자가 승인한 것은 이 카드의 기존 `## Recommended direction`을 그대로 결정으로 전환하라는
지시이며, 카드 본문의 비교 근거(완료기준 2)는 이 라운드에서 새로 사용자에게 제시되지 않았다.

TASK-255와의 차이를 분명히 남긴다. TASK-255는 사용자가 세 선택지와 비용표를 직접 보고 그
자리에서 권장안을 뒤집었다. 이 카드는 그런 실시간 검토가 없었다 — 사용자는 "카드가 이미 제안한
방향을 채택하라"는 실행 지시를 승인했을 뿐이다. 이 결정의 권한은 후자의 형태이며, 앞으로 이
결정을 재론할 때 "사용자가 비교표를 직접 검토하고 선택했다"고 서술해서는 안 된다.

### 이 판정이 하지 않는 것 — 완료기준 1은 면제되지 않는다

완료기준 1의 verify 조항은 "missing or stale evidence **stops route removal or hiding**"이다.
이번 결정은 두 route 모두 유지하고 제거·숨김을 하지 않으므로, 그 게이트가 막던 행동 자체를
아예 선택하지 않았다. 따라서 코퍼스가 없어도 이 방향(direction)은 확정할 수 있지만, 코퍼스를
면제하는 것은 아니다 — 완료기준 1은 여전히 미체크 상태로 남고, 장래에 어느 한 route를 제거하거나
숨기는 방향으로 재론하려면 그때 코퍼스가 필요하다.

### 완료기준 2 — 세 선택지 비교

| | `config validate` canonical + top-level 호환 (채택) | top-level canonical + `config validate` 호환 (기각) | 대등 route (기각) |
| --- | --- | --- | --- |
| 개념적 소속 | `config` 하위로 정확 — validate는 config 검증 | 부정확 — top-level이 config 개념을 가림 | 문서가 두 이름을 동등하게 가르쳐야 함 |
| 기존 skill/automation | 그대로 보존 (`config validate` 이미 주로 사용) | 재작성 필요 | 재작성 필요 없음, 그러나 중복 문서 |
| discoverability | top-level shortcut이 짧은 진입점 유지 | 동일 | 동일 |
| 구현 변경 | 없음 (이미 구현 공유) | 없음 | 없음 |
| 지원 부담 | 낮음 — 문서만 한 방향 고정 | 낮음 | 높음 — 두 문서 계열 유지 |

기각 사유: top-level canonical은 기존 skill/문서가 이미 `config validate`를 주 예시로 쓰고
있어 재작성 비용만 발생시키고 이득이 없다. 대등 route는 문서 이중화로 지원 부담만 늘리고
`config validate`가 이미 개념적으로 더 정확한 소속을 갖는다는 이점을 버린다.

### 완료기준 3·4·5 — 미결로 남는 이유

이번 결정은 문서상 canonical example 선택일 뿐, parity 동결(완료기준 3), manifest route-identity
표현(완료기준 4), deprecation/rollback 계약(완료기준 5)은 아직 다루지 않았다. 완료기준 4는
TASK-254에서 산출된 bounded child를 구현 전에 요구하는데, 그 child는 TASK-272다 — TASK-272는
이미 결정됐다(`44a8b78`). TASK-258을 시작하려면 완료기준 1·3·4·5가 먼저 닫혀야 한다.

### 이 판정이 만들어낸 후속 구속

두 route 모두 제거하지 않으므로 `docs/42-migration-and-compatibility.md` 등 기존 문서 내용은
무효화되지 않는다. TASK-258이 시작되기 전까지 코드·스키마 변경은 없다.
