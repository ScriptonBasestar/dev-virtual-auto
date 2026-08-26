---
id: TASK-128
title: "The recursion was right; the nodes it walked were not"
type: fix
priority: P1
effort: M
status: done
resolved-at: 2026-08-02T00:00:00+09:00
resolution: "Threaded inherited execution context through eachInteractionNode so recursive checks ask their question of the merged node the runtime executes, not the raw YAML node; removed the false positive TASK-125 shipped on examples/full-stack.yml and the matching false negative on inherited runners"
created-at: 2026-08-02T00:00:00+09:00
scope: "internal/config/validate_warnings.go — eachInteractionNode:316, inheritedExec:342, hasExecutionTarget:363, warnChildOverridesParentCritical:733, warnUnreachableCommands:806, warnInertProvisionSteps:230; internal/runner/docker_compose.go — buildStepArgs doc comment"
verified-at: 2026-08-03T15:45:00+09:00
archived-at: 2026-08-03T15:45:00+09:00
verification-summary: |
  Rebuilt the pre-fix binary (ec4c0b0) with `go build -overlay` into scratchpad and ran both
  binaries over all 17 shipped configs. The claimed deltas reproduce exactly: 20→19 semantic
  warnings, 2→1 `directly callable`, and the per-config diff shows one line removed and zero
  added — so "no config gains a warning" is measured, not inferred.
  Built a fixture deeper and wider than the bug (5 interaction trees, nesting to depth 4-5).
  Inheritance holds at every depth: `deepinherit d1`/`d1 d2`/`d1 d2 d3` all run the inherited
  `echo root`, `d1 d2 d3 d4` runs `echo leaf`, and validate stays correctly silent. The
  all-group tree warns at all 4 nodes that have subcommands. Runner and pod overrides through
  two silent intermediates are caught (local→docker, alpha→beta) and the runtime confirms both
  warnings are true.
  All 5 mutations reproduced independently via `-overlay` and were killed by exactly the tests
  the task names. The two sort mutations pass with `-skip TestFlatMapWarningsAreOrderStable`
  (exit=0, 381 PASS, 0 FAIL), confirming the new test is load-bearing rather than decorative.
  TASK-125's record is genuinely corrected in place: the criterion is `[ ]` with the reason,
  not rewritten green. The "observed but not fixed" step-env asymmetry is owned by TASK-129
  (done), so it left no orphan.
---

# Task 128: a check that recurses must ask its question of the node that runs

[TASK-125](../_archive/125-three-interaction-warnings-stop-at-depth-1.md) made three interaction warnings
recurse. The recursion was correct. What it recursed *over* was not: each check kept asking its
question of the node as parsed from YAML, and below depth 1 that node is not what the runtime
executes.

`runner.mergeInteraction` (`internal/runner/interaction_tree.go:235`) copies **every** execution
field parent → child — Description, Service, Command, CommandLines, Script, ScriptFile, Steps,
Workdir, User, DefaultArgs, Environment, Shell, Entrypoint, Runner, Pod, Compose — and takes only
`Subcommands` from the child. A child that declares nothing still runs the parent's command under
the parent's runner.

At depth 1 there is no ancestor, so raw node and merged node coincide. That is exactly why the
three checks were correct for as long as they stopped at depth 1, and wrong the moment they did
not. Making them recurse without making them inheritance-aware converted a silence gap into a
wrong-answer gap.

## Measured

Real binary, `bin/dva config validate`, over the 17 configs the repo ships:

| | warnings | `directly callable` |
|---|---|---|
| before (`c50cc93`) | 20 | 2 |
| after | 19 | 1 |

The one that disappeared is `examples/full-stack.yml`:

```
interaction.rails.subcommands.db: has subcommands but is not directly callable;
  add an execution target or remove subcommands
```

`dva rails db` runs. Proof from the same binary, not a reading of the YAML:

```
$ (cd fs/ && dva run rails db --dry-run)
Command: bundle exec rails          # inherited from interaction.rails
Runner: DockerCompose
Service: web
```

The one that survives is real, and the contrast is the whole task:

```
$ (cd applications/ && dva run db --dry-run)
Command:                            # empty — nothing to run, at any depth
Description: Database management
```

Same warning text, opposite truth value, and the shipped check could not tell them apart.

## The same root cause, opposite sign

`warnChildOverridesParentCritical` compared `cmd.Runner` — the parent's *declared* runner. An
intermediate node that declares nothing has an empty `Runner`, so the comparison ran against `""`
and never fired. Two fixtures, byte-identical at runtime:

```yaml
acase:                    bcase:
  runner: local             runner: local
  command: bundle exec …    command: bundle exec …
  subcommands:              subcommands:
    db:                       db:
      description: db group     description: db group
      subcommands:              runner: local          # restates what it inherits anyway
        migrate:                subcommands:
          runner: docker          migrate:
          command: db:migrate       runner: docker
                                    command: db:migrate
```

```
$ dva run acase db migrate --dry-run      $ dva run bcase db migrate --dry-run
Command: db:migrate                       Command: db:migrate
Runner: docker                            Runner: docker
```

Before: only `bcase` warned. After: both do, identically. Whether a user restates an inherited
value is a style choice; it was deciding whether they got told their backend changes.

So `warnUnreachableCommands` produced a false positive and `warnChildOverridesParentCritical` a
false negative from one shared cause — reading raw nodes. Fixing them separately would have
treated one root as two.

## Resolution

`eachInteractionNode` now passes each visitor an `inheritedExec` carrying what the node receives
from its ancestors under `mergeInteraction`'s rules:

- `callable` — does any ancestor supply something to execute
- `runner`, `pod` — the nearest ancestor's *effective* value, via `firstNonEmptyStr`

`warnUnreachableCommands` fires only when the node has no target of its own **and** inherits none.
`warnChildOverridesParentCritical` compares against the inherited value. Only the fields these
checks consult are carried, and `inheritedExec`'s doc says so — a future check that depends on
another inherited field has to add it rather than silently get a zero value.

`hasExecutionTarget` is extracted so "what counts as runnable" has one definition instead of one
per check.

### Two smaller things folded in, and why

`warnInertProvisionSteps` dropped its private walker for the shared one. TASK-125 named the two
path conventions as a maintenance trap and deferred it because unifying them changes a
user-visible message. It changes it toward correct: `interaction.db.migrate.before[0]` →
`interaction.db.subcommands.migrate.before[0]`. The old form cannot be pasted into a config —
there is no `migrate` key under `db`. It lands here because this task already rests on the merged
view being the one that matters, and because the trap it warned about is what let the same
inheritance bug reach two conventions.

`warnHealthCheckRedundancy` and `warnDeepSubcommandNesting` gained the `sort.Strings` TASK-125
gave its three checks and not these two. 4 distinct orderings over 15 runs of the real binary
before, 1 after.

## Acceptance criteria

- [x] The false positive is gone | verify: `human — sweep the 17 shipped configs, count 'directly callable'` — **2 → 1; the remaining one is `examples/applications.yml` `interaction.db`, confirmed true by `dva run db --dry-run` printing an empty `Command:`**
- [x] It is gone because the node is callable, not because the check got quieter | verify: `human — dva run rails db --dry-run on examples/full-stack.yml` — **`Command: bundle exec rails`, `Runner: DockerCompose`, `Service: web` — all inherited**
- [x] Inherited runners are compared | verify: `go test ./internal/config/ -run ChildOverrideComparesInheritedRunner -v | /usr/bin/grep -c '^    --- PASS'` — **2 subtests: implicit inheritance and redundant restatement now produce the same warning**
- [x] A/B fixture proves the two shapes are runtime-identical | verify: `human — dva run acase db migrate --dry-run vs bcase` — **identical execution plans; both warn after, only bcase before**
- [x] No shipped config gains a warning | verify: `human — same 17-config sweep, total semantic warnings` — **20 → 19, a net −1 accounted for entirely by the removed false positive; 0 nested-path warnings, matching what TASK-125's criterion meant to measure**
- [x] Inert-step paths are pasteable | verify: `human — d/ fixture` — **`interaction.db.subcommands.migrate.before[0]`, was `interaction.db.migrate.before[0]`; 0 doc references to the old form**
- [x] Flat-map warnings are order-stable | verify: `go test ./internal/config/ -run FlatMapWarningsAreOrderStable -v | /usr/bin/grep -c '^    --- PASS'` — **2; plus 15 real binary runs on the `d` fixture, 1 distinct ordering (was 4)**
- [x] Every fix fails when reverted | verify: `human — 5 mutations` — **all 5 caught; see below**
- [x] Full suite passes | verify: `make test` — exit 0
- [x] Lint clean | verify: `make lint` — `0 issues.`
- [x] Docs consistent | verify: `make doc-check` — `broken_links: 0`
- [x] Generated artifacts current | verify: `make check-generate` — clean

## Mutation-tested

| Mutation | Caught by |
|---|---|
| `warnUnreachableCommands` ignores `inherited.callable` | `RecurseIntoNestedSubcommands` — 3 warnings not 2, and the explicit false-positive guard names `tools.subcommands.nested` |
| `warnChildOverridesParentCritical` compares `cmd.Runner`/`cmd.Pod` | `ChildOverrideComparesInheritedRunner/runner_inherited_implicitly` — 0 warnings, want 1 |
| Walker joins `path+"."+subName` | `WarnInertProvisionSteps`, `RecurseIntoNestedSubcommands`, `ChildOverrideComparesInheritedRunner` |
| `sort.Strings` dropped from `warnHealthCheckRedundancy` | `FlatMapWarningsAreOrderStable/warnHealthCheckRedundancy` — unsorted at run 0, diverges at run 1 |
| `sort.Strings` dropped from `warnDeepSubcommandNesting` | `FlatMapWarningsAreOrderStable/warnDeepSubcommandNesting` — same |

The last two are why `TestFlatMapWarningsAreOrderStable` exists as its own test. Neither check goes
through `eachInteractionNode`, so removing either sort left `InteractionWarningsAreOrderStable` —
and the entire package — green. Measured before writing it: both mutations passed `go test
./internal/config/` with no failures at all. Without the new test these two sorts would have
shipped guarded by nothing but a hand-run binary probe, which is the same class of gap this task
is about.

## How TASK-125's own criterion missed this

TASK-125 swept the shipped configs for new warnings and reported 0. The grep was
`'subcommands\..*\.subcommands\.'` — two segments. The regression it shipped was
`interaction.rails.subcommands.db`: one segment. The query structurally could not match the thing
it was written to catch.

That task quoted "20 semantic warnings emitted, 0 of them nested-path" specifically so the 0 would
read as a measurement rather than a sweep that never ran — following the standing rule to print a
non-zero count beside every verdict. The rule worked as designed and was not enough: it proves the
sweep ran, not that it asked the right question. The 20 was real; the 0 was the answer to a
question nobody needed answered.

TASK-125's record is corrected in place — the criterion is `[ ]` with the reason, not quietly
rewritten green.

## Observed but not fixed

`buildStepArgs` (`internal/runner/docker_compose.go`) carried a comment saying env "reaches the
child through execComposeStep". Steps always run through `docker compose exec`, and nothing injects
`-e` on that path — only `run` does, via `composeArguments → runVars`. So a step sees a different
environment than the same command run as a non-step. The comment is corrected to record the
asymmetry; the asymmetry itself predates this change and belongs to whoever fixes step env, not to
a task about validation warnings.

## Related

- [TASK-125](../_archive/125-three-interaction-warnings-stop-at-depth-1.md) — this fixes what that shipped, and
  the pair is the actual lesson: a check that changes *how far* it walks has also changed *what* it
  is looking at.
- [TASK-107](../_archive/107-command-suggestions-come-out-in-a-different-order-every-run.md) — the map-ordering
  defect, now closed on the last two surfaces in this file.
- [TASK-118](../_archive/118-a-health-check-that-never-passes-is-still-exit-0.md) — the recurring shape. TASK-125
  was a check whose silence was read as a pass; this one is a check whose noise was read as a
  finding. Same root: output trusted beyond what it measured.
- [TASK-165](../todo/165-a-leaf-interaction-with-nothing-to-run-draws-no-warning-and-exits-0.md) —
  the coverage boundary next door, found while verifying this one and **not** introduced by it:
  `hasExecutionTarget` (`:363`) is a byte-for-byte extraction of the pre-fix `isCallable`, so
  this task moved the predicate without changing which nodes it is applied to. A leaf with no
  target and no subcommands is still outside `warnUnreachableCommands` entirely.
