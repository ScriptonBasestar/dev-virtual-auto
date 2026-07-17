---
id: TASK-017
title: "Decide stack runners.docker / runners.native semantics"
type: chore
priority: P1
status: done
effort: S
priority-raised-at: 2026-07-16T23:15:00+09:00
priority-raised-reason: "convergence check 2 proved three shipped examples/ files pass validate then hard-fail at stack up on this exact shape (TASK-026)"
created-at: 2026-07-16T20:15:00+09:00
needs-human: false
source-run-id: 20260716T091912Z-73dc094
discovered-in: TASK-009
decision-status: decided
decision-recommendation: "Option A — map runners.docker to the docker plugin"
decision-confidence: medium
moved-at: 2026-07-17T10:55:19+09:00
decided-at: 2026-07-17T10:55:19+09:00
completed-at: 2026-07-17T11:05:11+09:00
decision: "Option A — map runners.docker to the docker plugin"
completion-summary: |
  Option A: decodeRunnerNode("docker") now returns *DockerPluginConfig so
  resolveRunnerPlugin/applyRunnerConfig populates entry.Docker + Plugin=docker.
  Stack runners.docker matches nested docker: and the plan materialization path.
  native stays unservable on the stack path (not a registered plugin); plan path
  still reads NativeRunnerConfig for WorkingDir only. DockerRunnerConfig type kept
  for plan_orchestrator programmatic conversion.
verification-status: verified
verification-evidence:
  - "RED: TestSortedStackResolvesDockerRunnerToPlugin failed Plugin=\"\" want docker"
  - "GREEN: go test ./internal/config/ ./internal/lifecycle/ -count=1 EXIT=0"
  - "Unit: runners.docker + default_runner:docker → Plugin=docker, Docker.Image/Name/Ports set"
  - "Unit: GetRunnerConfig(docker) returns *DockerPluginConfig"
  - "Unit: native still leaves Plugin empty (unservable)"
  - "Regression: nested/flat/compose/script runner shapes still pass"
---

# Decision 017: stack runners.docker / runners.native 의미 확정

## Summary

`stack.<entry>.runners` 아래의 `docker`와 `native`는 lifecycle 플러그인 설정이 아니라
application 스타일 설정으로 디코딩된다. 그래서 `runners.docker`는 `dva stack up`에서
동작하지 않는다. docker는 실제 등록된 플러그인이고 중첩 `docker:` 형태는 동작하므로
이 비대칭은 사용자를 혼란시킨다. schema(TASK-010)가 이 둘을 허용할지 거부할지
결정하려면 의미를 먼저 확정해야 한다.

## Decision (implemented)

**Option A** — map `runners.docker` to the docker plugin.

| Shape | Before | After |
| --- | --- | --- |
| `runners.docker` | `*DockerRunnerConfig` (no plugin consumer) | `*DockerPluginConfig` → `Plugin=docker` |
| nested `docker:` | `*DockerPluginConfig` (works) | unchanged |
| `runners.native` | `*NativeRunnerConfig` (plan WorkingDir only) | unchanged — still not a stack plugin |

### Code changes

- `internal/config/lifecycle.go` — `decodeRunnerNode` docker case → `*DockerPluginConfig`
- `internal/config/lifecycle_helpers_test.go` — positive docker resolution test; native remains unservable

### Explicit non-goals (Option A scope)

- No new `native` lifecycle plugin
- `DockerRunnerConfig` type retained for `plan_orchestrator` conversion case
- Application `run.docker` / `AppDockerRef` untouched
- Schema allowlist work remains TASK-010 follow-up

## Evidence (pre-decision)

| 항목 | 증거 |
| --- | --- |
| docker는 등록된 플러그인 | `internal/lifecycle/plugin_type.go` — registry에 `docker` 포함 |
| 중첩 `docker:` 형태는 동작 | `internal/config/lifecycle.go` → `*DockerPluginConfig` |
| `runners.docker`는 다른 타입 (before) | `decodeRunnerNode` → `*DockerRunnerConfig` |
| `NativeRunnerConfig` 소비처 | `internal/lifecycle/resolver.go` — plan 경로 `WorkingDir`만 |
| `native`는 플러그인이 아님 | registry에 없음 → `NewPlugin("native")` 실패 |

## Options (historical)

### 옵션 A — `runners.docker`를 docker 플러그인에 연결 (**chosen**)

`decodeRunnerNode`의 docker case를 `*DockerPluginConfig`로 변경해 stack에서 실제 동작하게 한다.

### 옵션 B — `docker`/`native`를 stack runners에서 schema로 거부

Not chosen: shipped docs/examples already recommend `runners.docker`.

## Completion Criteria

- [x] 옵션 A/B 중 하나가 선택되어 이 파일에 기록된다 | verify: decision frontmatter + implementation
- [x] Option A implemented for docker | verify: `go test ./internal/config/ -run TestSortedStackResolvesDockerRunnerToPlugin -count=1`
- [ ] 결정에 따라 TASK-010의 schema 범위가 확정된다 | verify: follow-up on TASK-010 (out of scope for this task body)

## References

- [unified.md](../../tmp/gap-analysis-runs/20260716T091912Z-73dc094/unified.md) — G1/G2 맥락
- [009-fix-runners-plugin-resolution.md](../_archive/009-fix-runners-plugin-resolution.md)
