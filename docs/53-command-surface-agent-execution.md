# Command Surface 작업의 에이전트 실행 런북

이 문서는 [PLAN-002](../tasks/plan/002-command-surface-delivery.md)를 새 Codex 세션에서
실행할 때의 세션 경계, 모델 라우팅, 서브에이전트 역할, 중단 조건과 시작 프롬프트를 소유한다.
제품·보안 계약과 task acceptance criteria는 PLAN-002와 각 task card가 정본이며 여기서 반복하지
않는다. 이 런북은 **어떻게 실행할지**만 정한다.

## 1. 운영 결정

PLAN-002 전체를 한 장기 세션에 맡기지 않는다. **Dependency wave는 순서를 정하고, 변경을 만드는
fresh session은 task 하나만 소유한다.** 조사 서브에이전트는 한 세션 안에서 병렬 실행할 수 있지만
여러 writable root session을 동시에 운영하지 않는다.

한 세션은 다른 task를 “조금 함께” 고치지 않는다. Cross-cutting 결함을 발견하면 현재 task의 blocker로
기록하거나 새 task를 제안하고, scope를 자동 확장하지 않는다.

## 2. 모델과 역할 라우팅

2026-09-02 기준 [OpenAI model guidance](https://developers.openai.com/api/docs/guides/latest-model)는
`gpt-5.6-sol`을 flagship, `gpt-5.6-terra`를 intelligence/cost 균형, `gpt-5.6-luna`를 효율적인
high-volume 모델로 설명한다. `max` reasoning은 hardest quality-first workload에 유보한다.

PLAN-002는 public CLI, secret safety, filesystem atomicity와 cross-repository promotion을 함께
다루므로 root session의 권장값은 **`gpt-5.6-sol` + `max` reasoning**이다. 대체가 필요하면 먼저
`high`를 비교하며 Terra나 Luna를 root 판단 모델로 조용히 바꾸지 않는다.

| 역할 | 에이전트·모델 | 책임 |
| --- | --- | --- |
| Root | `gpt-5.6-sol`, `max` | 최종 판단, cross-cutting diff, 검증, 통합 |
| 기계 조사 | `ce-scanner` (`gpt-5.6-luna`, low) | 호출처·설정 shape·gate inventory |
| 읽기 분석 | `ce-explorer` (`gpt-5.6-terra`, low) | 코드 경로·호환성·의존 관계 분석 |
| 분리 구현 | `ce-worker` (`gpt-5.6-terra`, medium) | 소유 파일이 분리된 bounded 구현 |
| 독립 판정 | `ce-judge` (`gpt-5.6-sol`, high) | public contract·security·실제 diff review |
| 일반 bounded 작업 | `spark-worker` | specialist role이 지정되지 않은 작업 |
| Spark fallback | `luna-worker`, medium | Spark unavailable/quota/rate-limit일 때 동일 작업 1회 |

동시에 실행하는 서브에이전트는 최대 3개이며 하위 위임은 금지한다. 구현자와 reviewer는 분리한다.
최종 cross-cutting 판단은 root가 소유한다. Subagent summary는 lead일 뿐이며 root가 실제 파일,
diff와 명령 결과를 다시 확인한다.

## 3. Dependency wave

각 항목은 별도 writable session이며 source branch 통합은 직렬로 수행한다.

| Wave | Task | 세션 종료 조건 |
| --- | --- | --- |
| 1 | [TASK-245](../tasks/todo/245-freeze-env-bridge-contract.md) | 사용자 결정이 기록·검증됨 |
| 완료 | [TASK-247](../tasks/_archive/done/247-freeze-required-env-command-policy.md) | caller matrix 사용자 결정·검증·통합 완료 |
| 1 | [TASK-249](../tasks/todo/249-redesign-capability-driven-init.md) | init contract 사용자 결정이 기록·검증됨 |
| 1 | [TASK-244](../tasks/todo/244-validate-plan-declaration-drift.md) | D6/D7 구현·독립 review·통합 완료 |
| 선행 | [TASK-264](../tasks/todo/264-restore-imported-command-ownership.md) | imported interaction/provision owner 복구·통합 완료 |
| 1 | [TASK-265](../tasks/todo/265-decide-interaction-env-file-contract.md) | inert interaction env_file 사람 결정 기록 |
| 2 | [TASK-248](../tasks/todo/248-enforce-required-env-command-policy.md) | TASK-247·264·265 위에서 loader safety 구현·통합 완료 |
| 2 | [TASK-250](../tasks/todo/250-implement-capability-driven-init.md) | TASK-244·249 결과 구현·통합 완료 |
| 3 | [TASK-246](../tasks/todo/246-implement-secure-config-env-bridge.md) | TASK-245 결정과 TASK-248 loader contract 위에서 구현·통합 완료 |
| 4 | [TASK-252](../tasks/todo/252-decide-top-level-env-promotion.md) | 영구 유지 결정 또는 promotion evidence 요청이 기록됨 |
| 조건부 | [TASK-251](../tasks/todo/251-build-env-migration-evidence-gate.md) | TASK-252가 요청한 경우 gate 통합, 아니면 N/A 종료 |
| 조건부 재개 | [TASK-252](../tasks/todo/252-decide-top-level-env-promotion.md) | TASK-251 evidence를 사용한 최종 결정 기록 |

TASK-247 뒤 PLAN-003 런북으로 TASK-264를 복구하고 TASK-265를 결정한 다음 TASK-248로 돌아온다.
TASK-246은 TASK-248이 통합되기 전 시작하지 않는다.

TASK-252의 권장안은 TASK-251을 선행 구현하지 않고 permanent `config env`로 닫는 것이다. Promotion
가치 조사를 사람이 선택하면 TASK-252를 pending으로 유지한 채 TASK-251을 실행하고 같은 decision을
재개한다. 최종적으로 promotion을 선택해도 그 세션은 reservation을 구현하지 않는다. Exact route,
compatibility, rollback과 pinned-corpus 재검증을 소유하는 새 child card를 만들고 PLAN-002를 갱신한 뒤
종료한다.

## 4. 한 세션의 상태 전이

1. 가장 가까운 `AGENTS.md`, PLAN-002와 target task 전체를 읽는다.
2. Dependency·decision status를 검사하고 별도 task worktree와 stop condition을 확정한다.
3. 최대 3개의 독립 단위만 위임하고, decision task는 사용자 선택에서 멈춘다.
4. 결정된 contract만 구현하고 task criterion을 실행한다.
5. 별도 reviewer와 root가 실제 diff와 relevant gate를 확인한다.
6. Task/source push, current source tip 재검증, direct integration과 cleanup을 완료한다.

Dependency·사용자 결정 부재, conflict, gate 실패, secret/filesystem/OS safety 불명확, stale·ambiguous
external evidence나 SHA drift, task 밖 public contract 변경 필요에서는 즉시 멈춘다.
`ce task preflight`의 runnable 표시는 card 형식 준비도이며 `depends-on`, `needs-human` 또는
`decision-status` 승인을 대신하지 않는다. 이 런북과 frontmatter를 직접 확인한다.

## 5. Decision session 규칙

`needs-human: true`인 task에서 에이전트는 결정을 준비하지만 대신 확정하지 않는다. 사용자에게 요청하기
전까지 다음을 완성한다.

- 상호 배타적인 선택지와 권장안
- 현재 코드·문서·fixture 근거
- compatibility, security와 migration tradeoff
- text/JSON/exit 또는 filesystem state matrix
- 검증 가능한 acceptance fixture
- 기각 대안과 남은 불확실성

사용자 선택 전에는 `decision-status`를 확정하거나 dependent production code, schema 또는 command
registration을 변경하지 않는다. 선택 후 같은 session을 이어서 decision card를 정본으로 만들고 검증·
통합한다.

## 6. 변하지 않는 guardrail

세부 계약은 [PLAN-002 §4](../tasks/plan/002-command-surface-delivery.md#4-env-bridge의-비협상-수용-조건),
[§5](../tasks/plan/002-command-surface-delivery.md#5-promotion-evidence-contract)와 target task card가 소유한다.
Root는 변경 전에 해당 조항을 확인하고, 완화·선행·우회가 필요해지면 scope를 넓히지 말고 멈춘다.

## 7. 검증과 Git 완료 조건

Build, test, service, log 작업 전에는 repository `dva` skill을 읽고 `dva manifest -f json`으로 실행
표면을 찾는다. DVA에 동등한 workflow가 없을 때만 raw tool을 사용한다.

Task acceptance binding과 변경에 해당하는 repository gate가 모두 exit 0이어야 한다. Reviewer는 구현자와
다른 agent여야 하며 actual diff를 읽는다. Blocker/major를 수정한 뒤 focused re-review가 PASS가 아니면
통합하지 않는다.

Git 완료는 task commit에서 끝나지 않는다. Task branch 즉시 push, current source tip에 대한 재검증,
configured source branch direct integration과 push, task worktree·local branch·remote branch·빈 parent
정리까지 하나의 완료 조건이다. PR/MR은 만들지 않는다.

## 8. 새 세션 시작 프롬프트

`TARGET_TASK`만 바꾸어 매 session 사용한다. TASK-264 완료 뒤 재개 값은 `TASK-265`다.

```text
TARGET_TASK: TASK-265
PLAN: tasks/plan/002-command-surface-delivery.md
RUNBOOK: docs/53-command-surface-agent-execution.md

이 세션은 TARGET_TASK 하나만 소유한다. PLAN 전체나 다음 task를 선행 구현하지 마라.

1. 가장 가까운 AGENTS.md, RUNBOOK, PLAN과 TARGET_TASK를 끝까지 읽고 dependency,
   decision status, acceptance criteria와 stop condition을 먼저 보고하라.
2. Global Git policy대로 첫 편집 전에 별도 task worktree를 만들고 primary/source checkout을
   보존하라. 코드 작업은 저장소가 지정한 문서 loading order를 지켜라.
3. RUNBOOK의 agent routing을 적용하라. 최대 3개, 하위 위임 금지, 구현자와 reviewer 분리다.
   Subagent summary 대신 root가 실제 file, diff와 command result를 검증하라.
4. needs-human task는 option/evidence/tradeoff/fixture-ready contract를 만든 뒤 사용자 선택에서
   멈춰라. 승인 전 decision-status나 dependent code/schema/command를 확정하지 마라.
5. PLAN-002 guardrail을 완화하거나 다음 task를 선행하지 마라. Conflict, 관련 gate 실패,
   secret risk, 사용자 결정 부재 또는 external SHA drift에서 멈춰라.
6. Task criterion과 relevant repository gate를 실제로 실행하라. 별도 reviewer의 actual-diff
   PASS 뒤 root가 재검증하라.
7. Task/source push와 direct integration, worktree/local/remote task branch cleanup까지 끝낸 뒤
   acceptance, 변경, 검증, review, commit, push/cleanup과 다음 runnable task를 보고하라.

지금은 TARGET_TASK만 수행하라.
```

Prompt의 반복 규칙보다 repository policy와 target task의 구체 criterion이 우선한다. 모델 이름이나
agent role이 runtime에서 제공되지 않으면 조용히 대체하지 말고, 동일한 capability와 review separation을
보존할 수 있는지 먼저 보고한다.
