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

1. **사실성 게이트를 영구화할 것인가.** 273~276은 이미 쌓인 부채를 청산할 뿐 같은 드리프트가
   다시 쌓이는 것을 막지 못한다. 청산 후 손으로 쓴 사실 블록이 둘 이상 남는다면, 그 블록들을
   AUTOGEN 경계 안으로 옮기거나 대조 게이트를 추가하는 후속 카드를 발행한다.
   **273~276 완료 전에는 발행하지 않는다** — 남는 블록의 수와 위치가 그때 결정된다.
   ID는 발행 시점에 미사용 번호를 취한다: 이 계획을 열 때 잠정적으로 적어둔 277번은 다른
   세션이 무관한 결함으로 먼저 가져갔으므로 예약된 번호가 아니다.

## 검토 종료된 항목

**`down -v`에 확인 프롬프트를 붙일 것인가 — 아니오 (2026-09-03, 코드 대조로 종결).**

이 계획을 처음 열 때 "bare `-v`가 프롬프트 없이 볼륨을 지우는 것이 위험이며 제품 결정이
필요하다"고 open question으로 올렸으나, 오독이었다. 두 플래그의 범위가 다르고 프롬프트의
비대칭은 그 차이를 정확히 따른다 (`internal/cli/plan_lifecycle.go`, `Volumes:`/`RemoveImages:`
필드와 그 위 `--purge is \`clean\` folded into the plan path` 주석):

| 플래그 | named volume | 로컬 빌드 이미지 | provision 마커 | 프롬프트 |
| --- | --- | --- | --- | --- |
| `-v` | 삭제 | 유지 | 유지 | 없음 |
| `--purge` | 삭제 | 삭제 | 삭제 (config 디렉토리 전체) | 있음 |

`-v`는 이름이 말하는 것만 지우고 `docker compose down -v`와 의미가 같다 — 플래그를 타이핑한
행위 자체가 명시적 동의이므로 프롬프트는 중복이고, CI·스크립트 경로를 깨뜨린다.
`--purge`가 프롬프트를 거치는 이유는 범위가 이름을 넘어서기 때문이다. 특히 provision 마커
삭제는 주석이 스스로 인정하듯 plan 단위가 아니라 config 디렉토리 전체에 미쳐, 사용자가 이름
댄 plan 밖까지 건드린다.

따라서 `confirmDestruction`의 게이트 조건은 정확하며 코드 변경 대상이 아니다. 실제 결함은
README가 `-v`도 프롬프트를 거친다고 서술한 문장 하나뿐이고, [TASK-276](../todo/276-correct-example-corpus-and-close-md-yaml-gap.md)이
그것을 소유한다. **`-v` 경로에 프롬프트를 추가하는 후속 카드는 발행하지 않는다.**
