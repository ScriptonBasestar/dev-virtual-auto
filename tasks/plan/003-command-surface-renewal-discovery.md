---
id: PLAN-003
title: "Renew command discovery and prepare composition contracts"
type: plan
scope: "help and machine discovery, kubectl and validate route compatibility, project addressing, cross-project composition, and vNext vocabulary decisions"
progress: 43
total-tasks: 14
completed-tasks: 6
children: [TASK-253, TASK-254, TASK-255, TASK-256, TASK-257, TASK-258, TASK-259, TASK-260, TASK-261, TASK-262, TASK-263, TASK-264, TASK-267, TASK-272]
target-date: "2027-03-31"
created: 2026-09-02
---

## Goal

현재 command contract를 깨지 않으면서 사람이 읽는 help와 machine discovery의 품질을 높이고,
`ktl`·`validate` route의 compatibility를 증거로 판정하며, qualified project addressing과
cross-project plan composition을 구현 가능한 계약으로 만든다. 마지막으로 그 결과를 근거로
vNext vocabulary와 migration commitment를 선택한다.

이 계획은 renewal 토론 자료를 현재 코드와 기존 PLAN-002에 대조해 만든 self-contained 실행 단위다.
Ignore된 `tmp/` 자료는 역사적 입력일 뿐 clean checkout에서 필요한 실행 의존성이 아니다. Renewal의
큰 방향은 검토 대상으로 보존하되, `plan`/`exec`/`tool` namespace나 `/` 주소 문법을 이미 승인된
제품 계약처럼 구현하지 않는다.

| Workstream | 판정 | 작업 |
| --- | --- | --- |
| help group과 discovery 설명 | 기존 contract 안에서 개선 완료 | [TASK-253](../_archive/done/253-align-help-groups-and-discovery-descriptions.md) |
| command metadata 중복 | 구현 전 소유권 조사 | [TASK-254](../done/254-discover-command-metadata-registry.md) |
| `ktl`/`kubectl` route | evidence decision 후 구현 | [TASK-255](../todo/255-decide-kubectl-route-compatibility.md) → [TASK-256](../todo/256-implement-kubectl-route-decision.md) |
| `validate` route | evidence decision 후 구현 | [TASK-257](../todo/257-decide-validate-route-compatibility.md) → [TASK-258](../todo/258-implement-validate-route-decision.md) |
| manifest route identity | TASK-254 증거로 조건 발생, 구현 전 사람 결정 | [TASK-272](../todo/272-freeze-manifest-route-identity.md) |
| imported plan 실행 | advertised contract 복구 완료 | [TASK-262](../_archive/done/262-restore-imported-plan-execution.md) |
| imported interaction/provision 실행 | owner 복구 완료 | [TASK-264](../_archive/done/264-restore-imported-command-ownership.md) |
| qualified project addressing | discovery와 사람 결정을 분리 | [TASK-259](../done/259-discover-qualified-project-addressing.md) → [TASK-263](../todo/263-decide-qualified-project-addressing.md) |
| cross-project composition | 기존 import와 address contract 이후 판단 | [TASK-260](../todo/260-freeze-cross-project-plan-composition.md) |
| vNext vocabulary | 앞선 evidence를 모은 최종 결정 | [TASK-261](../todo/261-decide-vnext-vocabulary-and-migration.md) |

## Current status and review corrections (2026-09-02)

TASK-262는 imported plan lifecycle을 복구했고 TASK-253은 help discovery를 정렬했다. TASK-247 caller
audit에서 interaction/provision owner 결함이 확인되어 TASK-264를 추가했고 복구는 완료됐다.
저장소 전체의 다음은 PLAN-002 TASK-265→248이며, 그 뒤 TASK-254로 복귀한다. 이어서 P0 critical path인
TASK-259→263→260을 진행한다. TASK-255·257은 TASK-254 뒤에 시작한다.

TASK-262를 추가한 역사적 이유는 문서가 지원한다고 설명한 imported plan이 normal child stack에서
실패했기 때문이다. 당시 loader는 child plan만 parent `Plans`에 clone하고 resolver는 parent `Stack`에서
entry를 찾았다. 이 결함과 독립 리뷰 지적은 2026-09-02에 TASK-262 회귀 테스트로 닫혔으므로 별도
runtime 후속 카드를 만들지 않는다. TASK-244의 owner-partition criterion은 별도로 D6 경고가
root와 child 또는 서로 다른 child의 모양만 같은 plan을 중복으로 오인하지 않게 한다.

또한 project address grammar와 plan composition은 독립된 public contract다. TASK-259는 evidence만
수집하고 TASK-263이 address/exposure를 사람 결정으로 닫은 뒤, TASK-260이 composition 의미만 결정한다.
Manifest가 canonical/compatibility route를 표현할 schema가 필요하다고 TASK-254·255·257에서 판정되면
bounded child를 먼저 만들며 TASK-256·258 안에서 ad-hoc schema를 추가하지 않는다. TASK-254가 2026-09-03에
이 조건을 판정했다. `ManifestCmd`는 `description`/`type`/`options`/`subcommands` 네 필드뿐이고
`static_commands`는 이름 하나로 keying되므로 두 route가 같은 command라는 사실을 표현할 수단이 없으며,
측정된 manifest는 `subcommands`를 `skill`에만 채워서 TASK-257이 선택 대상으로 삼는 `config validate`
자체가 문서에 없다. 따라서 [TASK-272](../todo/272-freeze-manifest-route-identity.md)을 만들고 같은
변경에서 PLAN-003의 `children`, `total-tasks`, `progress`, graph, 완료 정의와 TASK-256·258의
`depends-on`을 갱신했다. TASK-272이 닫히기 전에는 PLAN-003이나 해당 구현 task를 완료할 수 없다.

권장 방향은 다음과 같다.

- Imported plan은 parent stack을 암묵 flatten하지 않고 owning child effective config, `env_file`과 working
  directory로 해석한다. Canonical/alias lifecycle route는 같은 child owner를 보존한다.
- Direct interaction의 machine route는 `run --project`, human shorthand는 `project:item`, explicit import의
  canonical name은 `project/item`을 유지한다. 자동 registration과 separator 통일은 채택하지 않는다.
- Composition은 arbitrary recursive include보다 root가 exposed child plans를 명시적으로 aggregate하는
  제한된 v1을 권장한다.
- Vocabulary는 current-compatible evolution을 기본으로 하고 `plan`/`exec`/`tool` namespace와 대규모
  noun rename을 도입하지 않는다.

## 1. 기존 계획과의 경계

[PLAN-002](002-command-surface-delivery.md)는 D6/D7, secure env bridge, required-env policy,
capability-driven init, env promotion gate를 소유한다. 이 계획은 그 판정을 수정하지 않으며 두 계획은
독립적으로 시작할 수 있다. 같은 command registry나 reserved-name 영역을 수정하는 구현은 source tip에
rebase한 뒤 현재 contract를 다시 확인하지만, PLAN-003이 PLAN-002의 미완료 결정을 선점하지 않는다.

특히 다음 PLAN-002 판정은 그대로 유지한다.

- lifecycle 7동사와 plan 위치 인자를 유지한다.
- 새 top-level reservation은 별도 compatibility evidence 없이 추가하지 않는다.
- reserved interaction collision은 hard error이고 `run`은 명시적 escape route다.
- env bridge, required-env policy, init scaffold, env promotion은 PLAN-002 child가 소유한다.

## 2. 현재 기준선과 renewal 판정

현재 route는 top-level lifecycle verbs, 명시적 `run`, `config` group, `compose`/`ktl`/`ssh` integration
surface로 구성된다. Direct **interaction** subproject selection은 `:` 또는 `run --project`를 사용하고,
imported child item은 `/` canonical name을 사용한다. 이 설명은 lifecycle과 provision에 `:` 또는
`--project`가 적용된다는 뜻이 아니다. Plan schema에는 다른 plan을 include하거나 여러 project plan을
합성하는 contract가 없다.

Renewal에서 채택하는 것은 다음 원리다.

1. 사람이 쓰는 shorthand와 자동화가 쓰는 explicit machine route를 구분한다.
2. help, manifest, completion, reserved-name 검사는 같은 public surface를 일관되게 설명해야 한다.
3. cross-project composition은 유용한 제품 방향이지만 identity, merge, dependency, failure contract를
   먼저 닫는다.
4. 이름 변경은 discoverability만으로 승인하지 않고 실제 invocation corpus, compatibility 기간,
   rollback 비용으로 판정한다.

다음 제안은 아직 채택하지 않는다.

- `dva plan`, `dva exec`, `dva tool` namespace 신설
- `run` 제거 또는 rename
- reserved-name collision을 warning으로 약화
- `/` project grammar, 자동 reachability, plan include, target vocabulary의 선행 구현
- 사용 증거 없는 `ktl` 또는 `config validate` 제거·숨김
- command hook을 임의 분리하거나 기존 collision ownership을 변경

## 3. 작업 graph와 시작점

```text
TASK-262  imported-plan repair (done) ─────────────────────────┐
TASK-253  help/discovery descriptions (done)                   │
TASK-247 decision ─> TASK-264 owner repair ─┐
                   └> TASK-265 decision ───┴─> TASK-248 (PLAN-002)
TASK-254  metadata ownership ─┬─> TASK-255 ─> TASK-256 ────────┤
                              ├─> TASK-257 ─> TASK-258 ────────┼─> TASK-261 vNext decision
                              └─> TASK-272 gates 256·258       │
TASK-259 discovery + TASK-264 ─> TASK-263 address decision ──┴─> TASK-260 composition decision ┘
```

TASK-254·259는 dependency 없이 착수 가능하지만 변경을 통합하는 root session은 직렬화한다.
TASK-264→TASK-265→TASK-248 뒤 다음 discovery는 TASK-254, product critical path의 첫 조사는 TASK-259를 권장한다.
TASK-255·257은 TASK-254 뒤에만 시작한다. TASK-272은 TASK-254 뒤에 시작하고 TASK-256·258의 manifest
변경을 gate한다. TASK-263은 TASK-259 evidence만으로 시작하지 않는다.
Imported interaction/provision owner가 복구되기 전 addressing을 결정하면 잘못된 현행 기준선 위에서
route를 고르게 되므로 TASK-264도 함께 닫혀야 한다. TASK-255·257·260·261·263은
`needs-human` decision card이므로 evidence와 independent review가 끝나도 사람이 선택을 기록하기 전에는
후속 public contract를 구현하지 않는다. TASK-256·258은 앞선 결정이 현재 route 유지로 닫히면 불필요한
alias를 만들지 않고, 결정이 요구한 문서·검증 정리만 수행한 뒤 완료할 수 있다. TASK-260은 TASK-262와
TASK-263이 모두 완료된 뒤 시작한다.

## 3-1. 착수 가능 상태 (2026-09-03)

위 graph는 시작 조건을 규칙으로 적었을 뿐 지금 무엇이 열려 있는지는 말하지 않는다. 선행 카드의
현재 위치를 실제로 조회해 정리한 결과는 다음과 같다 (`find tasks -name '<id>-*.md'` — 완료 카드
일부는 `tasks/_archive/done/`에 있어 `tasks/todo`·`tasks/done`만 보면 존재하지 않는 것처럼 읽힌다.
TASK-262·264가 그 경우이며, 실종된 카드가 아니라 완료 후 보관된 카드다).

| 카드 | 선행 | 상태 |
| --- | --- | --- |
| TASK-255 | TASK-254 ✅ | **착수 가능** |
| TASK-257 | TASK-254 ✅ | **착수 가능** |
| TASK-272 | TASK-254 ✅ | **착수 가능** — 닫히면 TASK-256·258이 함께 열린다 |
| TASK-263 | TASK-259 ✅ · TASK-264 ✅ | **착수 가능** — 닫히면 TASK-260이 열린다 |
| TASK-256 | TASK-255 · TASK-272 | 대기 |
| TASK-258 | TASK-257 · TASK-272 | 대기 |
| TASK-260 | TASK-262 ✅ · TASK-263 | TASK-263 대기 |
| TASK-261 | TASK-254 ✅ · TASK-256 · TASK-258 · TASK-260 | 대기 (이 계획의 마지막 카드) |

**착수 가능한 네 장은 전부 `needs-human` decision card다.** §4가 정하듯 세션은 evidence와 option
matrix를 재현 가능한 형태로 완성할 수 있지만, 사람이 선택을 기록하기 전에는 후속 public contract를
구현하지 않는다. 즉 세션 혼자 끝까지 닫을 수 있는 카드는 이 계획에 지금 없다 — 넷 다 "증거를
갖추고 선택을 대기 상태로 남기는" 것이 완료 지점이다.

병렬성 관점에서 TASK-272를 먼저 닫는 것이 회수가 가장 크다. 혼자서 구현 카드 두 장(256·258)을
동시에 gate하고 있고, 나머지 세 장 중 유일하게 다른 카드의 선행이면서 route 결정 자체는 아니다.

## 4. 실행 규칙

- 각 discovery/decision card는 외부 토론 자료 없이 현재 code, tracked documentation, pinned corpus와
  명시된 option matrix로 재현 가능해야 한다.
- 외부 repository evidence는 canonical repository ID와 commit SHA를 기록하고, dynamic invocation을
  완전하게 증명할 수 없는 한계를 finding으로 남긴다.
- 구현과 independent review를 분리한다. Public route·schema·migration 결정은 strong-tier review와
  사람 승인을 거친다.
- 기존 alias가 공존하는 동안 canonical name과 compatibility name은 모두 예약하고, help, manifest,
  completion, debug mode, exit/signal behavior가 갈라지지 않게 한다.
- vNext foundational schema/runtime implementation은 이 계획에 넣지 않는다. TASK-260 또는 TASK-261이
  이를 선택하면 exact contract를 가진 별도 plan과 bounded child cards를 만든다.
- TASK-254가 manifest route identity를 현행 schema로 표현할 수 없다고 판정하면 그 판정을 기록하는
  변경이 manifest contract child의 생성과 PLAN/task dependency 현행화까지 함께 소유한다.

세션 경계, 모델 라우팅, 서브에이전트 역할, task별 stop condition과 시작 프롬프트는
[Command Surface Renewal 작업의 에이전트 실행 런북](../../docs/54-command-surface-renewal-agent-execution.md)을
따른다. PLAN-002 전용 [기존 런북](../../docs/53-command-surface-agent-execution.md)의 wave, prompt와
guardrail은 PLAN-003에 적용하지 않는다.

## 5. 완료 정의

PLAN-003은 TASK-253~264, TASK-267, 그리고 TASK-254 판정으로 추가된 manifest route identity child
TASK-272이 모두 닫히고, 선택된
incremental compatibility 구현이 source branch에서 검증되며, project addressing·composition·vNext
vocabulary의 선택과 기각 근거가 기록됐을 때 완료한다.
vNext 구현을 선택한 경우 새 plan과 child cards를 만드는 것까지가 이 계획의 완료 범위이며, 그 구현
자체는 새 계획의 완료 조건이다.

## Children

- TASK-253 — align help groups and discovery descriptions
- TASK-254 — discover a command metadata registry boundary
- TASK-255 — decide the kubectl canonical route and ktl compatibility
- TASK-256 — implement the approved kubectl route decision
- TASK-257 — decide the canonical validate route and compatibility
- TASK-258 — implement the approved validate-route decision
- TASK-259 — discover qualified project addressing
- TASK-260 — freeze the cross-project plan-composition contract
- TASK-261 — decide vNext vocabulary and migration commitment
- TASK-262 — restore imported-plan execution against the owning project
- TASK-263 — decide qualified-project addressing and exposure
- TASK-264 — restore imported interaction and provision execution ownership
- TASK-267 — repair grammar-independent subproject exposure defects
- TASK-272 — freeze the manifest command route-identity representation
