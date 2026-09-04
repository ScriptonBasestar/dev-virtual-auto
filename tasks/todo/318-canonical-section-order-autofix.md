---
id: TASK-318
title: "fmt or migrate --write: apply canonical section order automatically"
type: feature
priority: P3
effort: S
exec-tier: standard
created-at: 2026-09-05T10:30:00+09:00
source: "docs/dogfood/scripton-db-orchestrator.md"
status: todo
---

# Task 318: canonical 섹션 순서 자동 정렬

새 semantic warning "section order … canonical order is [version → env_file → stack → plans → default_plan → environments →
checks → suggestion_ignore → interaction → provision → subprojects → endpoints]"가 수동 재배열을 요구한다.
`config migrate --write`가 순서를 맞추거나 `dva fmt` 동사가 필요. 주석 보존이 관건.

## Completion Criteria

- [ ] 주석 보존 재배열 구현 + 테스트 | verify: `make test`
