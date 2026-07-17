---
id: TASK-024
title: "patterns.md recommends migrating applications to runners.native/docker, which fails at runtime"
type: docs
priority: P2
status: done
effort: XS
created-at: 2026-07-16T22:25:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: convergence-check-2
source-severity: MEDIUM
depends-on: [TASK-017, TASK-050]
completed-at: 2026-07-17T11:45:00+09:00
completion-summary: |
  Resolved by TASK-017 Option A (docker) + TASK-050 Option A (native→process).
  patterns.md migration row applications → stack.runners.native/docker is now
  valid: docker maps to docker plugin; native aliases to process plugin.
  No doc rewrite required.
verification-status: verified
verification-evidence:
  - "TASK-017: runners.docker → Plugin=docker"
  - "TASK-050: runners.native → Plugin=process"
  - "patterns.md migration map agrees with implemented behavior"
---

# Task 024: patterns.md Recommends A Broken Migration

## Summary

`claude-plugin/skills/dva/references/patterns.md` instructed users to migrate
`applications.<name>` → `stack.<name>.runners.native/docker`. That shape passed
validate and then failed at runtime.

## Resolution

**Resolved by TASK-017 + TASK-050 Option A** — both halves of the migration row
are now servable:

| Runner | Mapping |
| --- | --- |
| `runners.docker` | docker plugin (TASK-017) |
| `runners.native` | process plugin alias (TASK-050) |

No patterns.md rewrite required; the documented migration is correct as-is.

## Completion Criteria

- [x] TASK-017 and TASK-050 decided Option A
- [x] migration row agrees with runtime behavior
