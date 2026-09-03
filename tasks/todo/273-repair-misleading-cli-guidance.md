---
id: TASK-273
title: "Repair misleading CLI guidance that dead-ends the user"
type: bug
priority: P1
effort: S
exec-tier: standard
created-at: 2026-09-03T00:20:00+09:00
source: "Session audit of internal/cli and internal/config error/advice strings"
scope: "plan_lifecycle.go suppressed-default-plan suggestion, the manifest options it echoes, validate.go clean-hook advice"
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

   Reproduced at `5ae7a39` with a minimal `dva.yml` (one `stack:` entry tagged `app`, one
   plan named `local-dev`). **All four suppressing options behave the same way**, so this is
   not specific to `--mode`:
   ```
   $ dva up --dry-run                    # baseline: default plan resolves
   [plan: local-dev] environment= site= entries=1

   $ dva up --tag app --dry-run
   ERROR: flags suppress the default plan "local-dev"; name it explicitly: dva up local-dev --tag app
   $ dva up local-dev --tag app          # the suggestion, verbatim
   ERROR: unsupported plan flag: --tag

   $ dva up --mode native --dry-run      # → dva up local-dev --mode native      → unsupported plan flag: --mode
   $ dva up --env prod --dry-run         # → dva up local-dev --env prod         → unsupported plan flag: --env
   $ dva up --exclude-tag app --dry-run  # → dva up local-dev --exclude-tag app  → unsupported plan flag: --exclude-tag
   ```
   `--dry-run` is *not* affected: it is a root persistent flag
   (`internal/cli/root.go`, `PersistentFlags()`), so `consumeRootPersistentFlags` takes it
   before the guard runs. That boundary is what separates the four broken options from the
   flags that work.

   **No working form exists for any of the four.** `parsePlanFlags`
   (`/usr/bin/grep -n 'unsupported plan flag' internal/cli/plan_lifecycle.go`) accepts exactly
   `--dry-run`, `--force`, `--no-wait`, `-v`/`--volumes`, `--purge`, and `--var`; every other
   `-`-prefixed argument falls through to the error. `--tag`/`--exclude-tag` select stack
   entries, but once a default plan resolves the execution never reaches a path that reads
   them. So the first branch of criterion 1 — "make the printed suggestion executable" — is
   **closed for these four options**; only the rewrite branch is available.

   **The message is a symptom; the manifest is the source.** `internal/cli/manifest.go`
   advertises all four options on `up`/`down`/`stop`/`restart` with descriptive help text
   (`/usr/bin/grep -n 'optTag\|optExcludeTag\|optMode\|optEnv' internal/cli/manifest.go`),
   and `internal/cli/manifest_static_commands_test.go` pins that advertisement
   (`/usr/bin/grep -n '"tag", "exclude-tag"' internal/cli/manifest_static_commands_test.go`).
   The machine-discovery surface an agent reads therefore promises four options the binary
   rejects, and a test enforces the promise. Repairing only the error string leaves that lie
   in place.

   Two directions are open and this card does not prejudge them: implement the four options
   on the plan path (stack entries already carry `tags:`), or stop advertising them and give
   the guard a suggestion that names something real. Whichever is chosen, the manifest, the
   static test, and the guard message must end up saying the same thing.

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

- [ ] `rejectSuppressedDefaultPlan`'s suggested command is always a working invocation — either the echoed option is accepted by plan routing, or the suggestion is rewritten to a form plans do accept — verified by a regression test that runs the exact suggested command and asserts it does not itself error on an unsupported plan flag | verify: `go test ./internal/cli -count=1`
- [ ] The manifest, `manifest_static_commands_test.go`, and the guard message agree on which options `up`/`down`/`stop`/`restart` accept: either all four (`tag`, `exclude-tag`, `mode`, `env`) are implemented on the plan path, or none of them is advertised | verify: `go test ./internal/cli -count=1`
- [ ] A test covers all four options, not `--mode` alone — the defect is uniform across them | verify: `go test ./internal/cli -count=1`
- [ ] A test reproduces the suppressed-default-plan scenario against a single-plan config and asserts the printed suggestion, when re-invoked, does not fail with `unsupported plan flag`; `--dry-run` stays unaffected because `consumeRootPersistentFlags` takes it first | verify: `go test ./internal/cli -count=1`
- [ ] The `clean` hook-relocation advice in `validate.go` no longer names the non-existent `exec` property | verify: `! /usr/bin/grep -q 'interaction.clean.exec' internal/config/validate.go`
- [ ] The replacement advice names only schema-valid `interaction_command` properties and still round-trips through the validator | verify: `go test ./internal/config -count=1`
- [ ] Repository gates pass | verify: `make lint && make test && make test-integration && make doc-check && make commit-check`

## Non-goals

- No new plan-path capability beyond making the surfaces agree. Implementing `--tag`
  filtering on the plan path is *permitted* as one of the two directions above, but this card
  does not require it — refusing to advertise what is not implemented satisfies it equally.
- No change to `--dry-run`, `--force`, `--no-wait`, `-v`, `--purge`, or `--var`, which
  `parsePlanFlags` already accepts and which were verified working at `5ae7a39`.
- No change to the `clean` → `down --purge` migration itself, or to which built-in commands
  are hookable — only the wording of the relocation advice.
