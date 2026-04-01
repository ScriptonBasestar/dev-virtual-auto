# Validation Severity Policy

## 배경

현재 schema validation, semantic warnings, reserved command 충돌, compose project name 검사 등이 분산되어 있습니다.
같은 문제라도 hard error, warning, doctor result 중 어디에 속해야 하는지 일관성이 약합니다.

## 현재 상태

- schema validation: `internal/config/validate.go`
- semantic warnings: `internal/config/validate_warnings.go`
- CLI 진입점: `internal/cli/validate.go`
- 관련 테스트:
  - `internal/config/validate_warnings_test.go`
  - `internal/cli/validate_test.go`

## 문제

- 어떤 문제는 hard error가 맞고, 어떤 문제는 warning이 맞지만 경계가 문서화되어 있지 않습니다.
- 동일한 정보를 `validate`와 `doctor`가 중복해서 가질 위험이 있습니다.
- 새 검사를 추가할 때 어느 계층에 넣어야 하는지 판단 기준이 약합니다.

## 이번 작업의 목표

config 및 실행 전 진단 항목을 `hard error`, `warning`, `doctor-only`로 나누는 기준을 고정합니다.

## 범위

- validation severity 기준 정리
- 기존 검사 항목의 재배치 후보 정리
- `doctor`와 `validate`의 책임 경계 명시

## 제외

- 개별 check 구현 전체
- plugin별 bespoke validation
- config deep merge semantics 변경

## 참조 정보

- `internal/config/validate.go`
- `internal/config/validate_warnings.go`
- `internal/cli/validate.go`
- `internal/cli/doctor.go`
- `internal/config/validate_warnings_test.go`
- `internal/cli/validate_test.go`
- 연관 태스크: `tasks/todo/doctor-preflight-expansion.md`

## 완료 조건

- 진단 항목 분류 기준이 문서화되어 있다.
- 기존 주요 검사 항목이 `hard error`, `warning`, `doctor-only` 중 하나로 배치되어 있다.
- `doctor`와 `validate`의 역할 경계가 설명되어 있다.
- 중복 검사 제거 또는 재사용 방향이 정리되어 있다.
- 테스트와 문서 반영 대상이 정리되어 있다.

## 체크리스트

- [ ] severity 판단 기준이 있다.
- [ ] 기존 주요 검사 항목의 분류표가 있다.
- [ ] doctor와 validate 경계가 적혀 있다.
- [ ] compose project name mismatch 처리 위치가 정리되어 있다.
- [ ] 후속 구현 순서가 적혀 있다.
