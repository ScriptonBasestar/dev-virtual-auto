---
id: TASK-140
title: "An interaction step marked parallel runs sequentially, and validate calls the config valid"
type: bug
priority: P2
effort: M
created-at: 2026-08-03T13:00:00+09:00
source: "TASK-085 finalize verification — the sixth ProvisionItem key, same shape as the five 085 fixed"
depends-on: [TASK-085]
scope: "dva repo — internal/runner/steps.go, internal/config/{config,validate_warnings,schema.json}"
status: done
quality-review: pass
quality-reviewed-at: 2026-08-07T18:05:08+09:00
verified-at: 2026-08-07T18:05:08+09:00
archived-at: 2026-08-07T18:05:08+09:00
quality-review-evidence: |
  - kind: test
    command-or-step: make test && make doc-check (mise go 1.26.4)
    result: exit 0; shared suite green
  - kind: recheck
    command-or-step: acceptance criteria re-observed
    result: parallel provision-only; IgnoredParallel tests PASS
verification-summary: |
  quality-review pass; re-checked deliverables. parallel provision-only; IgnoredParallel tests PASS. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 140: Honour `parallel:` on the interaction path, or refuse it

## Problem

`parallel: true` on an interaction step does nothing. The key parses, validates, and is
then dropped on the floor — the same silent-discard shape TASK-085 fixed for `compose_up`,
`compose_exec`, `compose_run`, `cmd:` and `echo:`.

`Parallel` appears **zero** times in non-test code under `internal/runner/`
(`grep -rn Parallel internal/runner --include='*.go' | grep -v _test.go` → 0), so
`runStepLoop` (`internal/runner/steps.go:20`) has no batching at all. The provision path
does have it: `internal/cli/provision.go:54` `groupParallelBatches(steps)` and `:67`
`executeParallelBatch(...)`.

Measured 2026-08-03 with `bin/dva` v0.1.44, two `sleep 1` steps both marked
`parallel: true`:

```
$ dva run par             # interaction path
  → a
  → b
real 2.02                 # sequential

$ dva provision default   # provision path, same two steps
✅ Provision complete!
real 1.01                 # concurrent
```

Nothing warns. On a config whose only content is that interaction:

```
$ dva validate
✅ dva.yml is valid
rc=0
```

…while `internal/config/schema.json:330-333` actively advertises the key:

```json
"parallel": {
  "type": "boolean",
  "description": "Run this step concurrently with other consecutive parallel steps",
  "default": false
}
```

So the schema promises concurrency, `validate` confirms the file is good, and the runner
takes twice as long as advertised. A user timing a slow interaction has no way to learn
the key was ignored.

## Why this survived TASK-085

TASK-085 enumerated the keys an interaction step drops and fixed five of them by adding
`hasStepKeys`/`runComposeStepKeys` (`internal/runner/step_keys.go:29,42`). `parallel:` is
not a *payload* key — it does not make a step do work, it changes how steps are
scheduled — so it fell outside the guard `len(cmds) == 0 && !hasStepKeys(step)`
(`internal/runner/steps.go:52`) that closes the other five. A sweep of `tasks/todo/`,
`tasks/blocked/` and `tasks/plan/` on 2026-08-03 found 0 files mentioning `parallel`.

## Acceptance criteria

- [x] Pick a direction and record why: (A) implement batching in `runStepLoop`, reusing
      `groupParallelBatches`/`executeParallelBatch` rather than writing a third scheduler,
      or (B) reject the key on the interaction path so `validate` fails loudly.
- [—] Under A: the fixture above measures ~1s, not ~2s, for `dva run par` — state the
      measured wall-clock for both paths side by side.
      **Not applicable — A was not chosen** (import cycle; see Decision). Both wall-clocks
      are stated anyway in Result: `dva run par` 2.03s, `dva provision default` 1.02s.
- [~] Under B: `dva validate` exits non-zero on an interaction step carrying `parallel:`,
      and `schema.json`'s description says where the key is honoured.
      **Deviated on the first half, met on the second.** validate *warns* and stays rc 0,
      following the `validate_warnings.go:220` precedent quoted in the Decision; erroring
      would break configs that validate today over a key that has always been inert. The
      loudness the criterion wanted is delivered by the runtime notice instead.
- [—] Whichever direction: output interleaving is handled the way `executeParallelBatch`
      handles it (per-step `bytes.Buffer`), so concurrent steps cannot shred each other's
      lines — TASK-086 already paid for that lesson.
      **Not applicable, and the premise is false.** Nothing here runs concurrently, so
      there is no interleaving to handle. `executeParallelBatch` does not in fact handle
      it either — measured in the Decision — which is filed as TASK-168.
- [x] A test fails without the change, and its `-run` pattern is proven to match a real
      test name (an unanchored pattern matching zero tests still exits 0).
- [x] `make test` exits 0.

## Notes

Check the remaining `ProvisionItem` fields against what `runStepLoop` consumes before
fixing this one. This sweep found `parallel` by timing one path, not by diffing the struct
against the runner — a field-by-field comparison is the cheap way to learn whether it is
the last one.

## Decision: neither A nor B as written — warn at validate, and say so at runtime

Criterion 1 offers (A) implement batching or (B) reject the key. Measuring both before
choosing ruled out A and softened B.

**A is not buildable as specified.** The criterion says to reuse
`groupParallelBatches`/`executeParallelBatch` "rather than writing a third scheduler". Both
live in `internal/cli/provision.go` (`:91` and `:237`), while `runStepLoop` is in
`internal/runner`. Reusing them therefore requires `internal/runner` to import
`internal/cli` — and `internal/cli` already imports `internal/runner`, which closes the
cycle:

```
$ go list -f '{{range .Imports}}{{.}}{{"\n"}}{{end}}' ./internal/cli | grep internal/runner
github.com/ScriptonBasestar/dva/internal/runner
$ go list -f '{{range .Imports}}{{.}}{{"\n"}}{{end}}' ./internal/runner | grep internal/cli
(nothing — which is the edge the reuse would have to add)
```

To be read carefully: the second command returning nothing is not evidence against the
cycle, it is the precondition for it. Today's graph is acyclic *because* that edge is
absent; A's whole proposal is to add it. So A reduces to *writing the third scheduler*,
which is the thing the criterion was written to avoid.

**Criterion 4's premise is false, which removes the rest of A's case.** It says output
interleaving should be handled "the way `executeParallelBatch` handles it (per-step
`bytes.Buffer`) — TASK-086 already paid for that lesson." `executeParallelBatch` does not
handle it. The buffer captures only the lines dva writes; the commands run through
`internal/exec`, which hardcodes `c.Stdout = os.Stdout` (`internal/exec/exec.go:75`), so
children write straight past it. Measured, two parallel provision steps emitting five lines
each:

```
  ⚡ Running 2 steps in parallel...
BETA-1 / ALPHA-1 / ALPHA-2 / BETA-2 / BETA-3 / ALPHA-3 / ALPHA-4 / BETA-4 / ALPHA-5 / BETA-5
  [1/2] alpha          ← the labels, after their own output
  [2/2] beta
```

TASK-086 fixed note *rendering* on that function, not stream ownership, so it never covered
this. Copying the model onto the interaction path would have propagated the defect to a
second place. Filed as [TASK-168](../todo/168-parallel-provision-steps-print-their-labels-after-the-output-they-describe.md).

**B, but a warning rather than an error.** Criterion 3 asks for `validate` to exit non-zero.
The repo already answered this question for the identical shape at
`internal/config/validate_warnings.go:220`:

> A warning rather than an error, deliberately… rejecting it would turn a config that
> validates today into one that fails, and the item has been quietly doing nothing since
> long before this check existed.

`parallel:` on an interaction step is exactly that: schema-valid since it was introduced,
inert the whole time. Erroring would break working configs at upgrade over a key whose
removal changes no behaviour. The full precedent there is warning **plus** a runtime notice,
and both halves are needed here for a reason specific to this key — an inert step betrays
itself by doing nothing, but an ignored `parallel:` produces byte-identical output to the
concurrent run the author expected and merely takes twice as long. `validate` alone would
miss every author who never runs it; the notice alone would miss every author reading the
config rather than running it.

The deviation is deliberate and recorded here rather than silently satisfying criterion 3.

## Result

`parallel:` is now reported on both surfaces and still ignored, which is the honest state.

| | before | after |
|---|---|---|
| `dva validate`, interaction step with `parallel:` | `✅ dva.yml is valid`, rc 0, silent | 3 warnings naming each path, rc 0 |
| `dva run par` | two `sleep 1` steps, 2.02s, no mention | same 2.03s, one notice before the steps |
| `dva provision default` | 1.02s, concurrent | 1.02s, 0 notices — unchanged |

The warning walks `.steps`, `.before`, `.replace` and `.after`, deliberately **not**
`provision:`, and reaches nested hooks — the fixture's deepest hit is
`interaction.db.subcommands.migrate.before[0] "backup"`. Sorted, because `Interaction` is a
map and unsorted output makes `validate` undiffable between runs.

That deepest hit is a poor advertisement for this change, and the reason is worth recording.
The nested hook it names **never runs at all** — hooks fire only via `wrapWithHooks` on the
seven hookable built-ins, and nothing walks `Subcommands` looking for them. Measured: `dva db
migrate` on that fixture prints `MIGRATING` and not `BACKUP-RAN`. So the traversal is
correct and the location is dead code, which means this change warns that a key inside an
unreachable block is ignored while nothing warns about the block. Declared one level up as
`interaction.migrate.before` the same hook is a hard validation error, so the guard exists —
it just does not recurse. Filed as
[TASK-169](../todo/169-hooks-on-a-nested-subcommand-are-accepted-and-never-run.md), where it
belongs; the depth claim here stands, the implied usefulness of that particular hit does not.

`IgnoredParallelMessage` is one const in `internal/config/config.go`, shared by the warning
and the runtime notice, and it names where the key *does* work:

> `'parallel:' is ignored here — interaction steps always run sequentially. It is honoured under 'provision:'.`

That last sentence is load-bearing and has its own test. An author who reads "ignored" and
nothing else concludes the key is dead and deletes it from their provision profiles too,
where it is doing real work.

The runtime notice is printed once per list, not once per marked step: the key describes how
the list is scheduled, so a per-step notice would grow with the config while repeating one
fact, and would bury the step labels it sits among.

There are **two** executors of a `[]ProvisionItem`, not one, and the first cut of this change
only found the first. `steps:` runs through `runner.runStepLoop`; `before:`/`replace:`/
`after:` run through `cli.runHookSteps`, a separate loop in a package `internal/runner`
cannot import. So `dva up` with a parallel-marked before-hook took twice as long in silence
while `validate` warned about it — which inverts this change's own argument, since the
runtime half exists precisely for authors who never run `validate`. Measured before the fix:
`dva build`, two `sleep 1` before-hooks, 2.04s, 2 validate warnings, **0** runtime notices.
After: same 2.04s, 2 warnings, 1 notice.

Both call sites now go through `config.StepsIgnoreParallel(steps)` rather than each carrying
its own `for … if step.Parallel` — `runStepLoop`'s header comment records five defects
(TASK-083, 085, 089, 091, 094) caused by exactly that kind of copied loop drifting, and a
further copy was not worth the three lines it saved. `internal/config` sits below both
executors, so both can reach one predicate.

`schema.json`'s description no longer promises unconditional concurrency: it now says the
key is honoured under `provision:` only, that interaction steps and hooks run sequentially,
and that `validate` warns when it appears there.

### The Notes sweep

The Notes section asked whether `parallel` was the last unconsumed `ProvisionItem` field.
It was. All ten fields now have at least one non-test consumer under `internal/runner`:

```
Cmd 49  ComposeExec 5  ComposeRun 5  ComposeUp 5  Echo 5
Note 4  Parallel 1  Raw 4  Run 2  Step 2
```

`Parallel` was the only zero before this change. A field-by-field diff is indeed cheaper
than timing each path, and it now reports nothing left to find.

### Tests

Six, all falsified against reverted code:

- `TestWarnIgnoredParallelSteps` — the fixture's provision entry is the load-bearing part:
  a check keyed off the field rather than off *where the field appears* would flag it and
  tell the author to delete a key that works.
- `TestWarnIgnoredParallelStepsIsSilentWhereTheKeyWorks` — the negative control.
- `TestIgnoredParallelMessageNamesWhereTheKeyWorks` — pins the "honoured under provision"
  clause.
- `TestIgnoredParallelIsAnnouncedAtRuntime` — all three runners, asserting the notice
  appears exactly once, the steps still run in declaration order (the notice must not
  become an excuse to skip work — that is TASK-085's silent-discard shape), and silence
  when no step asks for concurrency.

Reverting both halves fails the runtime test on every runner:

```
--- FAIL: TestIgnoredParallelIsAnnouncedAtRuntime/local/a_parallel_step_draws_the_notice
    no notice printed for a parallel step; got "  → a\n  → b\n"
--- FAIL: .../docker_compose/... --- FAIL: .../kubectl/...
```

A fifth test exists because that revert exposed a gap: `internal/config` stayed **green**
with the registration line deleted, since the three config tests call
`warnIgnoredParallelSteps` directly and nothing pinned `ValidateWarnings` to actually call
it. `dva validate` would have gone silent with no test failing.
`TestValidateWarningsReportsIgnoredParallel` goes through `ValidateWarnings` and fails on
that deletion:

```
--- FAIL: TestValidateWarningsReportsIgnoredParallel
    ValidateWarnings does not surface the ignored-parallel check:
```

A sixth covers the hook executor, in `internal/cli` where `runHookSteps` lives —
`TestHookStepsAnnounceIgnoredParallel`. It reads `os.Stderr` from a real `runHookSteps` call
rather than asserting on `StepsIgnoreParallel`, because the predicate is not what regressed;
the call to it is, and a predicate test stays green when the call is deleted. Same three
assertions as the runner test: notice present, exactly once for the list, and both hooks
still run in declaration order. On reverted code:

```
--- FAIL: TestHookStepsAnnounceIgnoredParallel/a_parallel_step_draws_the_notice
    ignored_parallel_hook_test.go:61: notice count = 0, want 1
```

The `-run 'IgnoredParallel'` pattern is proven non-vacuous: `-v` lists the matched test
names before any assertion runs.

`make test`, `go vet`, `gofmt -l` and `make doc-check` all exit 0.
