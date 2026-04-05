---
archived-at: 2026-04-05T23:02:00+09:00
verified-at: 2026-04-05T23:02:00+09:00
verification-summary: Verified implementation via git commits 2e25daf through 5524b0e.
---
# Task 003: Merge Semantics

## Goal

새 구조 기준으로 config merge 로직을 구현한다.

## Scope

- `internal/config/merge.go`
- 관련 테스트

## Out Of Scope

- runtime resolution
- CLI 연결

## Deliverables

- `vars` key별 merge
- `stack` deep merge
- `stack.runners` deep merge
- `plans` deep merge
- `plans.entries` replace
- `sites.entry_overrides` deep merge
- `subprojects.import.*` replace

## Acceptance Criteria

- 문서의 merge semantics와 코드가 일치한다
- `plans.entries`가 부분 merge되지 않는다
- `default_runner`와 runner 선언 무결성이 유지된다
- 기존 restricted field 정책이 새 구조에 맞게 정리된다

## Dependencies

- `001-config-model`
- `002-schema-v2`

## References

- [30-config-merge-semantics.md](/Users/archmagece/myopen/scripton/dev-virtual-auto/docs/30-config-merge-semantics.md)

