---
id: TASK-004
status: done
archived-at: 2026-04-05T23:02:00+09:00
verified-at: 2026-04-05T23:02:00+09:00
verification-summary: Verified implementation via git commits 2e25daf through 5524b0e.
---
# Task 004: Execution Resolver

## Goal

`plans` 기반 실행 해석 로직을 구현한다.

## Scope

- `internal/lifecycle/` resolver or orchestration layer
- 필요 시 `internal/config/` 보조 함수
- 관련 테스트

## Out Of Scope

- interaction runner 구현 변경
- full CLI migration

## Deliverables

- named execution entry lookup
- `env_file` + `vars` 우선순위 병합
- `stack` 참조 해석
- runner 결정 우선순위 구현
- `depends_on` + `order` 기반 wave 계산
- resolved immutable execution plan 모델

## Acceptance Criteria

- `dva up <name>`에 필요한 해석 정보가 하나의 resolved plan으로 생성된다
- runner 우선순위가 문서와 일치한다
- compose `services` subset이 반영된다
- undefined runner / missing stack / cyclic dependency가 명확히 실패한다

## Dependencies

- `001-config-model`
- `003-merge-semantics`

## References

- [31-execution-plan-resolution.md](../../docs/31-execution-plan-resolution.md)

