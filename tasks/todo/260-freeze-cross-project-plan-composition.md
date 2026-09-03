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

- [x] Compare no composition, declarative plan include, and explicit root aggregation; record the selected model and why the rejected models fail product, operability, or compatibility constraints | verify: human — convenience alone is insufficient to add a second orchestration layer
- [x] Apply TASK-263's frozen address and exposure contract, then freeze root/child identity, cycle detection, duplicate inclusion, default selection, environment/site/vars merge, entry overrides, `depends_on`, `order`, and resolved-plan immutability; reject child-stack flattening, child-`env_file` merging into root, and owner loss through aliases | verify: human — every ambiguity must have a fail-closed rule and an accepted/rejected YAML fixture
- [x] Freeze execution waves, working directories, every lifecycle verb, per-project scope and propagation or rejection of `--no-wait`, `--var`, tag selectors, `--force`, `--volumes`, and `--purge`, readiness, LIFO rollback, cancellation, retry and idempotence | verify: human — destructive flags require explicit scope and confirmation behavior; no child may receive an unsupported flag silently
- [x] Freeze partial failure, rollback failure with original-error preservation, partial-state reporting, recovery and retry, aggregate status/logs/build behavior, text/JSON output, diagnostics, and exit codes | verify: human — success and failure fixtures must cover at least two projects, a dependency cycle, a failed rollback, and a resumable partial state
- [x] Define compatibility and migration for existing local plans and imported item names, plus rollback after a failed rollout; do not silently reinterpret an existing valid configuration | verify: human — before/after configuration and invocation examples must be recorded
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

### 3. 완료기준 2 — identity, cycle, duplicate, default, merge precedence, entry override, depends_on, order, immutability

이 절부터는 §1에서 확정된 모델(단방향 root-aggregation) 위에서 사람의 재확인 없이 진행 가능한
상세 계약 작업이다 — "어떤 모델이냐"가 아니라 "그 모델의 각 분기에서 무엇이 fail-closed 규칙이냐"를
TASK-262/263의 기존 결정과 이 저장소의 기존 코드/관례를 그대로 연장해 확정한다.

**3.1 표현 — composition plan은 별도 plan 종류다.** `PlanConfig`(`internal/config/config.go:67`)는
오늘 `entries: []PlanEntry`만 갖는 "leaf plan"이다. 이 판정은 상호 배타적인 신규 필드
`composes: []ComposeEntry`를 얼린다 — **같은 plan이 `entries`와 `composes`를 동시에 선언하면 config
검증이 즉시 거부**한다(fail-closed). 정확한 Go 필드명·YAML 태그는 구현 카드가 코드 컨벤션에 맞춰
정하되(TASK-272가 `canonical_name` 필드명을 같은 방식으로 구현 카드에 위임한 선례를 따름), 의미는
다음으로 고정한다:

```go
// 예시 형태 — 필드명은 구현 카드가 확정
type ComposeEntry struct {
    Plan      string            // 참조 대상: 로컬 leaf plan 이름, 또는 이미 import된 canonical/alias 이름(project/plan)
    Order     int
    DependsOn []string          // 다른 ComposeEntry.Plan 값을 가리킴(같은 composes: 목록 안에서만)
    Vars      map[string]string // 이 composed 호출에만 적용되는 override(기존 PlanEntry.Vars와 동일 메커니즘)
}
```

**3.2 Root/child identity.** TASK-263가 동결한 문법을 그대로 재사용하고 새 식별자 문법을 만들지
않는다: `ComposeEntry.Plan`이 가리키는 이름은 로컬 plan(단순 이름) 또는
`subprojects.<name>.import.plans`로 이미 import된 canonical(`project/plan`) 혹은 그 import가
선언한 alias여야 한다. Import되지 않은 child plan을 `project/plan` 형태로 직접 참조하는 것은
거부한다 — TASK-263 §4의 "automatic reachability 없음"을 composition이 우회하는 뒷문이 되지
않게 한다.

**3.3 Cycle 금지 — recursion 자체를 거부한다(별도 cycle-detection 알고리즘 불필요).**
`composes:`가 참조하는 대상 plan이 그 자신도 `composes:`를 가진 composition plan이면 **무조건
거부**한다 — 진짜 cycle을 이루는지 여부와 무관하다(A→B→A든, cycle이 아닌 A→B→C라도 B나 C가
composition이면 거부). 이는 두 가지를 동시에 보장한다: (a) 진짜 cycle은 구조적으로 불가능해진다
(그래프가 항상 2-level DAG), (b) "composition의 composition"이라는 재귀적 실행 모델 자체를 만들지
않는다(`## Recommended direction`의 "recursive include 금지"). 이 검사는 로컬 plan과 import된 plan
양쪽에 적용되며, import 시점(`resolveSubprojectImports`, `internal/config/subproject.go:88`)에서
"composition plan은 import 대상이 될 수 없다"는 규칙으로 한 번 더 강제한다 — child가 자신의
composition plan을 부모에게 alias로 노출하려 시도해도 import 단계에서 즉시 거부되어, 조상
project가 그 사실을 몰라도 안전하다.

**3.4 Duplicate inclusion.** 같은 `composes:` 목록 안에서 같은 `Plan` 값이 두 번 나오면 거부한다
(기존 `entries:` 목록의 stack entry 이름 중복 거부와 동일한 원칙 — 이 저장소는 이미 plan/alias/
provision import 각각에서 "collision → 즉시 에러"를 반복 적용한다, `subproject.go`의 `already
exists` 에러들 참고). 서로 다른 두 alias가 같은 canonical import를 가리키는 경우(예: `api/deploy`와
그 alias `api-deploy`를 같은 `composes:`에 둘 다 넣는 것)도 owner 기준으로 동일 대상이므로 거부한다.

**3.5 Default selection.** Composition plan은 `default_plan` 및 "선언된 plan이 정확히 하나면
자동 선택" 규칙에서 leaf plan과 **동일하게** 취급된다 — 특별 취급하지 않는다. `default_plan`이
composition plan을 가리키면 `dva up`(인자 없음)은 그 composition을 실행한다. Plan이 여럿이고
`default_plan`이 없으면(leaf/composition 혼재 포함) 기존 규칙대로 이름을 지정하라고 거부한다.

**3.6 Environment/site/vars merge precedence.** Composition plan 자체는 `environment:`/`site:`/
`vars:`(plan 최상위 필드)를 **선언할 수 없다** — 선언 시 config 검증에서 거부한다. Composition
plan은 자신이 직접 실행하는 stack entry가 없으므로 이 필드들이 적용될 대상이 없기 때문이다. 각
composed child는 TASK-262가 동결한 대로 **자신의 owning child effective config**(자신의
environment/site/env_file/vars)를 그대로 쓴다 — root의 environment/site가 child로 흘러들어가는
경로는 없다. Root가 개입할 수 있는 유일한 지점은 `ComposeEntry.Vars`이며, 이는 기존
`PlanEntry.Vars`와 동일하게 "문서화된 CLI `--var` 수준의 override"로 취급한다(merge가 아니라
override — TASK-262의 "documented parent CLI overrides" 원칙 재사용).

**3.7 Entry override.** `ComposeEntry.Vars`만 허용한다. Root가 child plan의 `Runner`, `Services`,
stack entry 선택 같은 leaf-plan 개념을 override하는 것은 거부한다 — 그런 override는 child의
`entries:` 소관이며 root가 손댈 계약이 없다(child-stack flattening 금지의 직접적 귀결).

**3.8 `depends_on`/`order`/wave.** 기존 `internal/lifecycle/resolver.go`의 `CalculateWaves`
(order + topological depends_on)를 **그대로 재사용**한다 — `ComposeEntry.Order`/`DependsOn`은
`PlanEntry.Order`/`DependsOn`과 동일한 알고리즘으로 wave를 계산하되, 대상이 stack entry가 아니라
composed child plan이라는 점만 다르다. `DependsOn`은 반드시 같은 `composes:` 목록 안의 다른
`Plan` 값만 가리킬 수 있다(cross-composition 참조 없음 — composition이 하나뿐이므로 자명하다).

**3.9 Resolved-plan immutability.** 기존 `ExecutionPlan`/`ResolvedEntry`의 불변성 원칙을 그대로
확장한다 — composition을 resolve한 결과(각 wave에 배치된 composed child 목록과 그 owner effective
config)는 실행 시작 후 재계산되지 않는다. 실행 도중 child의 `dva.yml`이 바뀌어도 이미 시작된
composition run은 resolve 시점 스냅샷을 계속 쓴다(단일 project의 오늘 동작과 동일한 정신).

**3.10 명시적 거부 목록(재확인).**
- **Child-stack flattening**: `composes:`는 child의 `stack:` 항목을 root 네임스페이스로 복사하지
  않는다 — root manifest/status에 child의 개별 stack entry 이름이 노출되지 않는다(§5의 aggregate
  status가 project 단위로만 보고한다).
- **Child `env_file`의 root merge**: 어떤 경로로도 child의 `env_file` 값이 root의 environment로
  합쳐지지 않는다 — TASK-262가 이미 이 경계를 그었고, composition은 그 경계를 재확인할 뿐 다시
  정의하지 않는다.
- **Alias를 통한 owner 상실**: canonical과 alias는 항상 같은 `*Config` owner를 공유한다
  (`subproject.go`의 `cloneImportedPlan`이 이미 이렇게 구현돼 있음 — alias는 새 owner를 만들지
  않고 같은 clone을 재사용한다). Composition이 alias를 참조해도 owner 판별은 canonical과 동일하다.

**Accepted fixture(root aggregation, 두 project, `depends_on` 사용):**

`api/dva.yml`(child):
```yaml
version: "1.5"
plans:
  deploy:
    entries:
      - name: api-server
```

`web/dva.yml`(child):
```yaml
version: "1.5"
plans:
  deploy:
    entries:
      - name: web-server
```

`root/dva.yml`:
```yaml
version: "1.5"
subprojects:
  api:
    path: ../api
    import:
      plans:
        - name: deploy
  web:
    path: ../web
    import:
      plans:
        - name: deploy
plans:
  release:
    composes:
      - plan: api/deploy
        order: 0
      - plan: web/deploy
        order: 1
        depends_on: ["api/deploy"]
```

`dva up release`는 wave 0에서 `api/deploy`, wave 1에서 `web/deploy`를 순서대로 실행한다.
`dva status release`는 project 단위(`api`, `web`)로 집계된 상태를 보고한다.

**Rejected fixture(composition을 다시 compose — 거부):**

`root/dva.yml`에 추가로:
```yaml
plans:
  release:
    composes:
      - plan: api/deploy
      - plan: web/deploy
  release-all:
    composes:
      - plan: release       # 거부: release는 leaf plan이 아니라 composition plan
```
`config validate`는 `release-all.composes[0]`가 composition plan을 참조했다는 이유로 거부한다
("composition plans cannot compose another composition plan" 형태의 에러) — §3.3의 recursion
금지 규칙이 정확히 이 fixture를 막기 위한 것이다.

### 4. 완료기준 3 — execution waves, working directory, verb별 동작, flag scope

**4.1 실행 모델 — sequential, wave는 순서 개념이지 동시성 개념이 아니다.** 오늘 단일 project의
`Orchestrator.Up`(`internal/lifecycle/orchestrator.go:75`)은 `CalculateWaves`로 wave를 계산하고도
`for _, entry := range filtered`로 **순차 실행**한다 — wave는 topological 순서를 강제할 뿐 병렬
실행을 만들지 않는다. Composition도 동일한 정신을 따른다: root는 wave 순서대로, 같은 wave 안에서는
선언 순서대로 **순차적으로** 각 composed child plan에 대해 해당 verb를 호출한다. 이 판정은
composition에 새로운 동시성 모델을 도입하지 않는다 — 오케스트레이터 구현은 비목표이지만, "병렬
실행을 약속하지 않는다"는 계약은 지금 얼린다. 그 결과 같은 owner project의 두 plan이 우연히 같은
wave에 배치돼도 안전 문제가 없다(둘은 그냥 선언 순서대로 하나씩 실행된다).

**4.2 Working directory.** 각 composed child는 TASK-262가 동결한 대로 **자신의 project root**를
working directory로 쓴다(root의 CWD가 흘러들어가지 않는다) — composition은 이 경계를 바꾸지
않는다. Root 자신이 소유한 hook 등은 root의 CWD를 쓴다.

**4.3 Verb별 cross-project 동작.**

| Verb | Composition 동작 |
|---|---|
| `up` | Wave 순서대로 각 composed child에 `up` 호출. 실패 시 §5의 rollback이 발동한다 |
| `down` | Wave의 **역순(LIFO)** 으로 각 composed child에 `down` 호출 — 마지막에 올라간 wave부터 내려간다 |
| `stop` | `down`과 동일하게 역순이되 리소스는 유지(기존 `Orchestrator.Stop`이 이미 이 의미) |
| `restart` | Wave 순서대로 각 child에 `restart` — child 내부는 child 자신의 restart 의미(개별 entry 재시작)를 그대로 쓰고, root는 child 단위로만 순서를 준다 |
| `status` | 모든 composed child를 병렬로 조회할 수 있다(읽기 전용이라 4.1의 순차 제약이 적용되지 않는다 — 부작용 없는 조회는 동시성 제약 대상이 아니다), project 단위로 집계해 보고(§5) |
| `logs` | Child별로 구획된 로그 스트림 — root가 child 로그를 합치거나 재해석하지 않는다. `dva logs release`는 각 child의 `dva logs <plan>`을 project 라벨과 함께 출력한다. 정확한 인터리브 방식은 구현 세부이며, 계약은 "project 라벨 필수"만 동결한다 |
| `build` | Wave 순서대로 각 child에 `build` — `up`과 동일한 순서 계약이되, rollback 대상이 아니다(build는 파괴적이지 않고 실행 상태를 만들지 않는다) |

**4.4 Flag별 scope — propagate-to-all / require-explicit-scope / reject 중 하나로 고정.**

| Flag | 판정 | 근거 |
|---|---|---|
| `--no-wait` | **전체 전파(propagate-to-all)** | 파괴적이지 않은 타이밍 지시일 뿐 — 모든 child의 readiness 대기를 동일하게 생략해도 안전하다. Child별로 다르게 줄 이유가 없고, 다르게 주려면 새 문법이 필요해 범위가 커진다 |
| `--no-rollback` | **전체 전파(propagate-to-all), opt-out 플래그** | §5.1의 자동 LIFO rollback 기본값을 끄는 명시적 opt-out. 2026-09-04 사용자가 "Open questions for review" 항목 1에 대해 사전 승인했다(끄지 않으면 오늘처럼 기본 rollback이 발동) — 디버깅 목적(실패한 child의 컨테이너 로그 등 사후 조사 증거를 rollback이 지워버리지 않도록 보존)으로만 쓴다. 이 카드는 플래그 이름과 의미만 얼리고 구현은 비목표로 남긴다 |
| `--var KEY=VAL` | **거부(reject)** | Bare `--var`는 "어느 child에 적용?"이 근본적으로 모호하다(root는 vars 없는 composition plan이므로, §3.6). CLI에서 override하려면 dva.yml의 `ComposeEntry.Vars`를 쓰라고 에러 메시지로 안내한다 — 새 qualified CLI 문법(`--var api/deploy:KEY=VAL` 같은)을 발명하지 않는다(비목표: CLI 구현 없음) |
| Tag selector(`--tag`/`--exclude-tag`) | **거부(reject)**, 기존과 동일 | 오늘 이미 "plan이 지정되면 whole-stack-path 플래그는 거부"된다(`compose.go`의 기존 문서 참고) — composition도 plan 경로이므로 동일 규칙이 그대로 적용된다. 새 규칙이 아니라 기존 규칙의 자연스러운 연장이다 |
| `--force` | **명시적 per-child scope 요구(require-explicit-scope)** | `--purge` 확인을 건너뛰는 용도이므로 `--purge`/`--volumes`와 같은 scope 규칙을 따른다(아래) |
| `--volumes`/`-v` | **명시적 per-child scope 요구** — `--project <child>`와 함께가 아니면 전체 실행 전에 거부 | 파괴적 플래그이므로 `## Recommended direction`의 "root에서 명시한 scope 안에서만 child로 전달"을 그대로 적용한다. TASK-263가 이미 동결한 `--project` 플래그(collision-safe explicit route)를 재사용한다 — composition에서는 "이 파괴적 플래그를 어느 child에만 적용할지"를 고르는 데 그대로 쓴다. Scope 없이 `dva down release --volumes`를 실행하면 **어떤 child도 건드리기 전에** 에러로 거부한다(부분 실행 후 발견이 아니다) |
| `--purge` | **명시적 per-child scope 요구**, `--volumes`와 동일 규칙 | 위와 동일 근거. `--purge`가 지원되지 않는 child(예: 파괴적 purge 대상이 없는 러너 기반 plan)에 scope가 걸리면 그 자체로 거부한다 — "지원 여부가 다르면 전체 실행 전에 거부"(`## Recommended direction` 원문) |
| `--mode`, `--env` | **거부**, 기존과 동일 | Whole-stack-path 플래그이며 plan 경로에서는 이미 거부된다(단일 project와 동일 규칙, 변경 없음) |

`--project <child>` scope 문법 예시: `dva down release --project api --volumes` → composition
`release`의 `api/deploy` 부분만 volume까지 제거하고 `web/deploy`는 건드리지 않는다(단, `release`가
`api/deploy`를 실제로 참조하지 않으면 "unknown scope" 에러). Scope 없는 `dva down release
--volumes`는 §5의 "전체 실행 전 거부" 진단을 낸다.

**4.5 Readiness, LIFO rollback, cancellation, retry, idempotence.**
- **Readiness**: 각 wave가 끝나면(그 wave의 모든 child의 `up`이 반환하면) 다음 wave로 넘어가기 전
  그 wave의 모든 child에 대해 owning child 자신의 readiness(health check)가 통과하기를 기다린다
  (`--no-wait`가 없는 한) — 4.1의 순차 실행 안에서 "wave 경계에서 한 번" 발생하는 게이트다.
- **LIFO rollback**: §5에서 상세.
- **Cancellation**: 진행 중 취소(SIGINT 등) 시 현재 실행 중인 child의 취소를 그 child의 기존
  동작에 위임하고, 이미 성공한 child들에 대해 §5의 rollback을 시도한다 — "취소도 실패의 한 형태"로
  취급하고, 별도 취소 전용 경로를 새로 만들지 않는다.
- **Retry**: §5에서 상세(신규 CLI 플래그나 persisted state 파일을 발명하지 않는다).
- **Idempotence**: Composition은 각 child의 기존 idempotent `up` 동작에 의존한다 — 이미 up
  상태인 child를 다시 composition에 포함해 재실행해도 child 자신이 idempotent하게 처리한다(오늘
  동작 그대로, composition이 새 idempotence 규칙을 추가하지 않는다).

### 5. 완료기준 4 — partial failure, rollback failure, partial-state, recovery, aggregate status/logs/build, output, exit code

**5.1 오늘의 기준선(중요한 grounding 사실).** `internal/lifecycle/orchestrator.go`의 `Up`은 **오늘도
단일 project 안에서 rollback을 하지 않는다** — 중간 entry가 실패하면 이미 시작된 이전 entry는
그대로 남고 에러만 반환한다. 즉 "실패 시 자동 rollback"은 composition이 **새로 도입하는** 계약이며
단일 project 동작을 복제한 것이 아니다. 이 판정은 그 도입을 명시적으로 승인한다 — 이유: 여러
project에 걸친 partial 상태는 사람이 수동으로 추적·정리하기 훨씬 어렵고, 카드의 `## Recommended
direction`이 이미 "completed/failed/rolled-back state를 machine-readable하게" 요구했다. 자동 rollback
기본값에는 명시적 opt-out(`--no-rollback`, 위 4.4 표 참고)이 있다 — 2026-09-04 사용자가 사전
승인했다(아래 "Open question — resolved" 참고).

**5.2 Partial failure 처리.** Wave N의 child가 실패하면:
1. Wave N 안에서 그 실패한 child 이후의(아직 시작 안 한) child는 시작하지 않는다.
2. Wave 0..N에서 이미 성공적으로 `up`된 모든 child를 **LIFO 순서**(가장 나중에 성공한 것부터)로
   `down`한다 — 이것이 rollback이다. Rollback은 항상 plain teardown(volume/purge 없음)이며,
   `up`에 어떤 플래그가 있었든 rollback 자체는 파괴적 옵션을 쓰지 않는다(4.4의 `--volumes`/
   `--purge`는 rollback에는 적용되지 않는다 — 새 확인 프롬프트를 rollback 경로에 추가하지 않기
   위함이다).
3. Rollback이 전부 성공하면: 원래 실패의 에러가 primary error로 보고되고, 어떤 child가 되돌려졌는지
   기록된다.
4. Rollback 중 일부 child의 `down`이 또 실패하면(**rollback failure**): 원래 실패(primary)의 에러
   메시지는 **변경되지 않고 그대로 유지**된다("original-error preservation") — rollback 실패는
   secondary diagnostics 목록에 별도로 추가되고, 그 child는 "여전히 up 상태로 남아있을 수 있음,
   수동 확인 필요"로 명시적으로 표시된다.

**5.3 Partial-state reporting 형식(JSON, 기존 `status.go`의 "map[string]any + PrintJSON" 관례를
따름).**

```json
{
  "dva_version": "...",
  "plan": "release",
  "kind": "composition",
  "outcome": "failed",
  "children": [
    { "project": "api", "plan": "deploy", "wave": 0, "state": "up" },
    { "project": "web", "plan": "deploy", "wave": 1, "state": "failed", "error": "entry \"web-server\" up failed: ..." }
  ],
  "rollback": {
    "attempted": ["api/deploy"],
    "succeeded": ["api/deploy"],
    "failed": []
  },
  "error": "entry \"web-server\" up failed: ..."
}
```
`state`는 `up`/`failed`/`rolled_back`/`rollback_failed`/`not_started` 중 하나로 고정한다. Text
출력은 이 구조를 사람이 읽기 쉬운 순서(wave 순, project 라벨 포함)로 그대로 표현하되 필드를 새로
발명하지 않는다.

**5.4 Recovery/retry.** 신규 CLI 플래그(`--retry` 등)나 persisted state 파일을 이 판정에서
발명하지 않는다(비목표: 스키마/오케스트레이터 구현 없음, 그리고 persisted state 파일 자체가 하나의
새 foundational schema이므로 이 카드 범위를 넘는다). 대신: 5.3의 리포트는 실패 직후 **그 실행
동안만** 화면에 출력되는 진단이고, 복구는 "같은 `dva up release`를 다시 실행"으로 이뤄진다 — 이미
rollback된(또는 애초에 실패해서 안 올라간) child는 처음부터 다시 시도되고, §4.5의 idempotence에
의해 이미 정상 상태인 child는 child 자신이 idempotent하게 처리한다. **Resumable partial state**는
디스크에 저장된 상태가 아니라 "child들의 실제 실행 상태 자체가 진실"이라는 원칙으로 얻는다 — root는
매 실행마다 새로 resolve하고 각 child에 다시 물어본다.

**5.5 Aggregate status/logs/build.** §4.3에서 이미 동결했다. `status`의 JSON은 5.3과 같은 shape를
성공 케이스에도 재사용한다(`outcome: "up"`, 모든 `state: "up"`).

**5.6 Exit code.** 신규 exit code 체계를 만들지 않는다 — 이 저장소는 오늘 `os.Exit(1)`만 쓰는 flat
관례를 갖는다(`internal/cli/root.go`). Composition도 동일: 전체 성공은 0, 그 외(부분 실패, rollback
실패 포함, cycle 거부로 인한 실행 전 중단 포함)는 전부 1.

**Fixture 시나리오:**

1. **두 project 성공** — §3의 accepted fixture. `dva up release` → exit 0, JSON
   `outcome: "up"`, 두 child 모두 `state: "up"`.
2. **Dependency cycle(실행 전 거부)** — composition이 다른 composition을 compose하도록 잘못
   선언하면(§3의 rejected fixture) §3.3 규칙에 의해 **어떤 child도 시작되기 전에**
   `config validate`/실행 시점 리졸브에서 거부된다. Exit 1, 리포트는 `"children": []`로 어떤
   child에도 `up`이 호출되지 않았음을 명시한다.
3. **Rollback 실패** — `api/deploy`는 성공, `web/deploy`가 실패 → `api/deploy`를 rollback하려는
   `down`이(예: 외부에서 이미 컨테이너가 지워진 경우) 실패한다. Primary error는 여전히
   `web-server`의 원래 실패 메시지, `rollback.failed: ["api/deploy"]`,
   `children[0].state: "rollback_failed"`. Exit 1.
4. **재개 가능한 partial state** — 시나리오 3 직후 사용자가 그대로 `dva up release`를 재실행하면
   root는 새로 resolve하고, `api/deploy`가 실제로 아직 안 내려가 있으면(rollback 실패였으므로)
   child 자신의 idempotent up이 "이미 up"으로 처리해 통과하고, `web/deploy`만 다시 시도된다. 새
   명령·새 상태 파일 없이 순수 재실행만으로 재개된다.

### 6. 완료기준 5 — 호환성/마이그레이션

**6.1 기존 local(비-composed) plan.** 변경 없음. `entries:`만 가진 plan은 오늘과 완전히 동일하게
동작한다 — `composes:` 필드가 없으면 이 판정이 추가하는 어떤 규칙도 적용되지 않는다(§3.1의 상호
배타 검사는 두 필드가 동시에 있을 때만 발동한다).

**6.2 기존 imported item 이름.** 변경 없음. `subprojects.<name>.import.plans/interactions/
provision`으로 이미 import된 `project/item` 이름과 alias는 오늘과 동일하게 단독 실행 가능
(`dva up api/deploy`는 계속 그 child 하나만 실행한다) — composition은 그 위에 **새 selector**
(composition plan 이름)를 추가할 뿐, 기존 이름의 의미를 바꾸지 않는다. 유일한 새 제약: 어떤 child
plan이 미래에 자기 자신의 `composes:`를 선언하면(child가 자신의 subproject를 aggregate하는
composition을 만들면), 그 child plan은 §3.3에 의해 더 이상 조상에게 import될 수 없다 — 이는 "기존
유효 설정의 의미를 조용히 재해석"하는 것이 아니라 **새로 composition을 선언하는 쪽에서 새로 걸리는
제약**이므로 기존 설정을 깨지 않는다(오늘 이미 composition을 쓰던 설정이 없으므로 회귀 대상이
없다).

**6.3 실패한 rollout 이후의 rollback.** `dva down release`(사용자가 명시적으로 부르는 down)는
§4.3대로 LIFO로 항상 동작한다 — `up`이 실패해서 자동 rollback이 이미 시도됐든 안 됐든, 사용자가 그
뒤 수동으로 `dva down release`를 부르면 root는 다시 "지금 무엇이 up 상태인가"를 각 child에
물어보고 up 상태인 것만 역순으로 내린다(§5.4의 "child 실행 상태가 진실" 원칙 재사용) — 이미
rollback된 child에 대한 두 번째 `down` 호출은 child 자신의 기존 idempotent down 동작(이미 내려간
것을 다시 내리면 no-op)에 의존한다.

**Before/after 예시:**

*Before(오늘, composition 없음)* — 사용자가 수동으로 두 명령을 순서대로:
```bash
(cd api && dva up deploy)
(cd web && dva up deploy)
```
또는 root에서 import만 쓰던 경우:
```bash
dva up api/deploy
dva up web/deploy
```
둘 다 오늘도 그대로 계속 동작한다(변경 없음).

*After(composition 판정 적용, 미래 구현 카드가 만든 뒤)* — root의 `dva.yml`에 §3 fixture의
`release` composition plan을 추가하면:
```bash
dva up release
```
한 번의 호출로 wave 순서(`api/deploy` → `web/deploy`)를 지키며 실행되고, 실패 시 자동 LIFO
rollback이 적용된다. 기존 `dva up api/deploy` 단독 호출은 여전히 유효하며 `release`의 존재와
무관하게 동일하게 동작한다.

### 7. 완료기준 6 — 이 라운드에서도 하지 않는 것

완료기준 6은 "합성이 선택되면 별도의 bounded 구현 계획(schema·resolver·runtime·CLI·migration·
fixture 카드)을 만들라"고 요구한다. 이 하위 조항은 **이번 라운드에서도 실행하지 않는다** —
완료기준 6 자체가 "독립적인 architecture/operability 리뷰 획득"을 명시적으로 요구하고 있고, 그
리뷰는 사람 또는 이 카드를 작성하지 않은 별도 세션이 §3~§6에서 방금 얼린 상세 계약(특히 §4의 신규
자동 rollback 도입과 아래 "Open questions for review")을 검토한 뒤에 서야 의미가 있다. 지금 같은
세션이 계약을 쓰고 곧바로 스스로 리뷰했다고 표시하는 것은 카드 자신의 "no agent reviews its own
output" 원칙과 충돌한다.

`decision-status`는 이미 `decided`(모델 선택 완료, §1)이며 이 라운드는 그 필드를 다시 바꾸지
않는다. 완료기준 2~5는 이 라운드에서 상세 계약과 fixture로 채워졌으므로 Completion Criteria의
해당 체크박스를 체크한다. 완료기준 6(독립 리뷰 + 구현 계획 카드 생성)은 여전히 미체크로 남기고,
그 리뷰가 통과한 뒤에야 별도 세션이 TASK-256/258류의 구현 계획 카드를 만든다.

### Open question — resolved 2026-09-04

이 라운드에서 진짜로 새 product 선택이 필요했던 지점은 다음 하나로 좁혀졌었다(나머지는 TASK-262/263의
기존 결정이나 이 저장소의 기존 코드/관례를 그대로 연장하는 detail-working-out이었다):

1. **자동 rollback을 끌 수 있는 opt-out이 있어야 하는가? → (b) 채택, 사용자 승인.** §5.1은 실패 시
   자동 LIFO rollback을 기본값으로 얼렸다(단일 project에는 오늘 존재하지 않는, composition이 새로
   도입하는 계약). 초안 시점에는 opt-out 신설이 이 카드의 "CLI 구현 없음" 비목표와 충돌할 수 있어
   (a) 잠정 동결(opt-out 없음)로 남기고 사람에게 재검토를 요청했다. 2026-09-04 사용자가
   `AskUserQuestion`으로 두 대안 — (a) opt-out 없음(단순하지만 rollback이 실패 상태의 사후 조사
   증거를 지워버릴 수 있음) 대 (b) `--no-rollback`을 지금 사전 승인해 향후 구현 카드가 그대로 CLI에
   추가하게 하는 것 — 을 나란히 받고 **(b)를 명시적으로 선택했다**. 이 판단은 뒤집힌 것이 아니라
   두 방향 모두 합리적이라고 판단한 뒤 사람이 실무 디버깅 요구(실패 상태 보존)를 더 무겁게 본
   결과다. 반영: 위 4.4 표에 `--no-rollback` 행 추가, §5.1에 opt-out 존재를 명시. 이 카드는 여전히
   플래그의 이름과 계약(propagate-to-all, opt-out 의미)만 얼리고, 실제 CLI 플래그 파싱·배선
   구현은 비목표로 남긴다 — TASK-260 완료기준 6에서 만들 구현 계획 카드가 이 플래그를 반영해야
   한다.
