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
status: done
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

   Two directions were open:

   - **(a) Implement the four options on the plan path.** Stack entries already carry `tags:`
     and the filtering logic already exists; this connects it to plan routing.
   - **(b) State the path honestly.** Qualify the four in the manifest and in the guard, so
     the manifest says where they apply and the guard stops proposing a form that will be
     rejected.

   Whichever is chosen, the manifest, the static test, the help prose, and the guard message
   must end up saying the same thing.

   **Direction taken: (b).** (a) remains open and unprejudged — nothing here forecloses
   teaching plan routing to filter by tag; it would replace the qualifier with a wider
   statement rather than contradict one. (b) was chosen because the harm the card is about is
   the *advice*, and (b) removes it without changing which invocations run.

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

- [x] `rejectSuppressedDefaultPlan` never prints a suggestion that the plan path then rejects — either the echoed option becomes accepted by plan routing, or the suggestion is rewritten to a form the plan path accepts | verify: `go test ./internal/cli -count=1`
- [x] A regression test exercises all four options (`tag`, `exclude-tag`, `mode`, `env`), not `--mode` alone, running each printed suggestion and asserting it does not fail with `unsupported plan flag` | verify: `go test ./internal/cli -count=1`
- [x] The stack-path behaviour of all four options is pinned by a test — `--tag`/`--exclude-tag` narrow the execution set, `--mode`/`--env` resolve against the config sections — so no later cleanup can delete them as unused | verify: `go test ./internal/cli -count=1`
- [x] `manifest.go` makes the applicable path explicit for each of the four, following `optVar`'s qualifier convention but using wording that says the option is *rejected* off its path rather than ignored | verify: `human — read the four option strings: a reader who has declared plans must be able to tell from the manifest alone that the option will be rejected, not silently dropped`
- [x] The manifest, `manifest_static_commands_test.go`, the `Long` help prose, and the guard message agree on where each of the four options applies | verify: `go test ./internal/cli -count=1`
- [x] The `clean` hook-relocation advice in `validate.go` no longer names the non-existent `exec` property | verify: `! /usr/bin/grep -q 'interaction.clean.exec' internal/config/validate.go`
- [x] The replacement advice names only schema-valid `interaction_command` properties and still round-trips through the validator | verify: `go test ./internal/config -count=1`
- [x] Repository gates pass | verify: `make lint && make test && make test-integration && make doc-check && make commit-check`

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
  under TASK-279. An implementer touching the option strings here should leave it
  alone rather than half-fix it.

## Evidence

Measured on the branch binary against a two-entry fixture (`web` tagged `app`, `db` tagged
`infra`) with and without a `plans:` section. Three facts the card did not have:

1. **The guard fires for every leading flag, not only the four.** `up`/`down`/`stop`/`restart`
   answer `flags suppress the default plan` for `--force`, `--no-wait`, `--var K=V`, `--purge`,
   `-v` and an unknown flag too. For all of those the original suggestion is *correct* — the
   named-plan form runs. So the message could not be rewritten wholesale without breaking the
   cases it already got right; the repair discriminates, stripping only
   `stackPathOnlySelectorFlags` and leaving the original message otherwise untouched. That is
   what `TestSuppressedDefaultPlanKeepsWorkingSuggestions` pins.

2. **`build` is not one of the four commands.** It calls `parseDvaFlags` *before*
   `detectPlanRoute`, so the selectors are consumed off the raw args and never reach
   `parsePlanFlags`: `dva build <plan> --mode native` runs where `dva up <plan> --mode native`
   is rejected, and the guard never fires on `build` at all. `optMode` was shared by all five
   commands, so appending the qualifier to it would have published a false claim on `build`.
   `optModeBuild` holds the unqualified text and `build` alone uses it;
   `TestManifestQualifiesStackPathOnlySelectors` asserts the exception rather than leaving it
   to a reader of the call sites. (Separately observed and *not* fixed here: `build` accepts
   `--tag`/`--exclude-tag`/`--env` and ignores them — `--exclude-tag app` still built `web`,
   and `--env prod` did not fail against a config with no `environments:`. That is a behaviour
   defect, not a guidance one, and it belongs with TASK-279's family.)

3. **`status` never reaches the guard.** It uses cobra flag parsing, so `dva status --tag app`
   answers `unknown flag: --tag` first. Nothing to repair there.

`--dry-run`, `--debug` and `--json` were split out of `stackSelectorFlags` into a separate
`stackPathOnlySelectorFlags` for exactly this reason: they are root persistent flags that work
on both paths, and a guard that told the user to drop `--dry-run` when naming a plan would
invent a restriction that does not exist.

### Tests

- `internal/cli/plan_path_flag_guidance_test.go`
  - `TestSuppressedDefaultPlanSuggestionRuns` — 8 spellings of the four selectors
    (`--tag`, `--tag=`, `-T`, `--exclude-tag`, `--mode`, `-M`, `--env`, `-E`) x 4 commands,
    each parsing the suggestion back out of the printed message and re-running it. Criteria 1
    and 2. It re-runs the text rather than asserting on wording, because an assertion on
    wording would have passed for the message this card was filed against.
  - `TestSuppressedDefaultPlanKeepsWorkingSuggestions` — the other half: `--force`,
    `--no-wait`, `--var K=V`, `--purge`, `-v` keep the original suggestion and it runs.
  - `TestSuppressedDefaultPlanStripsOnlyTheSelectors` — a mixed invocation keeps `--no-wait`
    and `--var K=V`, drops `--tag`/`--mode`, and names both removed flags in the message.
  - `TestStackPathSelectorsNarrowAndResolve` — criterion 3, with an unfiltered baseline so the
    two filter assertions cannot pass on a fixture that only ever runs one entry. It reads the
    `[lifecycle] <entry>` execution lines, not bare names: every run also prints a `Lifecycle:`
    summary of every *declared* entry, and a substring test against that reports the excluded
    entry as run.
  - `TestManifestQualifiesStackPathOnlySelectors` — 16 option strings carry the qualifier, none
    borrows `optVar`'s "ignored", `build --mode` does not carry it.
  - `TestLongHelpAgreesWithTheManifest` — the four are listed under the qualified heading in
    all four `Long` strings.
- `internal/config/clean_hook_advice_test.go`
  - `TestCleanHookAdviceNamesSchemaValidProperties` — criterion 7. It parses the relocation
    targets out of the real error message, checks each against the property list read from
    `schema.json`, and round-trips a config written in that shape through `validateYAMLSchema`
    and `Validate()`. Reverting the message to the old spelling fails it with
    `interaction_command has no such property`, so it is a regression test and not a
    restatement.

### Out of scope, observed

`schema.json`'s `interaction_command.before` description still lists `clean` among the hookable
built-ins. That is the same class of false claim as this card's item 2, but the Non-goals bar
changing "which built-in commands are hookable", and the description is a statement about that
set rather than about the relocation advice. Left for a separate card.

## Reopened as TASK-283 (2026-09-03)

An independent review by a reviewer that did not write this card found the suppressed-default-plan
suggestion still dead-ends on four inputs, and regresses on a fifth. The implementer re-measured
all five against a fresh `make build` of master at `916b07e` and reproduced every one.

The regression is the reason this note exists rather than a silent follow-up card. `--dry-run` is
a root persistent flag, consumed before the guard runs, so `dva up --tag app --dry-run` now
suggests `dva up p1` — and following that suggestion **starts the entry**. Before `206918a` the
same input suggested `dva up p1 --tag app`, which exits 1 and runs nothing. For this one input the
change made the outcome worse, not better: a preview request became a real action.

Also still open: `logs` asserts a whole-stack path it does not have (`--tag` is rejected there
too, and the `logs` manifest entry has no Options, so the runtime message contradicts the manifest
this card required it to agree with); `restart`'s positional entry name is left in a suggestion the
plan route answers with `unexpected argument in plan mode`; and a flag-shaped selector value is
either stranded as a positional or swallowed unreported.

The work is [TASK-283](../todo/283-repair-plan-route-flag-guidance.md). What this card did fix and
the review confirmed correct — the `selectors.go` split, the `--mode` manifest qualifier,
`build`'s divergent route, and the `validate.go` clean-hook advice — stands.
