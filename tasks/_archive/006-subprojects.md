---
archived-at: 2026-04-05T23:02:00+09:00
verified-at: 2026-04-05T23:02:00+09:00
verification-summary: Verified implementation via git commits 2e25daf through 5524b0e.
---
# Task 006: Subprojects

## Goal

subproject import를 새 구조에 맞게 정리한다.

## Scope

- `internal/config/subproject.go`
- CLI / resolver integration
- 관련 테스트

## Out Of Scope

- general merge/schema refactor

## Deliverables

- `plans`, `interactions`, `provision` import
- canonical namespace 규칙
- alias 충돌 검증
- subproject root 기준 interaction/provision 실행

## Acceptance Criteria

- `backend/local-dev` 형식으로 plan 실행 가능
- `backend/shell`, `backend/setup` 같은 canonical name이 일관되게 처리된다
- alias 충돌은 hard error
- subproject interaction/provision은 subproject root 기준으로 실행된다

## Dependencies

- `001-config-model`
- `004-resolver`
- `005-cli-lifecycle`

## References

- [40-declarative-stack-and-plans.md](../../docs/40-declarative-stack-and-plans.md)

