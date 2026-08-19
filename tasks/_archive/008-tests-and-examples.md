---
id: TASK-008
archived-at: 2026-04-05T23:02:00+09:00
verified-at: 2026-04-05T23:02:00+09:00
verification-summary: Verified implementation via git commits 2e25daf through 5524b0e.
---
# Task 008: Tests And Examples

## Goal

새 구조 기준으로 테스트와 예제를 정합화한다.

## Scope

- config/lifecycle/cli tests
- examples validation
- docs/example consistency

## Out Of Scope

- core feature 설계 변경

## Deliverables

- 새 구조 기준 테스트 케이스
- 대표 예제 검증
- docs와 examples 사이 정합성 확인

## Acceptance Criteria

- 새 구조 example들이 validation 가능하다
- resolver / merge / CLI 핵심 경로 테스트가 존재한다
- 문서 예시와 실제 schema/struct가 어긋나지 않는다

## Dependencies

- `002-schema-v2`
- `003-merge-semantics`
- `004-resolver`
- `005-cli-lifecycle`
- `006-subprojects`

## References

- [examples/README.md](../../examples/README.md)
- [USAGE.md](../../USAGE.md)
