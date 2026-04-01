# Doctor Preflight Expansion

## 배경

현재 `dva doctor`는 Docker 접근성, compose 파일 존재, devcontainer, `.gitignore` 중심입니다.
하지만 실제 사용자 실패는 실행 직전의 preflight 체크 부족에서 자주 발생합니다.

## 현재 상태

- doctor 구현: `internal/cli/doctor.go`
- 관련 테스트: `internal/cli/doctor_test.go`
- 참고 가능한 config 검사:
  - `internal/config/validate.go`
  - `internal/config/validate_warnings.go`

## 문제

- `doctor`가 stack/mode/env 관련 사전 문제를 충분히 포착하지 못합니다.
- 외부 바이너리 존재 여부나 health check 설정 이상 같은 실행 전 문제를 한 번에 보기 어렵습니다.
- 사용자가 `doctor` 결과만으로 "바로 실행 가능한 상태인지" 판단하기 어렵습니다.

## 이번 작업의 목표

`doctor`를 실행 전 preflight 진단 도구로 확장할 수 있도록, 어떤 체크를 추가할지와 어떤 체크가 `doctor`에 적합한지를 정리합니다.

## 범위

- doctor에 넣을 preflight 체크 후보 정리
- 현재 built-in check와 추가 후보의 우선순위 정리
- 테스트 관점에서 검증 가능한 후보 추리기

## 제외

- validation severity 정책 전면 재설계
- plugin별 상세 health 구현
- UI 전면 개편

## 참조 정보

- `internal/cli/doctor.go`
- `internal/cli/doctor_test.go`
- `internal/config/validate.go`
- `internal/config/validate_warnings.go`
- 연관 태스크: `tasks/todo/validation-severity-policy.md`

## 완료 조건

- `doctor`에 추가할 preflight 체크 후보가 우선순위와 함께 정리되어 있다.
- 각 후보가 doctor에 적합한지, warning/validate 쪽이 적합한지 구분되어 있다.
- 빠르게 구현 가능한 항목과 추가 설계가 필요한 항목이 구분되어 있다.
- 테스트 보강 대상이 정리되어 있다.

## 체크리스트

- [ ] mode/env/stack 관련 사전 진단 후보가 있다.
- [ ] external binary / compose name / health check 관련 후보가 있다.
- [ ] doctor에 둘 항목과 다른 계층으로 보낼 항목이 구분되어 있다.
- [ ] 테스트 가능한 범위가 적혀 있다.
- [ ] 후속 구현 순서가 적혀 있다.
