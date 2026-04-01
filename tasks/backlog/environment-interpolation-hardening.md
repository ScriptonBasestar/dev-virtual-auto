# Environment Interpolation Hardening

## 배경

현재 env interpolation은 `$VAR`, `${VAR}` 수준만 지원합니다.
DVA가 설정 오케스트레이터 역할을 하려면, interpolation 실패나 default 처리 규칙이 더 명확해야 합니다.

## 현재 상태

- 환경 변수 모델: `internal/config/environment.go`
- env file 로딩: `internal/config/envfile.go`
- 관련 테스트:
  - `internal/config/environment_test.go`
  - `internal/config/envfile_test.go`

현재 구현은 미해결 변수를 원문 그대로 남기며, unresolved 상태를 별도 경고하지 않습니다.
쉘 스타일 기본값 문법이나 required 문법도 없습니다.

## 문제

- 오타가 있어도 조용히 지나갈 수 있습니다.
- `${VAR:-default}` 같은 실용적인 패턴이 없습니다.
- 향후 environment 기반 stack/plugin 설정 확장 시 표현력이 부족합니다.
- interpolation 규칙이 문서와 validation에서 충분히 드러나지 않습니다.

## 검토할 항목

- `${VAR:-default}` 지원 여부
- `${VAR?message}` 또는 이에 준하는 required syntax 지원 여부
- unresolved variable을 warning 또는 validation 대상으로 볼지
- recursive interpolation과 cycle 방지 정책
- OS env, config env, env_file 값의 우선순위와 문서화

## 범위

- interpolation semantics 설계
- validation/warning 연계 필요성 검토
- 문서와 테스트 시나리오 정리

## 제외

- shell 전체 문법 호환
- secret manager 연동
- stack_overrides 자체 구현

## 참조 정보

- `internal/config/environment.go`
- `internal/config/envfile.go`
- `internal/config/environment_test.go`
- `internal/config/envfile_test.go`
- `internal/config/schema.json`

## 완료 조건

- 지원할 interpolation 문법이 명시되어 있다.
- 지원하지 않을 문법도 명시되어 있다.
- 우선순위와 실패 동작이 정리되어 있다.
- warning 또는 validation으로 노출할 조건이 정의되어 있다.
- 테스트 시나리오가 최소 정상/기본값/오류/순환 참조 케이스로 구분되어 있다.

## 체크리스트

- [ ] OS env vs config env vs env_file 우선순위가 명시되어 있다.
- [ ] unresolved variable 처리 방침이 있다.
- [ ] default syntax 지원 여부가 결정되어 있다.
- [ ] required syntax 지원 여부가 결정되어 있다.
- [ ] 문서 반영 대상이 정리되어 있다.
