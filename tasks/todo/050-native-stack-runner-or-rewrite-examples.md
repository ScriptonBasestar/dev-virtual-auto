---
id: TASK-050
title: "native stack default_runner still unservable after TASK-017 Option A (docker only)"
type: bug
priority: P1
status: todo
effort: M
created-at: 2026-07-17T11:15:00+09:00
depends-on: [TASK-017]
parent: null
source: residual from TASK-026 after TASK-017 docker-only Option A
---

# Task 050: Native Stack Runner Residual

## Summary

TASK-017 Option A mapped `runners.docker` to the docker plugin. Shipped examples and docs still use `default_runner: native` + `runners.native`, which remains unregistered. TASK-026 stays blocked until this residual is fixed.

## Options (inherit product choice)

- **A** — implement a stack lifecycle plugin for `native` (or alias to process/script)
- **B** — rewrite the 12 native example/doc locations toward `process`/`script`/`applications` and reject `native` on the stack path in schema

## Completion Criteria

- [ ] One option chosen and applied so `examples/full-stack.yml` validate+stack up does not hard-fail on native entries
- [ ] TASK-026 unblocked and closed or superseded
- [ ] `make test` passes
