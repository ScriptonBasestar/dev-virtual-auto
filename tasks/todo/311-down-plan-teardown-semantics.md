---
id: TASK-311
title: "down <plan>: rm-based teardown leaves named volumes and networks"
type: bug
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-05T10:30:00+09:00
source: "reports/{sadawiki,scripton-signalhub,scripton-db-orchestrator,scripton-dns-bridge}.md (dogfood 2026-09-05)"
status: todo
---

# Task 311: `dva down <plan>` teardown 의미론 수정

## Summary

services가 있는 plan의 `down`/`down --volumes`/`--purge`가 `compose rm --force --stop [--volumes] <svc…>`로
실행돼(internal/lifecycle/compose.go composeDownArgs) named volume과 network가 남는다. 그 결과 4개 프로젝트가
`docker compose down -v`를 직접 호출하는 clean/reset interaction을 제거하지 못했다.

## Repro

- `cd ~/mydevbox/scripton-db-orchestrator-devbox && dva --dry-run down infra --volumes` → `compose rm …`
- `cd ~/mydevbox/scripton-dns-bridge-devbox && dva --dry-run down infra` → network 미제거
- services 없는 plan(funbricks-elemhant full-stack)만 `down --remove-orphans --volumes`

## Direction

프로젝트 전체 teardown 옵션(예: `--all` 또는 `--purge`의 의미를 compose down -v로 승격)과
부분 plan의 rm 동작을 구분해 문서화. 기존 `--purge` 의미 변경 시 docs/43 갱신.

## Completion Criteria

- [ ] services 있는 plan에서 named volume/network까지 제거하는 경로 존재 + dry-run 테스트 | verify: `make test`
- [ ] sadawiki/signalhub의 `docker compose down -v` interaction을 dva 동사로 대체 가능함을 dry-run으로 확인 | verify: human
