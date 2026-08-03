---
id: TASK-081
title: "`dva show` names plans and interactions but never a stack entry, so the answer to \"what is declared\" needs two commands"
type: fix
priority: P4
status: todo
effort: S
created-at: 2026-07-30T00:00:00+09:00
reopened-at: 2026-08-03T12:30:00+09:00
reopened-because: "show.go's --help still hand-enumerates the sections, and the list is already stale by three"
scope: "internal/cli — show.go (stack section, text + JSON, help text); internal/config — LifecycleEntry.DefaultRunnerName; skills/dva/references/commands.md; tests"
---

# Task 081: One command that names everything the config declares

## Problem

This task corrects a claim, so the correction comes first.
[TASK-074](../_archive/074-app-subcommands-answer-an-absent-section-three-ways.md) recorded that "no command
answers *what does this config declare?*" — **false**, and filed without being checked: `dva show`
answers it for plans and interactions, `dva status` for stack entries.

What was wrong is narrower. `dva show` reported the entry's *runner* as a heading — `Compose:`, with
the compose files under it — and never printed `infra`, the name every other command takes as its
argument. On a config with plans it printed the plan names and stopped. `dva status` was the only
command naming stack entries, and it presents them as runtime state, so it needs something running.

## Resolution

`show` gained a stack section, in declaration order (`order`, then name) — what the entries declare,
not a prediction of every command's sequence: `dva up <plan>` walks the plan's own entries instead.

```
Stack (dva stack up <name>):
  api     REST API server [runners:docker,native, default:native]
  infra   PostgreSQL and Redis infrastructure [runner:compose]
  web     Svelte frontend [runners:docker,native, default:native]
  worker  Background worker [runners:docker,native, default:native]
```

Placed **before** `Compose:` deliberately: that block reports one runner's settings, so read first
it is a heading with no owner. Read after, `compose.yml` visibly belongs to `infra`.

`stackViews()` is the single source for both `showText` and `showJSON`, because a consumer seeing
`compose` with no `stack` has the same gap the human reader did. Every shape is rendered from
`LifecycleEntry.RunnerNames()` rather than a second copy of the plugin-detection rules:

| declaration | row |
| --- | --- |
| `runners: {compose: …}` | `[runner:compose]` |
| `runners: {docker, native}` + `default_runner` | `[runners:docker,native, default:native]` |
| `plugin: script` + nested `script:` | `[runner:script]` |
| nothing | `[no runner declared]` |

`default:` is suppressed when it names the entry's only runner — noise the reader would have to
rule out. Both sides are canonicalized first (`DefaultRunnerName()`, new, mirroring what
`RunnerNames()` already does), so `default_runner: podman_compose` against
`runners.podman_compose` reads as one runner rather than as a default pointing at an undeclared
one. `order:N` prints only when declared, so the 0 default is not dressed up as a decision.

Two stale hand-written enumerations of `show`'s sections — its own `--help` and
`skills/dva/references/commands.md` — were changed to stop enumerating rather than to add one item.
Both already omitted `compose`, `plans` and `sites`.

## Evidence

The header advertises a command, so it was executed rather than quoted
([TASK-076](../_archive/076-manifest-advertises-the-one-invocation-that-cannot-reach-the-interaction.md) set
that precedent): `dva show` printed `alpha`, and `dva stack up alpha` ran
`[lifecycle] alpha (script)` → `$ echo up-alpha`. The name `show` prints is the name the command
takes.

Mutation checks — a *not-contains* assertion passes trivially if the section never renders:

| mutation | result |
| --- | --- |
| drop the name tiebreak `stackViews` then held | FAIL — determinism test, "render 4 differs" |
| print `default:` unconditionally | FAIL — `TestShowNamesStackEntries` |
| print `order:N` when undeclared | **passed at first** — see below |
| `no runner declared` → empty bracket | FAIL — `TestShowNamesStackEntries` |
| drop `data["stack"]` from `showJSON` | FAIL — `TestShowJSONNamesStackEntries` |
| drop the section header from `showText` | FAIL — `TestShowNamesStackEntries` (Fatal) |
| un-canonicalize the default (revert the A1 fix) | FAIL — both stack tests |
| drop `data["compose"]` from `showJSON` | FAIL — `TestShowJSON_FullConfig`; **passed** before |

Two rows **passed**, which is the point of running them. The third: every fixture entry declared an
order, so "must not print `order:0`" had nothing to catch — the fixture gained an entry declaring
nothing, and it then failed. The last: with the compose block deleted, the old
`Contains(output, "compose")` reported `ok` where the rewrite reports `FAIL`. Restores `diff -q`'d.

Determinism was measured through the binary too: 15 runs of `dva show` on two fixtures produced
**1** distinct stack section each; `dva stack status` on the same fixture produced **5**.

## Findings

**The declared startup sequence is not the same twice.** `SortedStack()` sorts on `Order` alone
with an unstable sort, so entries sharing an order — including the default where none declares
`order:` — come out in map-iteration order, and `NewOrchestrator` computes that slice once for
`Up`/`Down`/`Stop`/`Restart`/`Status`. Measured 5 distinct sequences in 20 runs on a config
`validate` calls valid; filed as
[TASK-084](../_archive/084-stack-up-walks-a-different-sequence-each-run.md), since fixed: `SortedStack` has the
tiebreak and `stackViews`' local copy is gone.

**A fixture no config can load proves less than it appears to — and over-correcting is its own
defect.** The first test built `infra` with an entry-level `Compose` struct, which load *rejects*
(`rejectLegacyComposeShape`), so it would have asserted on a config no user can write. Rebuilt
around shapes that load — but the rebuild then labelled `void`, the entry declaring nothing,
directly-built-only, and **that was false**: `void: {description: ...}` loads and validates at exit
0, since a stack entry has no required fields in `schema.json`. Wrong in the direction that invites
deletion — a live branch documented as test-only.

**One raw read of a field every other reader canonicalizes.** Suppression compared canonical
`RunnerNames()` output against raw `e.DefaultRunner`, so `default_runner: podman_compose` beside
`runners.podman_compose` rendered `[runner:podman-compose, default:podman_compose]` — asserting the
opposite of what its own comment promised — and JSON emitted a `default_runner` matching no element
of `runners`. Not a judgement call: every other `DefaultRunner` read already wraps it in
`normalizeRunnerName` (`lifecycle.go:504`, `:735`, `source.go:83`, `resolver.go:155`), so `show.go`
was the lone outlier against four precedents.

**Adding a section can weaken a test that never mentions it.** A substring assertion over a whole
document is a claim any later addition can silently satisfy — here `Contains(output, "compose")`
against a fixture whose stack entry is *keyed* `compose`.

## Non-goals

- `dva ls` still lists `interaction:` only — surprising, but a separate contract with its own users.
- `show` and `status` stay separate (declarations vs runtime state), and no new top-level command:
  `show` already claims this job.

## Acceptance criteria

- [x] `dva show` names every stack entry and its runner | verify: `go test ./internal/cli/ -run TestShowNamesStackEntries`
- [x] It does so on a config that also declares plans | verify: `human — dva show on examples/applications.yml printed the 4 stack entries and both plans`
- [x] The existing sections render unchanged | verify: `human — dva show on examples/applications.yml, byte-identical below the new section; the only edits to existing lines are Short/Long and one test assertion`
- [x] A config declaring nothing prints something truthful | verify: `human — version-only config: header alone, no empty Stack: heading, no "stack": {} in --json`
- [x] Both surfaces carry it, from one source | verify: `go test ./internal/cli/ -run TestShowJSONNamesStackEntries`
- [x] The listing is reproducible | verify: `go test ./internal/cli/ -run TestShowStackOrderIsStableAcrossRenders`
- [x] The advertised command accepts the name printed | verify: `human — dva stack up alpha ran the entry; see Evidence`
- [x] The assertions are not vacuous | verify: `human — the 8 mutations above; two passed and each exposed a real gap`
- [x] `default_runner` and `runners` are comparable in both surfaces | verify: `go test ./internal/config/ -run TestDefaultRunnerNameMatchesRunnerNames`
- [ ] No hand-written enumeration of show's sections is left | verify: `test $(/usr/bin/sed -n '14,24p' internal/cli/show.go | /usr/bin/grep -cE 'environments \(--env\)|stack entries, plans, commands') -eq 0`
- [x] Full suite passes under -race | verify: `make test`

## Left open

- **`dva stack up <typo>` exits 0.** Measured while verifying the header: an unmatched name prints
  `[warn] no lifecycle entries matched filters` and returns success, so a misspelled name reads as a
  completed start. Same class as
  [TASK-083](../_archive/083-a-step-without-run-announces-work-it-never-does.md) — success reported for
  work not done — different code path.
- **JSON keeps a redundant `default_runner` that the text row hides, and uses three emission rules
  in one object** — `description`/`order` unconditional (so `void` gets `""` and `0`),
  `default_runner` conditional. Intentional for the human/consumer split, and the unconditional
  pair matches the rest of `showJSON`; the rationale now sits beside the suppression in `show.go`
  rather than only in a test comment. One rule for all three is still worth a decision someday.
- **Rows do not show tags**, although the section's rationale cites the tag filters and
  `applications` already surfaces tags in JSON. An addition rather than a correction.
- The TASK-074 message can now route to `dva show` instead of inlining its own
  `stack (1), interaction (1)` counts. Not done here; it is that task's line to change.

## Related

TASK-084 (found here), TASK-074 (the claim corrected), TASK-083 and TASK-076 are linked above.

## Reopened 2026-08-03 — the enumeration was never removed from `show --help`

The stack section, the JSON surface, `DefaultRunnerName`, and `commands.md` are all
genuinely delivered. One criterion is not: **"No hand-written enumeration of show's
sections is left."**

`internal/cli/show.go` still hand-lists them, in both help strings:

```
:16  Short: "Show registered configuration summary (stack entries, plans, commands)"
:18  Long:  "One section per declared area — stack entries and the runners each declares,
:19         plans, modes (--mode), environments (--env), interaction commands, provision
:20         profiles, health checks, subprojects — and areas the config does not declare
:21         are omitted."
```

`git log -L14,24:internal/cli/show.go` shows commit `f2c6a76` **added** "stack entries …
plans" to the pre-existing list. The Resolution says both stale enumerations "were changed
to stop enumerating rather than to add one item"; for `show.go` the opposite happened.

The list is already stale by three. `showText` renders ten sections; the help text names
seven of them and omits `Compose:` (`show.go:142`), `Sites:` (`:224`) and
`Applications:` (`:247`).

`skills/dva/references/commands.md:174-177` (commit `f156a8f`) *is* correct and is the
model to copy: it describes the command rather than listing its sections.

**To close:** rewrite `Long` (and `Short`) to describe rather than enumerate. The criterion
was a `human —` binding, which is how a false claim passed; it now carries a shell binding
that fails while either enumeration is present.

Not a gap, for the record: the "Left open" note about `dva stack up <typo>` exiting 0 is
tracked by TASK-087 and TASK-098.
