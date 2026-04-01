# Doctor And Validation Convergence

## 배경

현재 `dva doctor`, schema validation, semantic warnings, compose project name 검사 기능이 분산되어 있습니다.
사용자가 체감하는 실패는 대부분 실행 직전에 드러나므로, 진단 계층을 더 일관되게 묶을 필요가 있습니다.

## 현재 상태

- doctor: `internal/cli/doctor.go`
- schema validation: `internal/config/validate.go`
- semantic warnings: `internal/config/validate_warnings.go`
- compose project name check: `internal/config/validate.go`

현재 `doctor`는 Docker 접근성, compose 파일 존재, devcontainer, `.gitignore` 중심입니다.
반면 config semantic 문제나 실행 계획 충돌은 별도 경로에 흩어져 있습니다.

## 문제

- 사용자가 어떤 명령을 먼저 실행해야 진단이 충분한지 애매합니다.
- config 문제와 환경 문제를 한 번에 보기 어렵습니다.
- `doctor`가 실행 전 문제를 충분히 포착하지 못합니다.
- validation/warning 결과를 어디서 어떻게 보여줄지 일관성이 부족합니다.

## 목표

`doctor`와 validation 계층의 역할을 정리하고, 어떤 진단을 어디에서 수행할지 명확히 합니다.

예시 후보:

- stack entry 존재 여부
- mode/env 참조 무결성
- required external binary 존재 여부
- compose project_name mismatch
- health check 설정 이상
- interaction reserved command 충돌

## 범위

- doctor와 validate의 책임 분리 또는 통합 방향 정리
- 어떤 체크를 hard error / warning / doctor result로 둘지 분류
- 사용자 출력 포맷과 명령 흐름 개선 포인트 정리

## 제외

- 실제 doctor UI 전면 개편
- plugin별 상세 health 구현
- config deep merge 구현

## 참조 정보

- `internal/cli/doctor.go`
- `internal/config/validate.go`
- `internal/config/validate_warnings.go`
- `internal/cli/validate.go`
- `internal/cli/doctor_test.go`
- `internal/config/validate_warnings_test.go`

## 완료 조건

- 진단 항목이 `hard error`, `warning`, `doctor-only`로 분류되어 있다.
- `doctor`가 담당해야 할 실행 전 체크 목록이 정리되어 있다.
- `validate`가 담당해야 할 config 무결성 범위가 정리되어 있다.
- 중복 체크를 어디서 제거하거나 재사용할지 제안이 있다.
- 테스트와 문서 반영 포인트가 정리되어 있다.

## 체크리스트

- [ ] doctor와 validate의 역할 경계가 적혀 있다.
- [ ] warning으로 둘 항목과 hard error로 올릴 항목이 구분되어 있다.
- [ ] compose name mismatch 처리 위치가 명시되어 있다.
- [ ] mode/env/stack 관련 사전 진단 후보가 정리되어 있다.
- [ ] 테스트 보강 대상이 적혀 있다.
