# Interaction Inheritance And Merge Semantics

## 배경

`interaction`은 DVA의 사용자-facing DSL에 가깝습니다.
현재 parent command와 subcommand 병합은 동작하지만, 필드가 늘어날수록 상속 누락이나 예외 규칙이 생기기 쉬운 구조입니다.

## 현재 상태

- command 해석: `internal/runner/interaction_tree.go`
- runner 선택: `internal/runner/runner.go`
- 실행 엔트리: `internal/cli/run.go`
- validation warnings: `internal/config/validate_warnings.go`

현재 `mergeInteraction()`은 parent를 기반으로 child 값을 덮는 방식입니다.
하지만 어떤 필드가 상속되고 어떤 필드가 replace되는지 규칙이 코드에 암묵적으로 들어 있습니다.

## 문제

- 새 필드가 추가되면 merge 로직 누락 위험이 있습니다.
- `Compose`, `Environment`, `Runner`, `Shell`, `DefaultArgs`의 merge semantics가 문서화되어 있지 않습니다.
- recursive subcommand 구조에 대한 warning/validation도 아직 얕습니다.
- 사용자는 "부모에서 무엇을 상속받는지" 예측하기 어렵습니다.

## 이번 작업의 목표

`interaction` 상속 규칙을 현재 구현 범위에서 명시적으로 정리하고, 이후 필드 추가 시 누락이 생기지 않도록 테스트와 규칙을 고정합니다.

## 범위

- parent/subcommand field inheritance 규칙 정의
- map/array/scalar별 merge 정책 정의
- nested subcommand에 대한 validation/warning 범위 검토
- `runner` 선택 로직과 interaction semantics 사이의 연결 정리

## 제외

- 새 runner 추가
- dynamic routing 자체 재설계
- command 실행 backend 변경

## 참조 정보

- `internal/runner/interaction_tree.go`
- `internal/runner/runner.go`
- `internal/cli/run.go`
- `internal/config/config.go`
- `internal/config/validate_warnings.go`
- `internal/cli/run_test.go`
- `internal/runner/runner_test.go`

## 완료 조건

- 각 interaction 필드의 상속/override 규칙이 표 또는 목록으로 정리되어 있다.
- recursive subcommand까지 포함한 validation/warning 대상이 정리되어 있다.
- runner 결정 시점과 interaction merge 시점의 책임 경계가 설명되어 있다.
- 테스트해야 할 대표 케이스가 정리되어 있다.

## 체크리스트

- [ ] scalar/map/list 필드별 merge 정책이 있다.
- [ ] `default_args`와 CLI 인자 결합 규칙이 적혀 있다.
- [ ] `compose` 옵션 상속 규칙이 적혀 있다.
- [ ] nested subcommand validation 필요 여부가 결정되어 있다.
- [ ] 문서 반영 대상과 테스트 대상이 정리되어 있다.
