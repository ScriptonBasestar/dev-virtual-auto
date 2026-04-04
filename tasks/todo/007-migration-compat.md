# Task 007: Migration And Compatibility

## Goal

구형 설정에서 새 구조로 넘어갈 수 있도록 migration/compatibility 전략을 만든다.

## Scope

- validation warnings
- compatibility checks
- migration docs or hints

## Out Of Scope

- full automatic converter 구현은 선택

## Deliverables

- 구형 `modes`, `applications`, `stack.*.order` 감지
- 친절한 migration hint
- legacy command/section deprecation 정책
- 필요 시 converter 초안

## Acceptance Criteria

- 구형 설정을 읽을 때 무엇을 어떻게 옮겨야 하는지 안내된다
- validation 메시지가 문서와 일치한다
- 사용자가 새 구조로 이동할 경로가 명확하다

## Dependencies

- `001-config-model`
- `002-schema-v2`
- `005-cli-lifecycle`

## References

- [40-declarative-stack-and-plans.md](/Users/archmagece/myopen/scripton/dev-virtual-auto/docs/40-declarative-stack-and-plans.md)
- [USAGE.md](/Users/archmagece/myopen/scripton/dev-virtual-auto/USAGE.md)

