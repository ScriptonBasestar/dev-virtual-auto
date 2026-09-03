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
decision-status: decided
decided-at: 2026-09-04T09:10:00+09:00
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
Aggregation은 child stack을 parent에 flatten하거나 child `env_file`을 root에 merge하는 기능이 아니다.
각 imported canonical/alias plan은 TASK-262가 확정한 owning child effective config를 계속 사용한다.

Destructive flag는 root에서 명시한 scope 안에서만 child로 전달하고 지원 여부가 다르면 전체 실행 전에
거부한다. Partial failure는 resolved plan과 completed/failed/rolled-back state를 machine-readable하게 남긴다.

## Completion Criteria

- [ ] Compare no composition, declarative plan include, and explicit root aggregation; record the selected model and why the rejected models fail product, operability, or compatibility constraints | verify: human — convenience alone is insufficient to add a second orchestration layer
- [ ] Apply TASK-263's frozen address and exposure contract, then freeze root/child identity, cycle detection, duplicate inclusion, default selection, environment/site/vars merge, entry overrides, `depends_on`, `order`, and resolved-plan immutability; reject child-stack flattening, child-`env_file` merging into root, and owner loss through aliases | verify: human — every ambiguity must have a fail-closed rule and an accepted/rejected YAML fixture
- [ ] Freeze execution waves, working directories, every lifecycle verb, per-project scope and propagation or rejection of `--no-wait`, `--var`, tag selectors, `--force`, `--volumes`, and `--purge`, readiness, LIFO rollback, cancellation, retry and idempotence | verify: human — destructive flags require explicit scope and confirmation behavior; no child may receive an unsupported flag silently
- [ ] Freeze partial failure, rollback failure with original-error preservation, partial-state reporting, recovery and retry, aggregate status/logs/build behavior, text/JSON output, diagnostics, and exit codes | verify: human — success and failure fixtures must cover at least two projects, a dependency cycle, a failed rollback, and a resumable partial state
- [ ] Define compatibility and migration for existing local plans and imported item names, plus rollback after a failed rollout; do not silently reinterpret an existing valid configuration | verify: human — before/after configuration and invocation examples must be recorded
- [ ] Obtain independent architecture and operability review, append an approved `## Decision Record` to this card, and change `decision-status` from `pending` to `decided`; if composition is selected, create a separate implementation plan with bounded schema, resolver, runtime, CLI, migration, and fixture cards | verify: `make doc-check`

## Non-goals

- No schema, resolver, orchestrator, CLI, or migration implementation in this card.
- No automatic reachability without an approved identity contract.
- No vNext vocabulary decision beyond terminology needed to make this contract unambiguous.

## Decision Record (2026-09-04)

**모델을 확정한다: 단방향 root-aggregation.** Root plan이 명시적으로 exposed된 child plan만
aggregate하고, child의 recursive include는 금지한다. Root가 전체 DAG와 rollback 순서를 한 번
해석하고, aggregation은 child stack을 parent로 flatten하거나 child `env_file`을 root에 merge하는
기능이 아니다. 각 imported canonical/alias plan은 TASK-262가 확정한 owning child effective
config를 그대로 사용한다. Destructive flag는 root가 명시한 scope 안에서만 child로 전달하고,
child 쪽 미지원 시 전체 실행 전에 거부한다. `## Recommended direction`에 기술된 방향을 그대로
채택한다.

### 1. 판단 권한

2026-09-04 사용자가 이 카드를 독립된 질문으로 직접 제시받고 권장안(단방향 root-aggregation)을
선택했다. 제시 시점에 대안 세 가지(합성 미도입, declarative 양방향 include, 증거 부족으로 보류)가
비용·근거와 함께 나란히 제시됐고, 그중 권장안을 채택했다 — 권고를 뒤집은 판단이 아니라 권고를
확정한 판단이다. 같은 자리에서 이번 라운드의 엔지니어링 범위도 "결정 기록만"으로 확정됐다 —
아래 §5에서 그 경계를 명시한다.

### 2. 완료기준 1 — 세 모델 비교 (이 절로 충족)

| | 합성 미도입 (기각) | **단방향 root-aggregation (채택)** | declarative 양방향 include (기각) |
| --- | --- | --- | --- |
| Subproject 소유권 | 변경 없음 — 각자 별도 dva.yml 실행 | root가 명시적으로 노출된 child만 소유 인식 | child가 parent/sibling을 스스로 include — 소유 경계 흐려짐 |
| Cycle 위험 | 없음 (합성 자체가 없음) | 낮음 — root 한 곳만 DAG를 해석 | 높음 — 양방향 include는 cycle 탐지를 모든 참여자에 분산시킴 |
| Teardown 순서 | 각자 독립 — 모호성 없음 | root가 LIFO rollback을 한 번 해석 | 여러 root 후보가 동시에 rollback을 주장할 수 있어 모호 |
| Duplicate ownership | 발생 불가 | root 하나가 aggregate하므로 발생 불가 | child가 여러 parent에 동시 include되면 소유자가 둘이 됨 |
| TASK-261(vocabulary) 의존성 | 즉시 해소되지만 subproject 조합 유즈케이스 자체가 사라짐 | 유지 — 별도 orchestration layer로 존재 | 유지 |
| 도입 비용 | 0 | 중간 — root만 새 계약 필요 | 높음 — 모든 참여자가 새 계약 필요 |

기각 사유. **합성 미도입**은 비용이 0이라는 점에서 가장 안전하지만, PLAN-003이 이미 전제한
subproject 조합 유즈케이스(여러 project를 하나의 plan으로 순서대로 기동·철거) 자체를 포기하는
것이므로 "편의성 부족"이 아니라 "요구된 제품 시나리오 미해결"로 기각한다. **Declarative 양방향
include**는 유연하지만 카드의 `## Recommended direction`이 이미 지목한 대로 cycle·중복
ownership·teardown 모호성을 구조적으로 늘린다 — 참여자 수만큼 해석 지점이 늘어나기 때문이다.

### 3. 완료기준 2~4 — 여전히 열려 있다 (이 결정으로 닫히지 않음)

완료기준 2(root/child identity·cycle 탐지·merge 규칙·fixture), 3(execution wave·lifecycle
verb별 destructive flag 전파·rollback·재시도), 4(partial failure·rollback failure·machine-readable
상태·exit code)는 **모델 선택과 별개의 상세 계약 작업**이며, 이번 결정으로 자동으로 채워지지
않는다. 이 세 항목은 사람이 "어떤 모델이냐"가 아니라 "그 모델의 각 분기에서 무엇이 fail-closed
규칙이냐"를 하나씩 확정해야 하는 작업으로, 카드 자신의 `verify: human` 조항이 요구하는 대로
accepted/rejected YAML fixture와 exit code 표가 필요하다. TASK-263의 결정 기록(§5)이 보여준
패턴과 동일하게, 이런 상세 계약은 이 카드의 방향 결정과 분리해 별도 라운드에서 다룬다.

TASK-263이 이미 동결한 조각은 그대로 재사용한다 — canonical route는 `--project` 명시·`:`
축약·`/` 명시적 import 세 형식 공존, automatic registration과 flattening 거부, explicit import
(자동 노출 없음). 완료기준 2가 "Apply TASK-263's frozen address and exposure contract"라고
요구하는 것은 이 조각을 상세 계약의 출발점으로 삼으라는 뜻이지, 이 결정 기록이 그 상세 계약
자체를 대신 작성하라는 뜻이 아니다.

### 4. 완료기준 5 — 호환성/마이그레이션도 열려 있다

기존 local plan과 imported item 이름의 호환성, 실패한 rollout 이후의 rollback 정의는 완료기준
2~4의 상세 계약이 먼저 확정돼야 의미 있게 작성할 수 있다. 이 결정 기록은 "기존 유효 설정을
조용히 재해석하지 않는다"는 원칙만 재확인하고, 구체적인 before/after 예시는 상세 계약 작업으로
넘긴다.

### 5. 완료기준 6 — 이 라운드에서 하지 않는 것

완료기준 6은 "합성이 선택되면 별도의 bounded 구현 계획(schema·resolver·runtime·CLI·migration·
fixture 카드)을 만들라"고 요구한다. 이 하위 조항은 **이번 결정 기록에서 실행하지 않는다** — 그
계획의 "bounded" 경계는 완료기준 2~4의 상세 계약이 정한 fail-closed 규칙에서 나오는데, 그
계약이 아직 없으므로 지금 계획 카드를 만들면 경계 없는 카드가 된다. 2026-09-04 사용자가 이번
라운드의 엔지니어링 실행 범위를 "TASK-260 결정 기록만"으로 명시적으로 한정했으므로(§1),
완료기준 2~5의 상세 계약과 완료기준 6의 구현 계획 카드 생성은 다음 라운드로 남긴다.

`decision-status`는 `decided`로 바꾸되 카드는 `todo/`에 남는다. 사람이 답할 것은 모델 선택
하나였고 그것은 끝났다 — 남은 것은 그 모델을 상세 계약으로 확장하는, 목표가 고정된 엔지니어링
판단 작업이다. **TASK-261은 이 카드의 완료(모든 완료기준 체크, 특히 6의 구현 계획 카드 생성)
전이 아니라 이 카드의 `decision-status: decided`만으로는 열리지 않는다** — TASK-261의
`depends-on`은 TASK-260을 카드 단위로 참조하므로, 실무적으로는 TASK-256·TASK-258 구현이 먼저
끝나는 편이 이 카드의 나머지 완료기준보다 빠를 가능성이 높다.
