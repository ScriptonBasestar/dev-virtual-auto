# Execution Plan Resolution For Mode And Environment

## 배경

현재 `mode`, `env`, `tags`, `applications`, `health_checks`, `compose_profiles`, `compose_services` 해석이 여러 위치에 흩어져 있습니다.
사용자 입장에서는 이것들이 하나의 "실행 계획"처럼 작동해야 하지만, 구현은 필터와 후처리의 조합입니다.

## 현재 상태

- stack entry 필터링: `internal/lifecycle/orchestrator.go`의 `filterEntries()`
- env 적용: `internal/cli/compose.go`의 `applyEnv()`
- default mode 적용: `internal/cli/compose.go`의 `applyDefaultMode()`
- mode lookup: `internal/cli/compose.go`의 `resolveMode()`
- native process 시작/중지: `internal/lifecycle/orchestrator.go`의 `startModeProcesses()`, `stopModeProcesses()`
- application 시작: `internal/lifecycle/orchestrator.go`의 `Up()`

## 문제

- `mode`와 `env`가 어떤 순서로 좁혀지는지 구현을 읽지 않으면 알기 어렵습니다.
- compose 관련 설정과 application 관련 설정이 같은 abstraction 위에 있지 않습니다.
- 실행 계획을 사용자에게 설명하거나 검증하기가 어렵습니다.
- 이후 `stack_overrides` 같은 기능이 추가되면 해석 로직이 더 분산될 가능성이 큽니다.

## 목표

실행 전 단계에서 "이번 실행에서 무엇이 적용되는가"를 단일 구조로 해석하는 모델을 정의합니다.

예시 질문:

- 어떤 stack entries가 선택되는가
- 어떤 environment vars가 최종 적용되는가
- 어떤 compose profiles/services가 활성화되는가
- 어떤 native health check process가 시작 대상인가
- 어떤 applications가 어떤 strategy로 시작되는가

## 범위

- `mode`, `env`, `tags`, default_mode의 해석 순서 정의
- 실행 계획 abstraction 필요 여부 판단
- CLI와 orchestrator 사이 책임 경계 정리

## 제외

- 실제 plugin 실행 로직 변경
- 새 CLI 명령 추가 구현
- config deep merge 구현

## 참조 정보

- `internal/lifecycle/orchestrator.go`
- `internal/cli/compose.go`
- `internal/lifecycle/app_manager.go`
- `internal/config/config.go`
- `internal/lifecycle/orchestrator_test.go`

## 완료 조건

- 실행 계획 해석 순서가 문서화되어 있다.
- CLI 계층과 orchestrator 계층의 책임이 분리되어 있다.
- `mode`, `env`, `tags`, `default_mode` 충돌 시 기대 동작이 명시되어 있다.
- 향후 노출 가능한 `explain` 또는 `plan` 출력의 기반 구조가 정의되어 있다.
- 테스트 대상 시나리오가 정리되어 있다.

## 체크리스트

- [ ] env 필터와 mode 필터의 결합 규칙이 적혀 있다.
- [ ] environment var merge 우선순위가 적혀 있다.
- [ ] applications / health_checks / compose hints가 같은 실행 계획 문맥에서 설명되어 있다.
- [ ] 오류 메시지 또는 validation 강화가 필요한 지점이 정리되어 있다.
- [ ] 테스트 케이스 초안이 있다.
