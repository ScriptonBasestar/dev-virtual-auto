---
id: TASK-312
title: "--dry-run up waits on native entry health checks"
type: bug
priority: P1
effort: S
exec-tier: standard
created-at: 2026-09-05T10:30:00+09:00
source: "docs/dogfood/{scripton-db-orchestrator,scripton-dns-bridge}.md"
status: done
---

# Task 312: `--dry-run up <plan>`이 native health check를 실제 대기

## Repro

`cd ~/mydevbox/scripton-db-orchestrator-devbox && timeout 20 dva --dry-run up hybrid`
→ compose/api dry-run 라인 출력 후 `http://localhost:11100/health/live` ready_timeout(120s)까지 블록.

## Completion Criteria

- [x] dry-run에서 health wait를 건너뛰고 "would wait for …"만 출력, 테스트 추가 | verify: `make test`
- 2026-09-05 추가 재현: scripton-gitrump-devbox `dva --dry-run up dev`(native gitrumpd, ready_timeout 60)가 200초 이상 정지, kill 필요. 이 프로젝트에서는 dry-run으로 plan 확인 자체가 불가능했다.

## Resolution (2026-09-05)

- 원인: `Orchestrator.Up`(internal/lifecycle/orchestrator.go)의 entry-level health wait가
  `opts.Wait`(CLI 기본 true)만 보고 `opts.DryRun`을 확인하지 않았다. 이 경로의
  `WaitUntilReadyWithContext`는 ready_timeout을 적용하지 않아 ctx 취소까지 폴링한다.
  `PlanChildExecutor.WaitReady`와 `startModeProcesses`는 이미 dry-run을 건너뛰고 있었다.
- 수정: dry-run이면 대기하지 않고
  `[health] (dry-run) would wait for entry "<name>": <check>=<type> <target> (ready_timeout=Ns)`
  한 줄만 stderr에 출력 (`describeHealthChecks`, health.go).
- 테스트: `TestUpDryRunSkipsEntryHealthWait`(orchestrator_dry_run_health_unix_test.go) —
  닫힌 포트 http check + DryRun/Wait. 수정 라인을 되돌리면 ctx 5s 타임아웃까지 대기해
  실패하는 것을 확인함.
- 검증: `make build`, `make test`, `make lint` exit 0.
