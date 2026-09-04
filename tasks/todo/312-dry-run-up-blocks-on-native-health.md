---
id: TASK-312
title: "--dry-run up waits on native entry health checks"
type: bug
priority: P1
effort: S
exec-tier: standard
created-at: 2026-09-05T10:30:00+09:00
source: "docs/dogfood/{scripton-db-orchestrator,scripton-dns-bridge}.md"
status: todo
---

# Task 312: `--dry-run up <plan>`이 native health check를 실제 대기

## Repro

`cd ~/mydevbox/scripton-db-orchestrator-devbox && timeout 20 dva --dry-run up hybrid`
→ compose/api dry-run 라인 출력 후 `http://localhost:11100/health/live` ready_timeout(120s)까지 블록.

## Completion Criteria

- [ ] dry-run에서 health wait를 건너뛰고 "would wait for …"만 출력, 테스트 추가 | verify: `make test`
- 2026-09-05 추가 재현: scripton-gitrump-devbox `dva --dry-run up dev`(native gitrumpd, ready_timeout 60)가 200초 이상 정지, kill 필요. 이 프로젝트에서는 dry-run으로 plan 확인 자체가 불가능했다.
