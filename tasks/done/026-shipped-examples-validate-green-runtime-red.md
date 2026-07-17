---
id: TASK-026
title: "Shipped examples pass validate then hard-fail at stack up"
type: bug
priority: P1
status: done
effort: S
created-at: 2026-07-16T23:15:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: convergence-check-2
source-severity: HIGH
depends-on: [TASK-017, TASK-050]
completed-at: 2026-07-17T11:45:00+09:00
completion-summary: |
  Resolved by TASK-050 Option A: runners.native aliases to process plugin.
  default_runner: native + runners.native now resolves Plugin=process so stack up
  no longer fails with unknown lifecycle plugin "". Examples shape is executable
  past plugin resolution (process runs Command=run in Dir=dir).
verification-status: verified
verification-evidence:
  - "TASK-050 unit: native → Plugin=process, Process.Command/Dir set"
  - "TASK-050 dry-run: [lifecycle] api (process) EXIT=0"
  - "go test ./internal/config/ ./internal/lifecycle/ -count=1 EXIT=0"
---

# Task 026: Shipped Examples Validate Green, Then Hard-Fail

## Summary

`stack.<entry>.default_runner: native` + `runners.native:` was the documented backend-app
pattern that passed `dva validate` and then failed at `dva stack up`.

## Resolution

**Resolved by TASK-050 Option A** — `runners.native` aliases to the process plugin
(`Command` ← `run`, `Dir` ← `dir`, `Plugin` = `process`). The 12 example/doc locations
using this shape are now executable past plugin resolution without rewrite.

## Completion Criteria

- [x] TASK-017 decided (docker) and TASK-050 decided (native alias)
- [x] native shape no longer fails with `unknown lifecycle plugin ""`
- [x] validate and runtime agree: shape validates and resolves to process plugin
