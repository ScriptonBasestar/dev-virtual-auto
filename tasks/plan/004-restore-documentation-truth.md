---
id: PLAN-004
title: "Restore documentation truth across skills, flows, and the example corpus"
type: plan
scope: "CLI advice strings, agent-mesh flow prompt claims, skill reference fictions, example corpus defects, and the markdown-YAML validation gap"
progress: 0
total-tasks: 4
completed-tasks: 0
children: [TASK-273, TASK-274, TASK-275, TASK-276]
target-date: "2026-11-30"
created: 2026-09-03
---

## Goal

`docs/43` command-surface 개편과 이후 기능 추가가 코드에는 반영됐지만, 사용자와 agent가 실제로
읽는 표면 — CLI가 출력하는 조언 문자열, `agent-mesh-flows/`의 프롬프트 본문, `skills/dva/`의
reference 문서, `examples/` corpus — 에는 제거된 명령·존재하지 않는 플래그·파싱되지 않는 config
모양이 남아 있다. 이 계획은 그 표면을 구현과 일치시킨다.

**런타임 동작은 바꾸지 않는다.** 네 카드 전부 "문서를 코드에 맞춘다"이며 그 반대가 아니다.
유일한 예외는 TASK-276의 markdown-YAML 게이트 추가로, 이는 동작 변경이 아니라 검증 확장이다.

## 근본 원인

드리프트는 무작위가 아니라 하나의 패턴에서 반복 발생했다 — **AUTOGEN 블록은 정확한데 손으로
쓴 사본이 낡는다.** 예약어 목록, doctor env-file 안내, flow 프롬프트의 예약 명령 목록에서 세 번
같은 방식으로 재현됐다.

원인은 게이트의 질문이 하나 모자란다는 것이다. `make check-generate`는 `make generate`를 실행한
전후 해시를 비교하므로 (`Makefile` `check-generate` 타깃) **"생성물이 소스와 일치하는가"** 는
답하지만 **"소스가 사실인가"** 는 결코 묻지 않는다. `make doc-check` 역시 링크·크기·바인딩
형식만 보고 산문의 사실성은 보지 않으며, `flowcheck`는 `docs/51`에서 프롬프트 산문을 명시적
non-goal로 선언했다. 따라서 AUTOGEN 경계 밖에서 손으로 쓴 사실 주장은 어떤 기계 게이트에도
걸리지 않는다.

이 계획의 카드들은 그 사각지대에 쌓인 부채를 청산한다. 사각지대 자체를 닫는 일은 아래 open
question 2가 소유한다.

| Workstream | 판정 | 작업 |
| --- | --- | --- |
| CLI 조언 문자열이 실행 불가능한 명령을 제안 | 재현 확인, 즉시 수정 가능 | [TASK-273](../todo/273-repair-misleading-cli-guidance.md) |
| flow 프롬프트가 유효한 config를 거부하거나 무효한 config를 생성 | 스키마 대조 완료 | [TASK-274](../todo/274-repair-flow-prompt-config-claims.md) |
| skill reference가 존재하지 않는 동작을 서술 | 소스 대조 완료 | [TASK-275](../todo/275-correct-skill-reference-fictions.md) |
| example corpus 결함 + markdown-YAML 게이트 공백 | 15/16 파일 `--strict` 실패 측정 | [TASK-276](../todo/276-correct-example-corpus-and-close-md-yaml-gap.md) |

## 권장 순서 (2026-09-03)

카드 간 코드 의존은 없지만 아래 순서가 재작업을 최소화한다.

1. **[TASK-267](../todo/267-repair-subproject-exposure-defects.md) 먼저** — PLAN-003 소속이지만
   `internal/cli`의 같은 영역(오류·힌트 문자열)을 건드린다. TASK-273보다 먼저 끝내면 두 카드가
   같은 파일에서 충돌하지 않는다.
2. **TASK-273** — 가장 작고(effort S) 코드 전용. 나머지 세 장은 문서 전용이라 여기서 분리된다.
3. **TASK-274** — flow 프롬프트. `make generate` 전파 경로를 여기서 한 번 확인해두면 TASK-275의
   동일 경로 작업이 단순해진다.
4. **TASK-275** — skill reference. `reference-examples.md`가 단일 소스이고 생성 사본이 셋이므로
   TASK-274에서 확인한 generate 경로를 그대로 재사용한다.
5. **TASK-276** — 마지막. 게이트를 추가하는 카드이므로 앞의 세 장이 남긴 결함이 없는 상태에서
   켜야 새 게이트가 기존 부채로 즉시 빨간불이 되지 않는다.

TASK-276은 [TASK-266](../todo/266-deprecate-and-reject-interaction-env-file.md)과 `examples/`
파일을 공유하므로 `depends-on: [TASK-266]`을 선언한다. TASK-266이 소유한
`examples/env-file-priority.yml`의 interaction `env_file` 선언과 `examples/README.md`의
"Command-specific env_file" 절은 TASK-276 범위에서 명시적으로 제외했다.

## Open questions

1. **bare `dva down <plan> -v`가 확인 프롬프트 없이 볼륨을 삭제한다.** `confirmDestruction`의
   유일한 호출부가 `--purge` 플래그로만 게이트되어 있다. TASK-276은 README의 잘못된 안심 문구를
   제거하지만 위험 자체는 남는다 — `-v` 단독도 프롬프트를 거쳐야 하는지는 제품 결정이며
   문서 카드가 대신 내릴 수 없다. 사람 판단 후 별도 카드로 발행한다.
2. **사실성 게이트를 영구화할 것인가.** 273~276은 이미 쌓인 부채를 청산할 뿐 같은 드리프트가
   다시 쌓이는 것을 막지 못한다. 청산 후 손으로 쓴 사실 블록이 둘 이상 남는다면, 그 블록들을
   AUTOGEN 경계 안으로 옮기거나 대조 게이트를 추가하는 카드(가칭 TASK-277)를 발행한다.
   **273~276 완료 전에는 발행하지 않는다** — 남는 블록의 수와 위치가 그때 결정되기 때문이다.
