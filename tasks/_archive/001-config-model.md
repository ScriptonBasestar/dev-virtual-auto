---
id: TASK-001
status: done
archived-at: 2026-04-05T23:02:00+09:00
verified-at: 2026-04-05T23:02:00+09:00
verification-summary: Verified implementation via git commits 2e25daf through 5524b0e.
---
# Task 001: Config Model

## Goal

새 설정 구조를 표현할 수 있도록 config model을 재설계한다.

핵심 대상:

- `vars`
- `env_file`
- `stack`
- `plans`
- `environments`
- `sites`
- `subprojects`
- `interactions`
- `provision`

## Scope

- `internal/config/config.go`
- `internal/config/lifecycle.go`
- 관련 보조 타입 파일

## Out Of Scope

- schema validation 구현
- merge semantics 구현
- CLI 연결
- runtime resolver 구현

## Deliverables

- 새 top-level config struct 초안
- multi-runner `stack` 엔트리 타입
- `plans` 엔트리 타입
- `sites.entry_overrides` 타입
- `env_file` 유지 방식 반영

## Acceptance Criteria

- 문서 기준 섹션들이 config struct로 표현된다
- `stack` 엔트리가 multi-runner logical unit으로 표현된다
- `plans.entries[].runner/services/order/depends_on`를 표현할 수 있다
- `modes`, 최상위 `applications` 제거 또는 deprecated 방향이 구조상 반영된다

## References

- [30-config-merge-semantics.md](../../docs/30-config-merge-semantics.md)
- [31-execution-plan-resolution.md](../../docs/31-execution-plan-resolution.md)
- [40-declarative-stack-and-plans.md](../../docs/40-declarative-stack-and-plans.md)

