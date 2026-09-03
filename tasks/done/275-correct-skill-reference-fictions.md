---
id: TASK-275
title: "Correct fictional behaviour in skill and flow reference docs"
type: bug
priority: P2
effort: M
exec-tier: standard
created-at: 2026-09-03T00:17:00+09:00
source: "Docs audit of skills/dva/references/* and agent-mesh-flows/shared/library/reference-examples.md at HEAD 5eb1af5"
scope: "reference-examples.md clean hook example, skills/dva/references/commands.md flag and hook-count fictions, skills/dva/references/advanced.md hookable-count claim + hook-step command: key + provision example, skills/dva/references/patterns.md version and section-order claims"
status: todo
---

# Task 275: correct skill reference fictions

## Summary

Several skill/reference documents that agents read to configure `dva.yml` or invoke the CLI
describe behaviour the binary does not have: a hook example that `validateHookPlacement`
hard-rejects, CLI flags that were never registered, a flag's effect overstated, a stale log
path, a hookable-command count that includes a removed command, a provision example shape
that cannot unmarshal, and a stated schema version that is behind the binary's actual
version. An agent following any of these produces a config that fails validation or a
command line that errors.

Two of the defects appear in more than one file — the hookable-command count in both
`commands.md` and `advanced.md`, and the `command:`-for-`run:` field confusion in two separate
`advanced.md` examples. Close each **claim**, not each line: fixing the first occurrence and
stopping is how the third copy of the hookable list survived until [TASK-280](../done/280-name-the-live-hookable-set-in-the-schema.md)
found it in `schema.json`.

## Problem

> Line numbers below are as observed at HEAD `5eb1af5` and shift as other sessions commit.
> The quoted strings are the authoritative anchors; locate each defect by string, not by line.

1. `agent-mesh-flows/shared/library/reference-examples.md`, anchor `# --- Clean (reserved) ---`
   (line 364), teaches:
   ```yaml
   clean:
     replace:
       - step: "Clean artifacts and volumes"
         run: "cd {workspace} && cargo clean && docker compose down -v"
   ```
   `validateHookPlacement` (`internal/config/validate.go:227`) special-cases exactly this shape:
   when a top-level hook key is named `clean` it hard-errors with "the 'clean' built-in was
   removed — teardown is 'dva down <plan> --purge', and a flag has no interaction key to
   hook. These hooks now run on nothing", naming `interaction.down.before/after` or
   `interaction.clean.exec/steps` as the two carry-forward shapes instead. The example
   currently teaches the config every reader would write that this validator was written to
   reject.

   **This file is the single source.** `tools/flowgen/main.go:26,34` copies this same
   `dva_flow_examples` block into `agent-mesh-flows/dva-improve.yaml` (confirmed at line 2153)
   and `agent-mesh-flows/dva-improve-guided/30-configure.yaml` (confirmed at line 1611), and
   `make generate`'s `GEN_LIBRARY` step (Makefile:20, Makefile:353) also renders it into
   `internal/cli/library_reference.txt`. Edit only `reference-examples.md`'s example, then run
   `make generate` to propagate — do not hand-edit the three generated copies.

2. `skills/dva/references/commands.md` carries five confirmed fictions:
   - Lines 63-64 document `dva config init --ai` and `dva config init --ai --no-ai-docs`.
     `initCmd.Flags()` (`internal/cli/init.go:81-83`) registers only `--recursive`,
     `--devcontainer`, and `--all`; there is no `--ai` or `--no-ai-docs` flag anywhere in
     `init.go`.
   - Lines 81 and 89 document `dva up local-dev --force # ignore health checks, force restart`
     and a flag table entry "Ignore health, force restart". `Force` reaches exactly one place:
     `internal/lifecycle/compose.go:76-79` appends `--force-recreate` to the compose `up`
     invocation. Nothing in `internal/cli/up.go` or `internal/runner/*.go` reads `Force` to
     skip a health check — the phrase "ignore health checks" describes behaviour that does not
     exist. (The automatic-skip-when-healthy behaviour on line 85 is real and unaffected;
     only the "ignore health checks" wording attached to `--force` is wrong.)
   - Line 141 says process/script log entries "read `.dva/logs/<name>.log`". The actual root is
     `config.DotDirName = ".sb/dva"` with `config.LogsDirName = "logs"`
     (`internal/config/constants.go:11,13`), i.e. `.sb/dva/logs/<name>.log`.
   - Line 325 states "These 7 commands support `before:`, `after:`, and `replace:` hooks",
     listing `up, down, stop, restart, build, clean, logs` (7 names, including `clean`).
     `hookableCommands` (`internal/config/reserved.go:30-34`) has exactly 6 entries — `up,
     down, stop, restart, build, logs` — and its own comment records that `clean` was
     deliberately removed from this set. The list should read 6 commands and drop `clean`.
   - Line 320's "Reserved Command Names" list carries the same kind of drift: it names 27
     commands including `clean`, `app`, `stack`, and `infra`, none of which appear in
     `reservedCommands` (`internal/config/reserved.go:13-21`, 24 entries), and it omits
     `skill`, which the map does reserve. The list should be the 24 names in
     `reservedCommands`, not a hand-maintained copy that has drifted from it.

3. `skills/dva/references/advanced.md` carries three defects. Two of them sit together in the
   "Lifecycle Hooks" section and were added to this card on 2026-09-03, after the original
   audit; the third is the provision example the audit found.

   **3a — the same hookable-count fiction as item 2, in a second file.** Anchor:
   `/usr/bin/grep -n 'The 7 hookable lifecycle commands' skills/dva/references/advanced.md`.
   The line reads "The 7 hookable lifecycle commands (`up`, `down`, `stop`, `restart`,
   `build`, `clean`, `logs`)". `hookableCommands` has six and excludes `clean`, exactly as
   item 2 establishes for `commands.md`. **Fixing `commands.md` alone leaves this one
   standing** — a sweep for the claim, not for the file, is what closes it. There were three
   copies of this list; the third was `schema.json`'s `interaction_command.before`
   description, corrected under [TASK-280](../done/280-name-the-live-hookable-set-in-the-schema.md),
   which also left `TestSchemaDescriptionNamesTheLiveHookableCommands` behind — a test that
   derives the expected list from `HookableCommandList()` rather than restating it. The two
   copies in this card's scope remain hand-written and ungated; see PLAN-004 open question 1
   for who owns closing that.

   **3b — hook steps in the same section use a key the schema rejects.** The example directly
   under 3a's sentence writes:
   ```yaml
   interaction:
     build:
       before:
         - step: "Generate code"
           command: "make generate"
   ```
   `before`/`replace`/`after` items are `#/definitions/provision_item` refs, whose object
   branch sets `"additionalProperties": false` over exactly
   `compose_exec, compose_run, compose_up, note, parallel, run, step` — no `command`. Verified
   by running that config through `validateYAMLSchema`: it is **rejected**, not silently
   ignored, so a reader who copies this block gets a hard schema failure. Three steps in the
   block use `command:`; all three take `run:`. This is the same field confusion as 3c below,
   which is why both live in this item.

   **3c — the provision example does not unmarshal.** `skills/dva/references/advanced.md`
   (around line 411-432) documents:
   ```yaml
   provision:
     default: setup
     profiles:
       setup:
         description: "Initial project setup"
         steps:
           - step: "Install dependencies"
             command: "npm install"
   ```
   `ProvisionConfig.UnmarshalYAML` (`internal/config/config.go:480-503`) accepts only a
   `default_profile` string key plus zero or more `<profile-name>: [ProvisionItem, ...]` keys
   decoded directly as a list — there is no `profiles:` wrapper, no per-profile `description:`
   field, no `steps:` wrapper, and `ProvisionItem` (`internal/config/config.go:507-524`) has no
   `command:` field (the field is `run:`). Every one of those four shapes fails to unmarshal
   against the real type; the example does not parse.

4. `skills/dva/references/patterns.md`:
   - Line 39 says "For current `0.1.44` validation, use `environments.<name>.environment`...".
     `internal/config/version.go:16` sets `Version = "0.1.47"`; `0.1.44` is only
     `MinScaffoldVersion` (line 12), the floor, not the current version. The sentence should
     name the current version, not the floor.
   - Line 34's "Canonical Section Order" text —
     `version -> vars -> environment -> env_file -> stack -> plans -> environments -> sites ->
     checks -> suggestion_ignore -> health_checks -> interaction -> provision -> subprojects ->
     endpoints` — omits `default_plan`, which `canonicalSectionOrder`
     (`internal/config/validate_warnings.go:21-30`) places immediately after `plans`. Add it
     back in that position (the order also omits `default_mode`, `modes`, `modules`, `infra`,
     `ssh`, and `devcontainer` from the same canonical list; fix at minimum the `default_plan`
     omission called out here, and align the rest of the sentence to the full list while the
     line is being edited).

Swept `skills/dva/SKILL.md` and `skills/dva/references/operation-safety.md` for surviving
references to the removed `applications:` section, `stack`/`app`/`infra` commands, and
`--purge` safety wording: both already describe the removal correctly (SKILL.md:136-137,165;
operation-safety.md:66) — no defect found there, so this task carries no changes to those two
files.

## Completion Criteria

- [x] No `clean:` key in `reference-examples.md` is followed by a hook verb; the example uses a shape `validateHookPlacement` accepts (`interaction.down.before/after` or `interaction.clean.steps`, per the validator's own guidance) | verify: `! /usr/bin/grep -A1 -E '^  clean:' agent-mesh-flows/shared/library/reference-examples.md | /usr/bin/grep -q 'replace:'`
- [x] The three generated copies (`internal/cli/library_reference.txt`, `agent-mesh-flows/dva-improve.yaml`, `agent-mesh-flows/dva-improve-guided/30-configure.yaml`) carry the corrected example because `make generate` was run, not because they were hand-edited | verify: `make check-generate`
- [x] `commands.md` no longer documents the unregistered `--ai`/`--no-ai-docs` init flags | verify: `! /usr/bin/grep -q -- '--no-ai-docs' skills/dva/references/commands.md`
- [x] `commands.md` no longer claims `--force` ignores health checks (its only effect is compose `--force-recreate`) | verify: `! /usr/bin/grep -q 'ignore health checks' skills/dva/references/commands.md`
- [x] `commands.md` states the log path under the real dot-dir root | verify: `! /usr/bin/grep -q '\.dva/logs' skills/dva/references/commands.md`
- [x] `commands.md` states the hookable command count as 6 and drops `clean` from that list | verify: `! /usr/bin/grep -q 'These 7 commands' skills/dva/references/commands.md`
- [x] `commands.md`'s Reserved Command Names list matches `reservedCommands` in `internal/config/reserved.go` — 24 names, no `clean`/`app`/`stack`/`infra`, includes `skill` | verify: `human — diff the doc list against the map literal name by name; a count match alone is not evidence` — diffed programmatically as two 24-element sets; identical.
- [x] `advanced.md` states the hookable command count as 6 and drops `clean` from that list, so no copy of the claim survives this card | verify: `! /usr/bin/grep -rq 'The 7 hookable' skills/`
- [x] No hook step example anywhere in `skills/dva/references/` uses `command:` where `provision_item` requires `run:` | verify: `human — for each 'step:' item in a before/replace/after block, confirm the sibling key is run/note/parallel/compose_*, never command; interaction.<name>.command and checks.*.command are different fields and stay as they are` — reviewed all before/replace/after and provision step blocks in advanced.md and reference-examples.md; every sibling key is now `run`.
- [x] `skills/dva/references/advanced.md`'s provision example parses against `ProvisionConfig.UnmarshalYAML` (`default_profile` plus flat `<profile>: [items]` keys, `run:` not `command:`, no `profiles:`/`steps:`/`description:` wrapper) | verify: human — reviewer copies the example's `provision:` block into a scratch `dva.yml` and confirms `dva config validate` does not reject it as unparseable — copied into a scratch dva.yml (with the fixed `clean:`/hook-step examples too) and `dva config validate` returned "✅ dva.yml is valid" (only an unrelated missing-`plans:` info notice).
- [x] `patterns.md`'s Canonical Section Order line includes `default_plan` in the position `canonicalSectionOrder` places it | verify: `/usr/bin/grep -q 'plans -> default_plan' skills/dva/references/patterns.md`
- [x] `patterns.md` no longer presents `MinScaffoldVersion` as the current version | verify: `! /usr/bin/grep -qE 'For current .0\.1\.44' skills/dva/references/patterns.md`
- [x] The replacement wording does not re-pin a version literal that will drift again at the next release | verify: `human — prefer naming the source of truth (internal/config/version.go) over hardcoding a number` — replaced with a pointer to `internal/config/version.go` instead of a literal.
- [x] Repository gates pass | verify: `make lint && make test && make doc-check && make check-generate` — this change touches no Go source (docs/flow examples only); ran the module-scoped equivalents instead: `go test ./internal/config/...` (ok), `go run ./tools/doccheck` (OK), `make check-generate` (clean, exit 0). Full `make lint`/`make test` intentionally not run per this session's single-card, no-full-suite instruction.

## Non-goals

- No new skill content or restructuring of any skill's section layout beyond the corrections above.
- No change to skillgen projection targets — `claude-code`/`antigravity`/`opencode` remain plain symlinks; only `cursor`/`codex` are real conversions, and neither conversion path is touched by this task.
- No change to `validateHookPlacement`, `ProvisionConfig`, `hookableCommands`, `reservedCommands`, or any other runtime behaviour — this task only brings documentation into agreement with the existing implementation.
- No action on `skills/dva/SKILL.md` or `skills/dva/references/operation-safety.md` — swept and confirmed clean at this HEAD.

## Troubleshooting Log

- Symptom: after editing all four target files and confirming them with `git diff`, a later `git status`/`git diff` showed the four card-scoped files back at HEAD content (unmodified) while unrelated files from concurrently-running cards in the same shared working tree stayed edited. Cause: `git reflog` showed a `reset: moving to origin/master` between the two checks — this session shares one working directory across multiple concurrently-running card workers with no per-card worktree isolation, and something in that shared environment periodically hard-resets the tree to newly-integrated `origin/master` commits, discarding any uncommitted edits present at that moment; files other workers had touched again after the reset looked untouched by the reset only because they were re-edited afterward. Resolution: re-applied all edits via a single idempotent Python script (asserting the expected old text first, so a silent no-op edit fails loudly instead of passing), then immediately re-ran `make generate`/verification in the same turn to shrink the exposed window. Took about 20 minutes end-to-end including the second attempt.
