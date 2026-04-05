# Task 005: CLI Lifecycle

## Goal

CLI를 `plans` 기반 실행 모델에 맞춘다.

## Scope

- `internal/cli/`
- lifecycle wiring
- help text / usage text

## Out Of Scope

- schema/merge core 구현
- subproject namespace execution

## Deliverables

- `dva up <name>`
- `dva down <name>`
- `dva stop <name>`
- `dva status [name]`
- `dva ls`
- `dva show <name>`

## Acceptance Criteria

- 실행 대상이 `stack`이 아니라 named execution entry로 해석된다
- `dva ls`가 실행 가능한 이름을 보여준다
- `stack up/down/...` 계열은 legacy 또는 inspection-only로 정리된다
- help / usage가 새 모델 기준으로 일관된다

## Dependencies

- `004-resolver`

## References

- [40-declarative-stack-and-plans.md](/Users/archmagece/myopen/scripton/dev-virtual-auto/docs/40-declarative-stack-and-plans.md)
- [USAGE.md](/Users/archmagece/myopen/scripton/dev-virtual-auto/USAGE.md)

