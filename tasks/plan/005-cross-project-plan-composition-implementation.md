---
id: PLAN-005
title: "Implement cross-project plan composition"
type: plan
scope: "composition plan schema and validation, cross-project resolver identity, wave-sequential runtime with LIFO rollback, CLI flag-scope enforcement and aggregate output, and compatibility fixtures"
progress: 40
total-tasks: 5
completed-tasks: 0
children: [TASK-289, TASK-290, TASK-291, TASK-292, TASK-293]
target-date: "2027-01-31"
created: 2026-09-04
---

## Goal

[TASK-260](../todo/260-freeze-cross-project-plan-composition.md)가 완료기준 1-5로 얼린 단방향
root-aggregation 계약(§2-§6, `decided-at: 2026-09-04`)을 정확히 그 경계 안에서 구현한다. 이 계획은
PLAN-003의 자식이 아니다 — PLAN-003 §4 실행 규칙이 "vNext foundational schema/runtime
implementation은 이 계획에 넣지 않는다... 별도 plan과 bounded child cards를 만든다"고 이미
명시했고, TASK-260 완료기준 6이 요구하는 "별도의 bounded 구현 계획"이 바로 이 PLAN-005다.

TASK-260이 얼린 것 이상을 결정하지 않는다 — 이 계획의 어떤 카드도 identity, merge, flag-scope,
rollback, exit-code 계약을 새로 만들거나 바꾸지 않고 TASK-260의 §3-§6을 문자 그대로 구현한다.
계약과 구현 사이에 새로 발견되는 간극(현재 코드와의 불일치, 명명 충돌 등)은 각 카드의
Non-goals나 아래 "## Open questions" 절에 기록하고 조용히 다르게 구현하지 않는다.

| Workstream | TASK-260 근거 | 작업 |
| --- | --- | --- |
| Composition plan schema | §3 (identity·cycle·duplicate·merge·override·depends_on·order·immutability) | [TASK-289](../done/289-implement-composition-plan-schema.md) — 완료 (`1420ed4`) |
| Cross-project resolver | §3.2·3.5·3.8·3.9 (identity 참조·default 선택·wave 계산·불변성) | [TASK-290](../done/290-implement-composition-plan-resolver.md) — 완료 (`0e14d4b`) |
| Wave 실행과 rollback runtime | §4·§5 (verb별 동작·flag 전파·LIFO rollback·`--no-rollback`·partial-state·exit code) | [TASK-291](../todo/291-implement-composition-runtime-and-rollback.md) |
| CLI flag-scope와 aggregate 출력 | §4.4(flag 표)·§4.3(logs/status/build)·§5.3(JSON) | [TASK-292](../todo/292-implement-composition-cli-and-flag-scope.md) |
| Fixture와 호환성 회귀 | §3 fixture·§5 4개 시나리오·§6(호환성) | [TASK-293](../todo/293-verify-composition-fixtures-and-compatibility.md) |

## 작업 순서와 의존성

```text
TASK-289 schema ──> TASK-290 resolver ──> TASK-291 runtime/rollback ──┐
                                       └─> TASK-292 CLI flag-scope ───┼──> TASK-293 fixtures & compat
                                                                       ┘
```

TASK-289는 의존성 없이 착수 가능하다(TASK-260이 이미 `decided`). TASK-290은 289가 정의한
`composes:`/`ComposeEntry`(정확한 Go 명명은 아래 "## Open questions" 참고) 스키마 위에서만
resolve 대상을 만들 수 있으므로 289에 의존한다. TASK-291(runtime)과 TASK-292(CLI)는 둘 다 290의
resolved 결과(wave가 매겨진 composed entry 목록)를 입력으로 삼지만 서로 독립적으로 병행 가능하다
— runtime은 오케스트레이터 내부 실행·rollback을, CLI는 명령 파싱·flag 거부·출력 포맷을 담당하며
서로의 내부 구현에 의존하지 않는다. TASK-293은 289-292가 모두 존재해야 의미 있는 end-to-end
fixture와 회귀 테스트이므로 넷 모두에 의존한다.

## Non-goals (계획 전체)

- TASK-260이 이미 거부한 것(child-stack flattening, child-`env_file`의 root merge, recursive
  composition, 자동 reachability)을 다시 논의하지 않는다.
- `dva plan`/`dva exec`/`dva tool` 같은 vNext namespace나 `/` 문법 통일을 도입하지 않는다 —
  PLAN-003 §2가 이미 이를 비채택으로 명시했고 TASK-261이 별도로 판단한다.
- Persisted partial-state 파일이나 `--retry` 플래그를 발명하지 않는다(TASK-260 §5.4가 이미
  "child 실행 상태 자체가 진실"이라는 원칙으로 이를 대체했다).

## Open questions

1. **`composes:`(YAML 키)와 `dva compose <entry>`(기존 raw Docker Compose passthrough 명령,
   `internal/cli/compose.go`) 사이의 용어 동음이의.** TASK-260 §3.1은 `composes:` 필드명을
   이미 얼렸지만, 그 결정 시점에는 이 저장소에 이미 완전히 다른 의미의 `compose` 최상위 명령이
   존재한다는 사실이 §3.1 텍스트에 명시적으로 대조되지 않았다(같은 코드베이스 검증 과정에서
   TASK-260 커밋 이후 이 계획을 작성하며 뒤늦게 발견함). 사용자가 `dva.yml`에 `composes:`를
   선언하는 것과 `dva compose ps` 같은 완전히 무관한 디버깅 escape hatch를 같은 세션에서 접하면
   혼동할 수 있다. **이 계획은 그 결정을 재론하지 않는다** — `composes:`는 TASK-289가 얼린 그대로
   구현하되, Go 심볼 레벨의 혼동만 피하기 위해 구조체 이름은 `ComposeEntry`가 아니라
   `CompositionEntry`로 짓는다(TASK-260 §3.1이 이미 "정확한 Go 필드명은 구현 카드가 코드
   컨벤션에 맞춰 정한다"고 위임했으므로 이는 결정 재론이 아니다). YAML 키 자체의 재검토가
   필요하다고 판단되면 별도로 TASK-260에 human 재확인을 요청해야 한다 — 이 계획의 어떤 카드도
   `composes:` 키를 스스로 바꾸지 않는다.

## Children

- TASK-289 — implement the composition plan schema
- TASK-290 — implement the composition plan resolver
- TASK-291 — implement composition runtime and LIFO rollback
- TASK-292 — implement composition CLI flag scope and aggregate output
- TASK-293 — verify composition fixtures and compatibility
