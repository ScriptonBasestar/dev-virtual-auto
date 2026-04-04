# Task 002: Schema V2

## Goal

새 설정 모델을 `schema.json`에 반영한다.

## Scope

- `internal/config/schema.json`
- schema 관련 테스트

## Out Of Scope

- Go struct 구현
- merge / resolver / CLI 동작 구현

## Deliverables

- `vars`, `plans`, `sites`, multi-runner `stack` 스키마
- `plans.entries` 필드 정의
- `sites.entry_overrides` 정의
- `env_file` 유지 반영

## Acceptance Criteria

- 대표 예제들이 schema 관점에서 표현 가능하다
- `stack.default_runner`와 `stack.runners`가 검증된다
- 정의되지 않은 runner 선택은 schema 또는 후속 validation 대상으로 분리된다
- 구형 `modes`, `applications`는 제거 또는 deprecated 처리 방향이 명시된다

## Dependencies

- `001-config-model`

## References

- [30-config-merge-semantics.md](/Users/archmagece/myopen/scripton/dev-virtual-auto/docs/30-config-merge-semantics.md)
- [40-declarative-stack-and-plans.md](/Users/archmagece/myopen/scripton/dev-virtual-auto/docs/40-declarative-stack-and-plans.md)

