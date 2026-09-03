---
id: TASK-273
title: "Repair misleading CLI guidance that dead-ends the user"
type: bug
priority: P1
effort: M
exec-tier: standard
created-at: 2026-09-03T00:20:00+09:00
source: "Session audit of internal/cli and internal/config error/advice strings"
scope: "plan_lifecycle.go suppressed-default-plan suggestion, the manifest and help text it echoes, validate.go clean-hook advice"
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
   re-emitting the options the user originally passed
   (`/usr/bin/grep -n 'flags suppress the default plan' internal/cli/plan_lifecycle.go`). A
   comment a few lines above the call site records the invariant this is supposed to respect —
   `/usr/bin/grep -n 'wrote no flag, and "flags suppress the default plan" would be a false account of an' internal/cli/plan_lifecycle.go`
   — but the invariant is only honored for the terminator case; the flag-echo case still
   produces a plan-name form that the plan path rejects.

   **The four options are real, and they are path-conditional.** This is the fact the card
   turns on, so both halves were measured against the built binary at `6e3f581`.

   *Stack path* — a `dva.yml` with two `stack:` entries (`web` tagged `app`, `db` tagged
   `infra`) and **no `plans:` section**. All four options work:
   ```
   $ dva up --dry-run                      → [lifecycle] db   [lifecycle] web
   $ dva up --tag app --dry-run            → [lifecycle] web                       # filtered
   $ dva up --exclude-tag infra --dry-run  → [lifecycle] web                       # filtered
   $ dva up --mode native --dry-run        → ERROR: mode 'native' not found. No modes defined in dva.yml
   $ dva up --env prod --dry-run           → ERROR: env 'prod' not found. No environments defined in dva.yml
   ```
   `--tag`/`--exclude-tag` genuinely narrow the execution set; `--mode`/`--env` are parsed and
   then validated against the config sections — they resolve, and only fail because this
   fixture declares no `modes:`/`environments:`. `down`, `stop`, and `restart` filter
   identically (`dva down --tag app --dry-run` → `[lifecycle] stopping web`). The parser is
   `parseDvaFlags` (`/usr/bin/grep -n 'func parseDvaFlags' internal/cli/compose.go`).

   *Plan path* — the same stack with one plan `local-dev` added. The options disappear:
   ```
   $ dva up --dry-run                       → [plan: local-dev] entries=2
   $ dva up --tag app --dry-run
   ERROR: flags suppress the default plan "local-dev"; name it explicitly: dva up local-dev --tag app
   $ dva up local-dev --tag app --dry-run   # the suggestion, verbatim
   ERROR: unsupported plan flag: --tag
   ```
   `--mode`, `--env`, and `--exclude-tag` dead-end the same way.

   **The guard directs the user to the one action that guarantees failure.** `dva up --tag app`
   is the shape that works on the stack path. The guard reads it as a suppressed default plan
   and tells the user to name the plan explicitly — which moves the invocation onto the plan
   path, where the option is rejected. Following the advice is what breaks it.

   `--dry-run` is *not* affected: it is a root persistent flag (`internal/cli/root.go`,
   `PersistentFlags()`), so `consumeRootPersistentFlags` takes it before the guard runs.
   `parsePlanFlags` (`/usr/bin/grep -n 'unsupported plan flag' internal/cli/plan_lifecycle.go`)
   accepts exactly `--dry-run`, `--force`, `--no-wait`, `-v`/`--volumes`, `--purge`, and
   `--var`; every other `-`-prefixed argument falls through to the error.

   **The advertising surfaces omit the path qualifier.** `internal/cli/manifest.go` lists all
   four options on `up`/`down`/`stop`/`restart` with descriptive help text and no indication
   that they are stack-path-only
   (`/usr/bin/grep -n 'optTag\|optExcludeTag\|optMode\|optEnv' internal/cli/manifest.go`), and
   `internal/cli/manifest_static_commands_test.go` pins that list
   (`/usr/bin/grep -n '"tag", "exclude-tag"' internal/cli/manifest_static_commands_test.go`).
   Neither is *wrong* — the options exist — but an agent reading the manifest cannot tell that
   declaring a plan takes them away. The convention for saying so already exists in the same
   surface, one line below the four: `optVar` reads
   `Override a plan variable; takes a KEY=VAL value (--var KEY=VAL). Plan path only — ignored
   when no plan is being run` (`/usr/bin/grep -n 'optVar' internal/cli/manifest.go`).

   **The qualifier must say `rejected`, not `ignored`.** `--var` and the four options are
   mirror images: `--var` is plan-path-only and is *silently ignored* off its path, whereas the
   four are stack-path-only and are *hard-rejected* off theirs (`unsupported plan flag:
   --tag`). Copying `optVar`'s wording across would be a second false claim. The prose help
   already carries both forms and distinguishes them — `--var` is `Ignored off the plan path.`
   while `--volumes` is `Rejected off the plan path.` — so the precedent for the harsher word
   exists too.

   The prose help is the one surface that gets the path right: it splits `Plan usage:` from
   `Stack flags:` (`dva up --help`). That is why the comment above the `want` map calls its
   list authoritative — the comment was right about the list and simply silent about the path.
   It is not an instance of the blind spot it describes (a documented flag that *stopped* being
   accepted); these four never stopped.

   Two directions are open and this card does not prejudge them:

   - **(a) Implement the four options on the plan path.** Stack entries already carry `tags:`
     and the filtering logic already exists; this connects it to plan routing.
   - **(b) State the path honestly.** Qualify the four in the manifest and in the guard, so
     the manifest says where they apply and the guard stops proposing a form that will be
     rejected.

   Whichever is chosen, the manifest, the static test, the help prose, and the guard message
   must end up saying the same thing.

2. The hook-relocation advice in `internal/config/validate.go` tells the author to move
   dead `clean` hooks "to interaction.clean.exec/steps to keep 'dva clean' as a command of
   its own" (`/usr/bin/grep -n 'interaction.clean.exec/steps' internal/config/validate.go`).
   `exec` is not a property of `interaction_command` in the schema. The valid property list is
   `after, before, command, compose, default_args, description, entrypoint, env_file,
   environment, pod, replace, runner, script, script_file, service, shell, steps,
   subcommands, tags, user, workdir` — read it back with
   `/usr/bin/grep -n '"interaction_command"' internal/config/schema.json` and follow the
   definition. Only `steps` is valid; the message advises writing a schema-invalid field.

## Completion Criteria

- [ ] `rejectSuppressedDefaultPlan` never prints a suggestion that the plan path then rejects — either the echoed option becomes accepted by plan routing, or the suggestion is rewritten to a form the plan path accepts | verify: `go test ./internal/cli -count=1`
- [ ] A regression test exercises all four options (`tag`, `exclude-tag`, `mode`, `env`), not `--mode` alone, running each printed suggestion and asserting it does not fail with `unsupported plan flag` | verify: `go test ./internal/cli -count=1`
- [ ] The stack-path behaviour of all four options is pinned by a test — `--tag`/`--exclude-tag` narrow the execution set, `--mode`/`--env` resolve against the config sections — so no later cleanup can delete them as unused | verify: `go test ./internal/cli -count=1`
- [ ] `manifest.go` makes the applicable path explicit for each of the four, following `optVar`'s qualifier convention but using wording that says the option is *rejected* off its path rather than ignored | verify: `human — read the four option strings: a reader who has declared plans must be able to tell from the manifest alone that the option will be rejected, not silently dropped`
- [ ] The manifest, `manifest_static_commands_test.go`, the `Long` help prose, and the guard message agree on where each of the four options applies | verify: `go test ./internal/cli -count=1`
- [ ] The `clean` hook-relocation advice in `validate.go` no longer names the non-existent `exec` property | verify: `! /usr/bin/grep -q 'interaction.clean.exec' internal/config/validate.go`
- [ ] The replacement advice names only schema-valid `interaction_command` properties and still round-trips through the validator | verify: `go test ./internal/config -count=1`
- [ ] Repository gates pass | verify: `make lint && make test && make test-integration && make doc-check && make commit-check`

## Non-goals

- **Do not remove the four options from the manifest.** They are implemented and working on
  the stack path, measured at `6e3f581`; deleting the advertisement would delete a truthful
  description of real behaviour. An earlier revision of this card offered "stop advertising
  them" as a direction — that was based on a plan-path-only measurement and is withdrawn.
- Implementing the four options on the plan path is *permitted* as direction (a) but is not
  required; direction (b) satisfies this card.
- No change to `--dry-run`, `--force`, `--no-wait`, `-v`, `--purge`, or `--var`, which
  `parsePlanFlags` already accepts and which were verified working at `5ae7a39`.
- No change to the `clean` → `down --purge` migration itself, or to which built-in commands
  are hookable — only the wording of the relocation advice.
- `optForce`'s accuracy is out of scope. Its text (`Compose only: pass --force-recreate; other
  plugins ignore it`) omits that the `restart` plan path discards `flags.force` and hardcodes
  `Force: true`; that is a behaviour defect, not a guidance one, and it is reported separately
  under TASK-269's evidence. An implementer touching the option strings here should leave it
  alone rather than half-fix it.
