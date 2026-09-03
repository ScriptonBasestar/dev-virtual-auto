---
id: PLAN-004
title: "Restore documentation truth across skills, flows, and the example corpus"
type: plan
scope: "CLI advice strings, agent-mesh flow prompt claims, skill reference fictions, example corpus defects, and the markdown-YAML validation gap"
progress: 30
total-tasks: 5
completed-tasks: 1
children: [TASK-273, TASK-274, TASK-275, TASK-276, TASK-283]
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
| CLI 조언 문자열이 실행 불가능한 명령을 제안 | **완료 (`206918a` → `20d0f67`)** — 잔존 다섯 입력을 TASK-283이 닫음 | [TASK-273](../done/273-repair-misleading-cli-guidance.md) → [TASK-283](../done/283-repair-plan-route-flag-guidance.md) |
| flow 프롬프트가 유효한 config를 거부하거나 무효한 config를 생성 | **완료 (`159bf1b`)** | [TASK-274](../done/274-repair-flow-prompt-config-claims.md) |
| skill reference가 존재하지 않는 동작을 서술 | 소스 대조 완료 | [TASK-275](../todo/275-correct-skill-reference-fictions.md) |
| example corpus 결함 + markdown-YAML 게이트 공백 | 15/16 파일 `--strict` 실패 측정 | [TASK-276](../todo/276-correct-example-corpus-and-close-md-yaml-gap.md) |

## 권장 순서 (2026-09-03)

카드 간 코드 의존은 없지만 아래 순서가 재작업을 최소화한다.

1. **[TASK-267](../done/267-repair-subproject-exposure-defects.md) — 완료 (`517dd11`).**
   PLAN-003 소속이지만 `internal/cli`의 같은 영역(오류·힌트 문자열)을 건드리므로 TASK-273보다
   먼저 끝내도록 배치했고, 그대로 처리됐다. 두 카드가 같은 파일에서 충돌할 위험은 해소됐다.
2. **TASK-273 — 완료 (`206918a`).** 코드 전용이라 나머지 세 장(문서 전용)과 여기서 분리됐다.
   처음에는 effort S로 적었으나, 측정 결과 `--tag`/`--exclude-tag`/`--mode`/`--env`가 죽은
   옵션이 아니라 stack 경로에서만 동작하는 경로 조건부 옵션임이 드러나 범위가 manifest·정적
   테스트·help 산문까지 넓어졌고 **effort M**으로 상향됐다.
   이 카드는 조언 문자열만 고쳤고 그 아래 **동작** 결함 — `restart`가 `--force`를 버리고,
   `stop`/`down`이 `--no-wait`를 흡수하며, `build`가 `--env`/`--tag`/`--exclude-tag`를
   파싱 지점에서 버리는 것 — 은 의도적으로 남겨
   [TASK-279](../todo/279-repair-plan-flag-behaviour-defects.md)로 분리했다. TASK-279는
   PLAN-004 소속이 아니며(문서가 아니라 런타임을 바꾼다) 이 계획의 자식 수에 포함되지 않는다.

   **`206918a` 이후 재개된 항목.** 이 카드를 쓰지 않은 리뷰어의 독립 검토에서 조언 문자열
   결함이 닫히지 않았음이 드러났고, 구현자가 `916b07e` 기준 새 빌드로 다섯 입력을 모두
   재현했다. 그중 하나는 개선이 아니라 **후퇴**다 — `dva up --tag app --dry-run`의 제안에서
   `--dry-run`이 사라져, 미리보기를 요청한 사용자가 실제로 실행되는 명령을 안내받는다.
   수정 전에는 같은 입력이 rc=1로 실패하며 아무것도 실행하지 않았다. 후속은
   [TASK-283](../done/283-repair-plan-route-flag-guidance.md)이며, TASK-279와 달리 조언
   문자열을 고치는 카드이므로 PLAN-004의 자식으로 둔다.

   **닫힘 (`20d0f67`).** TASK-283이 다섯 입력을 모두 처리했고 후퇴했던 `--dry-run` 보존도
   회복했다. 이 카드가 세운 성질은 문자열을 하나씩 고친 것이 아니라 **검증되지 않은 제안을
   출력하지 않는다**는 것이다 — 가드가 규칙을 자체적으로 재유도하는 대신 실제 파서
   (`parsePlanFlags`)에게 되물어, 제안이 실행 가능함을 확인한 뒤에만 출력한다. 손으로 유도한
   규칙은 두 번의 측정에서 두 번 다 틀렸고, 그것이 위임의 근거다. 이 설계는 리베이스에서
   검증됐다: TASK-279가 파서를 verb 인식으로 바꾸자 `dva stop --tag --no-wait`에 대한 조언이
   가드 규칙을 한 줄도 고치지 않은 채 올바르게 따라 움직였다. 이제 TASK-273의 워크스트림을
   완료로 읽어도 된다.
3. **TASK-274 — 완료 (`159bf1b`).** flow 프롬프트. `make generate` 전파 경로를 여기서 한 번
   확인해두면 TASK-275의 동일 경로 작업이 단순해진다고 적었고, 그대로 됐다 — TASK-275를 맡는
   세션은 이 카드의 diff에서 전파 경로를 먼저 읽는 것이 가장 빠르다.
4. **TASK-275** — skill reference. `reference-examples.md`가 단일 소스이고 생성 사본이 셋이므로
   TASK-274에서 확인한 generate 경로를 그대로 재사용한다.
5. **TASK-276** — 마지막. 게이트를 추가하는 카드이므로 앞의 세 장이 남긴 결함이 없는 상태에서
   켜야 새 게이트가 기존 부채로 즉시 빨간불이 되지 않는다. 이 카드가 켜는 `validate --strict`
   게이트는 [TASK-277](../done/277-repair-nondeterministic-env-interpolation.md)의 `MergeVars`
   비결정성 위에서는 간헐 실패했을 것이므로 선행 조건이었으나, 해당 카드가 `e9ce4e6`으로
   완료되어 더 이상 막지 않는다.

TASK-276은 [TASK-266](../todo/266-deprecate-and-reject-interaction-env-file.md)과 `examples/`
파일을 공유하므로 처음에 `depends-on: [TASK-266]`을 선언했다. **2026-09-03에 해제했다.**
TASK-266은 Stage B가 0.1.48 릴리스 게이트에 걸려 열려 있지만, 공유하던 파일은 이미 끝난
Stage A(`c6aa64b`)가 정리했다 — `examples/env-file-priority.yml`에는 root `env_file:`만 남았고
`examples/README.md`의 "Command-specific env_file" 절은 사라졌다. Stage B는 스키마와 Go 타입만
건드린다. 선언을 남겨두면 릴리스가 나갈 때까지 TASK-276 전체가 차단된 것처럼 읽히므로,
실질적으로 충족된 의존을 해제하고 그 두 파일은 TASK-276의 Non-goals에서 계속 범위 밖으로
못박아 둔다.

## Open questions

1. **사실성 게이트를 영구화할 것인가.** 273~276은 이미 쌓인 부채를 청산할 뿐 같은 드리프트가
   다시 쌓이는 것을 막지 못한다. 청산 후 손으로 쓴 사실 블록이 둘 이상 남는다면, 그 블록들을
   AUTOGEN 경계 안으로 옮기거나 대조 게이트를 추가하는 후속 카드를 발행한다.
   **273~276 완료 전에는 발행하지 않는다** — 남는 블록의 수와 위치가 그때 결정된다.
   ID는 발행 시점에 미사용 번호를 취한다: 이 계획을 열 때 잠정적으로 적어둔 277번은 예약된
   번호가 아니며, 무관한 결함을 다루는
   [TASK-277](../done/277-repair-nondeterministic-env-interpolation.md)이 이미 그 번호를
   사용했다.

   **사전 조사 완료 (2026-09-03).** 발행 조건은 그대로지만 — 275가 닫히기 전에는 발행하지
   않는다 — 그때 필요한 측정은 미리 끝내 두었으므로, 다음 세션은 조사부터 다시 시작할 필요가
   없다.

   *(1) 손으로 쓴 사실 블록은 세 종류뿐이고, 셋 다 이미 Go에서 생성되고 있다.*
   `tools/libgen`은 `agent-mesh-flows/shared/library/shared-guardrails.md` 한 파일에만
   주입하며 마커는 셋이다 — `version_rule`, `reserved_commands`(예약 24 + hookable 6을 한
   블록에서 함께 출력), `section_order`(정규 22개 순서). 즉 **메커니즘도 소스도 이미
   존재하고, 적용 범위만 한 파일이다.**

   *(2) `skills/` 트리에는 AUTOGEN 커버리지가 전혀 없다.* 같은 세 사실을 손으로 다시 적은
   사본의 현재 위치와 상태:

   | 파일 | 사본 | 상태 |
   | --- | --- | --- |
   | `skills/dva-config/references/schema-reference.md` | 예약 24, hookable 6, 정규 순서 22 | **정확함** — docs/43 이후 손으로 맞춰졌다. 게이트가 없을 뿐 값은 맞다 |
   | `skills/dva/references/commands.md` | 예약 목록, hookable 개수 | 낡음 — TASK-275 item 2가 소유 |
   | `skills/dva/references/patterns.md` | 정규 순서 | 낡음 (`default_plan` 누락 등) — TASK-275 item 4가 소유 |
   | `skills/dva/references/advanced.md` | hookable 개수 | 낡음 — TASK-275 item 3a가 소유 |

   `agent-mesh-flows/` 안의 사본들은 전부 flowgen이 `shared-guardrails.md`에서 복사한
   생성물이므로 손으로 쓴 블록이 아니다 (`tools/flowgen/main.go`의 target 표).

   *(3) 따라서 후속 카드의 범위는 "게이트를 새로 설계한다"가 아니라 "이미 있는 세 마커를
   `skills/` 파일들에 설치하고 libgen의 대상 목록을 한 파일에서 표로 넓힌다"이다.* libgen은
   현재 `guardrailsPath` 단일 경로를 하드코딩하고 있어 flowgen과 같은 target 표로 바꾸는 것이
   실제 작업의 대부분이다. TASK-275가 값을 먼저 고쳐야 하는 이유도 여기 있다 — 마커를 먼저
   설치하면 낡은 값이 생성물로 덮여 사라지고, 무엇이 왜 틀렸는지 기록이 남지 않는다.

   *(4) 선례:* [TASK-280](../done/280-name-the-live-hookable-set-in-the-schema.md)은 같은
   hookable 목록의 네 번째 사본을 `internal/config/schema.json`에서 찾아 고쳤다. JSON은 Go
   헬퍼를 호출할 수 없어 AUTOGEN 대신 **파생 테스트**를 썼다 —
   `TestSchemaDescriptionNamesTheLiveHookableCommands`가 기대값을 `HookableCommandList()`에서
   가져온다. 마커를 넣을 수 없는 표면에는 이 패턴을 재사용한다. TASK-280은 이 계획을 연 뒤에
   발견된 건이라 children에 넣지 않았지만 근본 원인은 같다.

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
