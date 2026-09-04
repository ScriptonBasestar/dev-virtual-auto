---
id: TASK-314
title: "logs/build <plan>: scope to plan services and buildable services"
type: bug
priority: P2
effort: S
exec-tier: standard
created-at: 2026-09-05T10:30:00+09:00
source: "reports/{flow-knowchain,scripton-db-orchestrator}.md"
status: todo
---

# Task 314: `dva logs/build <plan>` 서비스 범위 지정

## Repro

- `cd ~/mydevbox/flow-knowchain-devbox && dva --dry-run --debug logs docker-full` → `compose logs -f` (서비스 미지정). profile 뒤 서비스는 로그 미출력.
- `cd ~/mydevbox/scripton-db-orchestrator-devbox && dva --dry-run build hybrid` → `compose build postgres redis kafka prometheus …` (build 컨텍스트 없는 서비스 포함).

## Completion Criteria

- [ ] logs가 plan entry services로 범위를 좁힘, build는 build 컨텍스트 있는 서비스만 전달(또는 인자 생략) | verify: `make test`
