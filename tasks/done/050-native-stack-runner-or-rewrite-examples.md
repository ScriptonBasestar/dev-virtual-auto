---
id: TASK-050
title: "native stack default_runner still unservable after TASK-017 Option A (docker only)"
type: bug
priority: P1
status: done
needs-human: false
effort: M
created-at: 2026-07-17T11:15:00+09:00
depends-on: [TASK-017]
parent: null
source: residual from TASK-026 after TASK-017 docker-only Option A
decision-status: decided
decision: "Option A — alias stack runners.native → process plugin"
decided-at: 2026-07-17T11:44:00+09:00
completed-at: 2026-07-17T11:45:00+09:00
completion-summary: |
  Option A: applyRunnerConfig(*NativeRunnerConfig) maps to ProcessPluginConfig
  {Command: Run, Dir: Dir}; resolveRunnerPlugin sets Plugin="process" (not "native").
  NativeRunnerConfig type retained for plan WorkingDir path. No separate NativePlugin.
  Unblocks TASK-026 (examples shape executable past plugin resolution) and TASK-024
  (patterns.md migration to runners.native is now valid).
verification-status: verified
verification-evidence:
  - "GREEN: go test ./internal/config/ -count=1 EXIT=0"
  - "GREEN: go test ./internal/lifecycle/ -count=1 EXIT=0"
  - "Unit: default_runner:native + runners.native {dir,run} → Plugin=process, Process.Command/Dir set, DetectPlugin()=process"
  - "Unit: GetRunnerConfig(native) still returns *NativeRunnerConfig"
  - "Unit: docker mapping from TASK-017 intact (TestSortedStackResolvesDockerRunnerToPlugin)"
  - "Dry-run: temp dva.yml native entry → [lifecycle] api (process) command=echo hello dir=apps/api EXIT=0"
---

# Task 050: Native Stack Runner Residual

## Summary

TASK-017 Option A mapped `runners.docker` to the docker plugin. Shipped examples and docs still use `default_runner: native` + `runners.native`, which remained unregistered. TASK-026 stayed blocked until this residual was fixed.

## Decision (implemented)

**Option A** — alias `stack runners.native` to the process plugin (consistent with TASK-017 docker mapping and patterns.md migration TO runners.native).

| Shape | Before | After |
| --- | --- | --- |
| `runners.native` | `*NativeRunnerConfig` (Plugin empty) | still `*NativeRunnerConfig` in map; `Process` filled; `Plugin=process` |
| nested `process:` | `*ProcessPluginConfig` (works) | unchanged |
| `runners.docker` | `*DockerPluginConfig` (TASK-017) | unchanged |

### Code changes

- `internal/config/lifecycle_helpers.go` — `applyRunnerConfig` case `*NativeRunnerConfig` → Process; `resolveRunnerPlugin` maps name `native` → Plugin `process`
- `internal/config/lifecycle_helpers_test.go` — positive native→process test (replaces unservable negative test)

### Explicit non-goals

- No separate `NativePlugin` type
- `NativeRunnerConfig` type retained for plan WorkingDir
- Application `run.native` / AppExecPaths untouched

## Completion Criteria

- [x] One option chosen and applied so `examples/full-stack.yml` validate+stack up does not hard-fail on native entries
- [x] TASK-026 unblocked and closed or superseded
- [x] `go test ./internal/config/ ./internal/lifecycle/` passes
