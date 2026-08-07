---
id: TASK-163
title: "Decide whether detectedProject survives as a name or collapses to a boolean"
type: decision
priority: P4
effort: S
created-at: 2026-08-03T15:40:00+09:00
completed-at: 2026-08-07
decision: C
scope: "dva repo — internal/runner/docker_compose.go, compose.go"
status: done
quality-review: pass
quality-reviewed-at: 2026-08-07T18:05:08+09:00
verified-at: 2026-08-07T18:05:08+09:00
archived-at: 2026-08-07T18:05:08+09:00
quality-review-evidence: |
  - kind: test
    command-or-step: make test && make doc-check (mise go 1.26.4)
    result: exit 0; shared suite green
  - kind: recheck
    command-or-step: acceptance criteria re-observed
    result: decision C; detectedProject stays string
verification-summary: |
  quality-review pass; re-checked deliverables. decision C; detectedProject stays string. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 163: The deferred `detectedProject` simplification

## Decision: **C — keep the name and record why**

`detectedProject` stays a **project name string**, not a "running" boolean.

### Why not B (boolean)

When `project_name:` is **absent** from dva.yml:

1. `ComposeArgv` omits `--project-name` (compose infers from directory).
2. Detection reads the **actual** project name from the running container.
3. That name reaches exec only via `detectedProject` → `composeArgv` `projectOverride`.

A boolean would preserve "is up" and drop "which project", breaking
`TestProjectNameUsesDetectionWhenConfigDeclaresNone`.

When `project_name:` **is** declared, the string often matches config and looks
redundant — that is harmless. One field covers both cases.

### What would make B correct later

Resolve an effective project name once at load/detect, then let detection be
boolean *against that name*. That is a resolve-pipeline change, not a P4 field trim.

### Evidence

Comments on `DockerComposeRunner.detectedProject` and `composeArgv` projectOverride
document the undeclared-`project_name` path. No behaviour change.

```
go test ./internal/runner/ -run ProjectName -count=1
```
