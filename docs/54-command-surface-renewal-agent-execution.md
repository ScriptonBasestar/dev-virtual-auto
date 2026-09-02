# Command Surface Renewal 작업의 에이전트 실행 런북

이 문서는 [PLAN-003](../tasks/plan/003-command-surface-renewal-discovery.md)을 새 Codex 세션에서
실행할 때의 세션 경계, 모델 라우팅, dependency wave, 사람 승인 지점과 시작 프롬프트를 소유한다.
제품 계약과 acceptance criteria는 PLAN-003과 target task card가 정본이다. PLAN-002 전용
[기존 런북](53-command-surface-agent-execution.md)의 task graph, prompt와 guardrail은 이 계획에
적용하지 않는다.

## 1. 실행 단위

Fresh writable session 하나는 task 하나만 소유한다. Discovery 또는 decision 조사에 서브에이전트를
병렬 활용할 수 있지만, 여러 root session이 같은 source surface를 동시에 통합하지 않는다. 현재 task
밖의 결함은 blocker나 새 card 제안으로 남기고 자동으로 scope를 넓히지 않는다.

각 세션이 지키는 단계 순서는 아래 §6 공통 프롬프트가 정본이며 여기에 중복해 두지 않는다.

## 2. 역할과 런타임 매핑

이 런북은 벤더 에이전트 이름이 아니라 **역할**을 계약으로 삼는다. 어떤 런타임에서 이어받아도 아래
경계가 유지되어야 하며 여러 역할을 하나로 합치는 대체는 허용하지 않는다. 표에 없는 런타임은 같은
경계를 만족하는 수단을 고르고 그 선택을 세션 보고에 남긴다.

| 역할 | 범위와 경계 | Codex | Claude Code |
| --- | --- | --- | --- |
| Root | 최종 판단·통합. 요약이 아니라 실제 파일·diff·명령 결과로 재확인 | `gpt-5.6-sol` high/max | 최상위 tier 세션 |
| 조사 | command owner, corpus, code path, compatibility. 수집만 하고 결정·편집 금지 | `ce-scanner`, `ce-explorer` | `Explore` |
| 구현 | 결정이 닫힌 bounded 구현. 승인된 contract 밖을 정리하지 않음 | `ce-worker`, `spark-worker` | `patterns:backend-dever` |
| 판정 | public contract·operability·actual-diff review. 자기 변경은 검토 금지 | `ce-judge` | `core:code-reviewer` |

Public CLI, compatibility, composition 또는 migration 결정을 다루는 TASK-255·257·260·261·263은 가장
강한 설정을 고려한다. Codex에서 Spark가 unavailable·quota·rate-limit이면 `luna-worker`로 1회 대체한다.
요구한 tier나 역할을 제공할 수 없으면 조용히 낮은 tier로 대체하지 말고 review separation과 판단
품질을 보존할 수 있는지 먼저 보고한다. 구현과 리뷰는 같은 agent에게 맡기지 않으며 subagent summary는
lead이고 root가 실제 code, diff, pinned revision과 gate 결과를 확인한다.

## 3. Dependency wave

| Wave | Task | 종료 조건 |
| --- | --- | --- |
| 완료 | [TASK-262](../tasks/_archive/done/262-restore-imported-plan-execution.md) | imported-plan owner contract 복구·독립 review·통합 완료 |
| 완료 | [TASK-253](../tasks/_archive/done/253-align-help-groups-and-discovery-descriptions.md) | help 정비 구현·독립 review·통합 완료 |
| 완료 | [TASK-264](../tasks/_archive/done/264-restore-imported-command-ownership.md) | imported interaction/provision owner 복구·독립 review·통합 완료 |
| 1 | [TASK-254](../tasks/todo/254-discover-command-metadata-registry.md) | evidence와 recommendation이 card에 기록·통합 |
| 1 | [TASK-259](../tasks/todo/259-discover-qualified-project-addressing.md) | addressing evidence와 recommendation이 기록·통합 |
| 2 | [TASK-255](../tasks/todo/255-decide-kubectl-route-compatibility.md) | TASK-254 근거를 사용한 사람 승인 decision 기록 |
| 2 | [TASK-257](../tasks/todo/257-decide-validate-route-compatibility.md) | TASK-254 근거를 사용한 사람 승인 decision 기록 |
| 2 | [TASK-263](../tasks/todo/263-decide-qualified-project-addressing.md) | TASK-259 근거와 TASK-264 owner 복구 위에서 address/exposure 사람 승인 기록 |
| 3 | [TASK-256](../tasks/todo/256-implement-kubectl-route-decision.md) | TASK-255 결정 구현·통합 |
| 3 | [TASK-258](../tasks/todo/258-implement-validate-route-decision.md) | TASK-257 결정 구현·통합 |
| 3 | [TASK-260](../tasks/todo/260-freeze-cross-project-plan-composition.md) | TASK-262·263 위에서 composition 사람 승인 기록 |
| 4 | [TASK-261](../tasks/todo/261-decide-vnext-vocabulary-and-migration.md) | 선행 결과를 근거로 사람 승인 또는 현행 유지 기록 |

TASK-262·253·247·264는 완료됐다. 다음 변경 세션은 PLAN-002의 TASK-265→248이다.
그 뒤 TASK-254와 product critical path TASK-259→263→260을 진행한다. TASK-255·257은 TASK-254가
manifest route identity의 소유권을 조사한 뒤 시작한다. 서로 독립인 조사도 source integration은 최신
source tip에 대해 직렬화한다.

## 4. Session 종류별 stop condition

Discovery task(TASK-254·259)는 code/schema/route를 바꾸지 않는다. 조사 결과는 target card의
`## Evidence and Recommendation`이 canonical record가 되며, 근거가 크면 tracked artifact를 만들고
card에서 링크한다.

Decision task(TASK-255·257·260·261·263)는 상호 배타적 option, 권장안, current evidence, compatibility,
migration, rollback, failure fixture와 남은 불확실성을 완성한 뒤 사용자 선택에서 멈춘다. 승인 후 card에
`## Decision Record`를 추가하고 `decision-status: decided`로 바꾼다. 그 전에는 dependent production
code, schema, command registration 또는 vNext plan을 만들지 않는다.

Implementation task(TASK-256·258·264)는 승인된 contract 밖의 정리를 함께 하지 않는다. TASK-264는
documented child-owner contract만 복구하고 TASK-247 failure policy를 구현하지 않는다. 선행 decision이
현재 route 유지를 선택한 경우 불필요한 alias를 만들지 않고, 결정이 특정한 test/documentation gap만
닫는다. TASK-256·258이 현재 manifest로 표현할 수 없는 route identity를 필요로 하면 TASK-254가 요구한
bounded child를 만든다. 그 변경에서 PLAN-003의 `children`, `total-tasks`, graph와 완료 정의를 갱신하고
영향을 받는 TASK-256·258의 `depends-on`에 child를 추가한 뒤 멈춘다. Child가 통합되기 전에는 schema
변경이나 해당 구현 task 완료를 시작하지 않는다.

Dependency 미완료, 사람 승인 부재, stale/unpinned external evidence, source conflict, task-relevant gate
failure, route collision 의미 불명확, destructive lifecycle flag scope 미정에서는 즉시 멈춘다.
`ce task preflight`의 runnable 표시는 card 형식 준비도이며 `depends-on`, `needs-human` 또는
`decision-status` 승인을 대신하지 않는다. 이 런북과 frontmatter를 직접 확인한다.

## 5. 검증과 Git 완료

Build, test, service 또는 log 작업 전에는 repository `dva` skill을 읽고 `dva manifest -f json`으로
실행 표면을 찾는다. DVA에 같은 workflow가 없을 때만 raw tool을 사용한다. Target criterion과 관련
repository gate가 exit 0이어야 하고, blocker 수정 뒤 independent focused re-review가 PASS해야 한다.

대화형 실행은 위와 같이 `dva`를 쓰지만 **card의 `verify:` binding은 `make` target으로 쓴다**. Binding은
clean checkout·CI·reviewer 환경에서 재현되어야 하는데 `dva` 경로는 설치된 바이너리에 의존하고, DVA
자신의 config 로딩이나 route 해석을 고치는 task에서는 수정 전 바이너리로 결과를 검증하는 순환이 된다.

완료는 task commit만 뜻하지 않는다. Task branch push, current source tip 재검증, configured source
branch direct integration과 push, task worktree·local branch·remote branch·빈 parent 정리까지 수행한다.
PR/MR은 만들지 않는다.

## 6. 새 세션 공통 프롬프트

아래 블록에서 `TARGET_TASK`만 바꾼다. 현재 첫 권장 실행값은 PLAN-002 런북의 `TASK-265`이다.
이 PLAN-003 프롬프트는 `TASK-254`부터 다시 쓴다.

```text
TARGET_TASK: TASK-254
PLAN: tasks/plan/003-command-surface-renewal-discovery.md
RUNBOOK: docs/54-command-surface-renewal-agent-execution.md

이 세션은 TARGET_TASK 하나만 소유한다. PLAN 전체나 dependent task를 선행 구현하지 마라.

1. 가장 가까운 AGENTS.md, RUNBOOK, PLAN과 TARGET_TASK를 끝까지 읽고 dependency,
   decision status, acceptance criteria, canonical record 위치와 stop condition을 먼저 보고하라.
2. 첫 편집 전에 Global Git policy에 맞는 별도 task worktree를 만들고 primary/source checkout을
   보존하라. Source tip과 기존 PLAN-002의 관련 변경을 확인하되 PLAN-002 계약을 수정하지 마라.
3. RUNBOOK의 모델·role routing을 적용하라. 최대 3개, 하위 위임 금지, 구현자와 reviewer 분리다.
   Subagent summary가 아니라 root가 actual file, diff, pinned evidence와 command result를 검증하라.
4. Discovery task는 evidence/recommendation만 기록하고 production surface를 바꾸지 마라.
   Decision task는 option/evidence/tradeoff/rollback/fixture를 준비한 뒤 사용자 선택에서 멈춰라.
   승인 전 decision-status나 dependent code/schema/command를 확정하지 마라.
5. Implementation task는 승인된 decision과 target scope만 구현하라. 새 namespace, reserved-name
   약화, run 제거, 자동 project reachability 또는 composition을 선행하지 마라.
6. Target criterion과 relevant repository gate를 실제로 실행하라. 별도 reviewer의 actual-diff
   PASS 뒤 root가 재검증하라.
7. Task/source push와 direct integration, worktree/local/remote task branch cleanup까지 끝낸 뒤
   acceptance, 변경, 검증, review, commit, push/cleanup과 다음 runnable task를 보고하라.

지금은 TARGET_TASK만 수행하라.
```

Prompt보다 repository policy와 target card의 구체 criterion이 우선한다.
