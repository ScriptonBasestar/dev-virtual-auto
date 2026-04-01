# Environment Stack Overrides

## 배경

현재 `EnvironmentProfile`은 `stack` 필드로 "어떤 stack entry를 포함할지"만 제어합니다.
하지만 실제 운영에서는 같은 plugin entry를 유지한 채 환경별로 일부 설정만 바꾸고 싶은 요구가 생깁니다.

예를 들면:

- `kubectl.namespace`
- `compose.files`
- `helm.values_files`
- `exports`

단순 scalar는 env interpolation으로 우회 가능하지만, 배열 필드나 plugin-specific nested config는 표현력이 부족합니다.

## 현재 상태

- 환경 구조: `internal/config/config.go`의 `EnvironmentProfile`
- env 적용: `internal/cli/compose.go`의 `applyEnv()`
- stack 필터링: `internal/lifecycle/orchestrator.go`의 `filterEntries()`
- stack 구조: `internal/config/lifecycle.go`

현재 환경은 stack entry 선택과 environment var merge만 수행합니다.
stack entry 자체의 필드를 환경별로 덮는 개념은 없습니다.

## 문제

- staging/production에서 같은 plugin을 쓰되 값만 다르게 두기 어렵습니다.
- 배열 필드 override가 env var로 해결되지 않습니다.
- 같은 목적의 stack entry를 환경별로 복제하게 되어 설정이 중복됩니다.
- 이 기능은 config deep merge semantics 없이는 일관되게 설계하기 어렵습니다.

## 고려 중인 방향

환경에 `stack_overrides`를 두고, base stack entry 위에 제한적인 partial override를 적용하는 모델입니다.

핵심 쟁점:

- override 가능 필드와 금지 필드 구분
- deep merge 규칙
- 적용 시점
- `mode`, `env`, `tags`와의 결합 순서
- env interpolation과의 우선순위

## 예시 형태

```yaml
environments:
  stg:
    stack: [kubectl]
    stack_overrides:
      kubectl:
        namespace: myapp-staging
  prd:
    stack: [kubectl, helm]
    stack_overrides:
      kubectl:
        namespace: myapp-production
      helm:
        values_files: [values-production.yaml]
```

## 선행 조건

- config deep merge semantics 정리
- execution plan에서 env 적용 순서 정리

## 범위

- `EnvironmentProfile` 확장 여부 판단
- override 대상/비대상 필드 구분
- schema 반영 필요성 검토
- 테스트 시나리오 도출

## 제외

- env interpolation 확장 자체
- plugin별 bespoke override 규칙 즉시 구현
- subproject override까지 한 번에 확장

## 참조 정보

- `internal/config/config.go`
- `internal/cli/compose.go`
- `internal/lifecycle/orchestrator.go`
- `internal/config/lifecycle.go`
- 관련 선행 태스크:
  - `tasks/backlog/config-deep-merge-semantics.md`
  - `tasks/backlog/execution-plan-resolution.md`

## 완료 조건

- `stack_overrides` 도입 여부가 결정되어 있다.
- 도입 시 허용 필드와 금지 필드가 명시되어 있다.
- 적용 우선순위가 base stack, env override, interpolation 기준으로 정리되어 있다.
- schema 및 테스트 영향 범위가 정리되어 있다.
- env interpolation만으로 충분한 경우와 그렇지 않은 경우가 구분되어 있다.

## 체크리스트

- [ ] 문제 사례가 scalar/array/plugin nested config로 나뉘어 정리되어 있다.
- [ ] override 대상 필드 분류가 있다.
- [ ] merge semantics 의존성이 명시되어 있다.
- [ ] backward compatibility 위험이 적혀 있다.
- [ ] 구현 전 필요한 테스트 시나리오가 정리되어 있다.
