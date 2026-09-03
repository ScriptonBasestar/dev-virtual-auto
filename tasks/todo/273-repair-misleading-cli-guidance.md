---
id: TASK-273
title: "Repair misleading CLI guidance that dead-ends the user"
type: bug
priority: P1
effort: S
exec-tier: standard
created-at: 2026-09-03T00:20:00+09:00
source: "Session audit of internal/cli and internal/config error/advice strings"
scope: "plan_lifecycle.go suppressed-default-plan suggestion, validate.go clean-hook advice"
status: todo
---

# Task 273: repair misleading CLI guidance

## Summary

Two CLI-surfaced messages tell the user to run a command, or write a config field, that does
not work. Both defects turn a helpful-looking error message into a dead end: a user (or an
agent acting on the user's behalf) follows the printed advice and gets a second, unrelated
failure instead of a working invocation.

## Problem

1. `rejectSuppressedDefaultPlan` (`internal/cli/plan_lifecycle.go`) builds its suggested
   command with `"flags suppress the default plan %q; name it explicitly: dva %s %s %s"`,
   re-emitting the legacy flags the user originally passed
   (`grep -n 'flags suppress the default plan' internal/cli/plan_lifecycle.go`). A comment a
   few lines above the call site records the invariant this is supposed to respect —
   `grep -n 'wrote no flag, and "flags suppress the default plan" would be a false account of an' internal/cli/plan_lifecycle.go`
   — but the invariant is only honored for the terminator case; the flag-echo case still
   produces a plan-name form that plans do not accept.

   Reproduced at HEAD (5eb1af5) with a minimal `dva.yml` (one `stack:` entry, one plan named
   `local-dev`):
   ```
   $ dva up --mode native --dry-run
   ERROR: flags suppress the default plan "local-dev"; name it explicitly: dva up local-dev --mode native

   $ dva up local-dev --mode native
   ERROR: unsupported plan flag: --mode
   ```
   The suggested command is not executable — it fails with a second, different error.

2. The hook-relocation advice in `internal/config/validate.go` tells the author to move
   dead `clean` hooks "to interaction.clean.exec/steps to keep 'dva clean' as a command of
   its own" (`grep -n 'interaction.clean.exec/steps' internal/config/validate.go`). `exec` is
   not a property of `interaction_command` in the schema. The valid property list is
   `after, before, command, compose, default_args, description, entrypoint, env_file,
   environment, pod, replace, runner, script, script_file, service, shell, steps,
   subcommands, tags, user, workdir` — read it back with
   `grep -n '"interaction_command"' internal/config/schema.json` and follow the definition.
   Only `steps` is valid; the message advises writing a schema-invalid field.

## Completion Criteria

- [ ] `rejectSuppressedDefaultPlan`'s suggested command is always a working invocation — either the flag form the printed command echoes is accepted by plan routing, or the suggestion is rewritten to a form plans do accept (e.g. dropping the legacy flags, or naming the plan without them) — verified by a regression test that runs the exact suggested command and asserts it does not itself error on an unsupported plan flag | verify: `go test ./internal/cli -count=1`
- [ ] A test reproduces the `dva up --mode native --dry-run` scenario against a single-plan config and asserts the printed suggestion, when re-invoked, does not fail with `unsupported plan flag` | verify: `go test ./internal/cli -count=1`
- [ ] The `clean` hook-relocation advice in `validate.go` no longer names the non-existent `exec` property | verify: `! /usr/bin/grep -q 'interaction.clean.exec' internal/config/validate.go`
- [ ] The replacement advice names only schema-valid `interaction_command` properties and still round-trips through the validator | verify: `go test ./internal/config -count=1`
- [ ] Repository gates pass | verify: `make lint && make test && make test-integration && make doc-check && make commit-check`

## Non-goals

- No redesign of plan-flag grammar (which legacy flags plans should or should not accept)
  — only the printed guidance is in scope.
- No change to the `clean` → `down --purge` migration itself, or to which built-in commands
  are hookable — only the wording of the relocation advice.
