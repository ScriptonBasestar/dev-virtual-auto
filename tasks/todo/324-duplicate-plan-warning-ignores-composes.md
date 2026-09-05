---
id: TASK-324
title: "duplicate-plan warning ignores composes and misfires on composition plans"
type: bug
priority: P2
effort: S
exec-tier: standard
created-at: 2026-09-05T14:00:00+09:00
source: "docs/dogfood/flow-taskchain.md"
status: todo
---

# Task 324: composition plan 쌍이 항상 "equal declaration" warning

## Repro

flow-taskchain-devbox `dva validate`:
`local-dev`(composes: local-infra, engine/dev, mcp/dev)와 `local-full`(composes: local-infra-full + 4 child)은
composes가 다른데 `plans "local-dev" and "local-full" declare equal environment, site, vars, endpoint_tags, and entries` warning이 뜬다.

## Cause

`internal/config/validate_warnings.go` `plansHaveEqualDeclaration`이 TASK-244 D6 필드만 비교하고
`Composes`(TASK-260)를 보지 않는다. composition plan은 entries가 비어 있어 두 개만 있어도 항상 동일 판정.

## Completion Criteria

- [ ] `Composes`(plan, order, depends_on)를 비교에 포함하고, 메시지에 composes를 언급 | verify: `make test`
- [ ] composes가 다른 plan 쌍은 warning 없음 / 같은 쌍은 warning 유지 테스트 | verify: `go test ./internal/config/ -run Duplicate`
- [ ] flow-taskchain-devbox `dva validate` warning 0 | verify: human — 출력 첨부
