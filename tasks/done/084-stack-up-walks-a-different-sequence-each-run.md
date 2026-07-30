---
id: TASK-084
title: "`dva stack up` walks a different entry sequence each run, and the warning that would catch it is suppressed by the very state dva's advice steers you into"
type: fix
priority: P2
effort: M
status: done
created-at: 2026-07-30T00:00:00+09:00
scope: "internal/config — lifecycle_helpers.go SortedStack, validate_warnings.go warnDuplicateStackOrder; internal/lifecycle — orchestrator.go"
---

# Task 084: A startup sequence that is not the same twice

## Problem

As found — half 1 below has since fixed this half; the record is kept because the measurement is
what makes half 2 a real question. `SortedStack()` sorted on `Order` alone:

```go
sort.Slice(entries, func(i, j int) bool { return entries[i].Order < entries[j].Order })
```

`sort.Slice` is not stable and there is no tiebreaker, so entries sharing an `Order` come out in
Go's randomized map-iteration order. `NewOrchestrator` computes this slice once
(`orchestrator.go:63`) and `Up`, `Down`, `Stop`, `Restart` and `Status` all read it, so the
sequence every stack operation walks is unspecified whenever two entries share an order —
including the default case where no entry declares `order:` at all.

Measured on 0.1.44, five `script` entries with no `order:`, `dva stack status` (read-only, and the
same `o.entries` slice `up` filters), 20 runs:

| distinct sequences | shape |
| --- | --- |
| 5 | rotations of `alpha,bravo,charlie,delta,echo-entry` |

Rotations rather than shuffles, because pdqsort on an all-equal comparator preserves input order —
so what leaks through is the map's randomized starting bucket. A startup failure caused by
ordering therefore may not reproduce on the next run.

### The warning that should catch this is suppressed by the state dva recommends

Four configs, all `dva validate` exit 0:

| config | order warning emitted |
| --- | --- |
| A — 5 entries, no `order:`, no plans | `entries … have order 0 (default); set explicit order values to control startup sequence` |
| B — A + explicit `stack.*.order` | `⚠ 'stack.*.order' detected — execution order should move to 'plans.*.entries[].order'` |
| C — B + a plan | same as B; `warnLegacyStackOrder` only tests `Order > 0` |
| D — A + a plan naming all five | **silent** |

Following A's advice lands in B, where dva calls that advice legacy and points at plans. Following
*that* lands in D, the one silent state — and D is where `dva stack up` rotates, because
`warnDuplicateStackOrder` deliberately suppresses itself once plans exist. Its own test says why
(`validate_warnings_test.go:199`, "Plan order owns sequencing when plans exist"), and that premise
is false for this command: `dva stack up`'s help states it "does NOT consult plans or
default_plan". Plan order governs `dva up <plan>` only.

So the advice funnel ends at the one configuration where the hazard is real and unannounced.
Verified on D: silent at validate, 4 distinct sequences in 12 runs.

### Related, and not the same problem

`dva up <plan>` never consults stack `order:` at all — `NewPlanOrchestrator`
(`internal/lifecycle/plan_orchestrator.go:17-28`) walks the plan's own entries in declaration
order, each `PlanEntry` carrying its own `order` and `runner` (`config.go:77-79`), and
`SiteEntryOverride.Runner` (`config.go:93`) can replace the runner again. That path is
deterministic; it is only worth stating here because it is why half 2 is a real question rather
than an obvious yes. Two commands read `stack` for different purposes, and only one of them is
unstable.

## Half 1: shipped

`lessByOrderName(a, b *LifecycleEntry)` now holds the rule once, and all five call sites read it.
`SortedStack` gained the tiebreak it was the only one of the five to lack; `show.go`'s local copy —
which existed only to compensate for that — is deleted, so the listing and `dva stack up` cannot
drift apart.

Measured through the binary, since a within-process loop cannot see this: Go's map seed is
per-process, so 200 in-process calls may share one seed. 20 separate `dva stack status` runs on the
5-entry no-order fixture:

| binary | runs | distinct sequences |
| --- | --- | --- |
| with the tiebreak | 20 | **1** |
| tiebreak reverted, rebuilt | 20 | **5** — rotations of the YAML declaration order |

The 1 means nothing without the 5: a stable sort and a lucky one look identical. State D measured
too — 20 runs, 1 sequence — so the hazard is gone there while the silence remains, which is half 2.

### Three of the five call sites had no test at all

Measured with `-coverpkg=./internal/config/` across the whole suite, not per package:
`PrimaryKubectlConfig`, `ComposeEntries` and `KubectlEntries` were at **0%**. They were rewired with
nothing able to catch a transcription slip — and `PrimaryKubectlConfig`'s old form was an inlined
min (`e.Order < best.Order || (e.Order == best.Order && e.Name < best.Name)`), not a sort
comparator, so agreement needed case analysis on all three order relations rather than a glance.
`TestEntryListingsShareOneComparator` now covers them (0% → 100/100/88.9%); reverting the tiebreak
fires 26 assertions across 10 runs. **Collecting duplicated logic is not a safe refactor when the
copies are untested** — the copies looking alike is what makes it feel safe.

## Half 2: shipped — the suppression narrowed

Decided: `warnDuplicateStackOrder` stops exempting configs that declare a plan, rather than
`dva stack up` learning to read plans. The second would have made the warning's premise true by
contradicting the command's own help, and is ambiguous when two plans name one entry at different
orders — there is no answer to "which plan wins" that does not invent a rule.

The four states are now coherent, measured through `dva validate`:

| state | before | after |
| --- | --- | --- |
| A — no `order:`, no plans | warns | warns, without the plan clause |
| C — explicit orders + plan | legacy-order warning | unchanged |
| D — no `order:` + plan | **silent** | warns, naming what plan order does and does not govern |

The message also had to change, which was not in the original plan. It said execution order "is
undefined" — true when filed, false after half 1, which is the trap in fixing a hazard without
re-reading what was written about it. The remaining concern is not that the sequence is unstable
but that nobody chose it, so the wording names the sequence instead of calling it unknowable.

That claim is factual, so it was executed rather than reasoned about: `dva stack up` on the
no-order fixture walks alpha, bravo, charlie, delta, echo — identical across 6 runs.

## Non-goals

- Not changing `Down`'s relationship to `Up` order; whether teardown should reverse the sequence
  is a separate question this task does not open.
- Not deprecating `stack.*.order`. `warnLegacyStackOrder` already advertises that direction; this
  task only stops the intermediate states from being silent or unstable.

## Acceptance criteria

- [x] Equal-order entries come out in a fixed sequence | verify: `go test ./internal/config/ -run TestSortedStackIsDeterministic`
- [x] The sequence is stable across processes, not just within one | verify: `human — 20 runs of 'dva stack status' on the 5-entry no-order fixture: 1 distinct sequence, against 5 from the same binary with the tiebreak reverted`
- [x] The tiebreak matches PrimaryComposeEntry's documented one | verify: `/usr/bin/grep -c 'alphabetically first Name' internal/config/lifecycle_helpers.go` — print the count, expect ≥1 (got 2)
- [x] One comparator, not five | verify: `/usr/bin/grep -c 'Order < ' internal/config/lifecycle_helpers.go internal/cli/show.go` — print both counts, expect 1 and 0
- [x] The listings that had no test now have one | verify: `go test ./internal/config/ -run TestEntryListingsShareOneComparator`
- [x] State D is no longer silent | verify: `human — dva validate on fixture D now prints "entries alpha … are all at the default order, so 'dva stack up' starts them in name order rather than one you chose; plan entry order governs 'dva up <plan>' only"`
- [x] The mode-isolation exemption still holds | verify: `go test ./internal/config/ -run TestWarnDuplicateStackOrder`
- [x] The warning states the sequence rather than calling it undefined | verify: `human — dva stack up on the no-order fixture walked alpha, bravo, charlie, delta, echo, identical across 6 runs`
- [x] The plan clause appears only with plans | verify: `human — mutation: emit it unconditionally, and the no-plans assertion fails from the same fixture`
- [x] Not vacuous | verify: `human — revert the tiebreak: the determinism test fails, and so does show's TestShowStackOrderIsStableAcrossRenders, which is the only proof the delegation is real`
- [x] Full suite passes | verify: `make test`

## Reproduction fixture

```yaml
version: "0.1.44"
stack:
  alpha:
    plugin: script
    script: {up: echo up-alpha}
  bravo:
    plugin: script
    script: {up: echo up-bravo}
  charlie:
    plugin: script
    script: {up: echo up-charlie}
# add a `plans:` section naming all three to reach silent state D
```

`for i in $(seq 1 20); do dva stack status; done` — the entry order rotates.

## Related

- [TASK-081](../done/081-config-discovery-is-split-across-show-and-status.md) — found here. Its
  stack section carried a local `(Order, Name)` tiebreak for exactly this reason; half 1 removed it,
  and `TestShowStackOrderIsStableAcrossRenders` was kept at the `show` layer because what a reader
  compares against `dva stack up` is the rendered listing.
- [TASK-067](../done/067-version-field-rule-stated-three-incompatible-ways.md) — same class: one
  rule stated in mutually incompatible ways, here as two warnings that each undo the other's advice.
