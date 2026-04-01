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

## 이번 작업의 목표

현재 구현 범위 안에서 interpolation semantics를 명확히 하고, 작은 범위 확장 또는 실패 노출 방식을 도입할 준비를 합니다.
이 태스크는 "전체 셸 호환"이 아니라 DVA config에서 실제 필요한 규칙을 닫는 것이 목적입니다.

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
- 연관 태스크: `tasks/backlog/config-deep-merge-semantics.md`

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
