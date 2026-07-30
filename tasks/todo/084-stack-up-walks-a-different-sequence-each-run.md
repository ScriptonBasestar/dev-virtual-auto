---
id: TASK-084
title: "`dva stack up` walks a different entry sequence each run, and the warning that would catch it is suppressed by the very state dva's advice steers you into"
type: fix
priority: P2
effort: M
status: todo
created-at: 2026-07-30T00:00:00+09:00
scope: "internal/config — lifecycle_helpers.go SortedStack, validate_warnings.go warnDuplicateStackOrder; internal/lifecycle — orchestrator.go"
---

# Task 084: A startup sequence that is not the same twice

## Problem

`SortedStack()` (`internal/config/lifecycle_helpers.go:98`) sorts on `Order` alone:

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

## Proposed fix

Two halves; the first needs no decision, the second does.

1. **Make the sequence deterministic.** Tiebreak `SortedStack()` by `Name` after `Order`. This is
   already the documented convention one function away — `PrimaryComposeEntry` says
   "alphabetically first Name when Order values are equal" — so this makes the file
   self-consistent rather than inventing a rule. Arbitrary-but-stable is strictly better than
   arbitrary-and-varying: it makes ordering bugs reproducible.

   Collect the copies while doing it. The `(Order, Name)` comparator is written out five times:
   `lifecycle_helpers.go:113`, `:172`, `:190-195`, `:207-212`, and the local one TASK-081 added at
   `internal/cli/show.go`. Four of the five live in the file being changed, and the fifth exists
   only because `SortedStack` lacks the tiebreak — so this half should end with one copy, and
   `show.go`'s sort deleted.
2. **Decide what `dva stack up` owes plan order.** Either it consults the plan (contradicting its
   own help, and ambiguous when several plans name an entry), or the suppression in
   `warnDuplicateStackOrder` narrows to the commands plans actually govern, so state D warns
   again. The second is smaller and matches what the code already believes about plans.

### Related, and not the same problem

`dva up <plan>` never consults stack `order:` at all — `NewPlanOrchestrator`
(`internal/lifecycle/plan_orchestrator.go:17-28`) walks the plan's own entries in declaration
order, each `PlanEntry` carrying its own `order` and `runner` (`config.go:77-79`), and
`SiteEntryOverride.Runner` (`config.go:93`) can replace the runner again. That path is
deterministic; it is only worth stating here because it is why half 2 is a real question rather
than an obvious yes. Two commands read `stack` for different purposes, and only one of them is
unstable.

## Non-goals

- Not changing `Down`'s relationship to `Up` order; whether teardown should reverse the sequence
  is a separate question this task does not open.
- Not deprecating `stack.*.order`. `warnLegacyStackOrder` already advertises that direction; this
  task only stops the intermediate states from being silent or unstable.

## Acceptance criteria

- [ ] Equal-order entries come out in a fixed sequence | verify: `go test ./internal/config/ -run TestSortedStackIsDeterministic`
- [ ] The sequence is stable across processes, not just within one | verify: `human — 20 runs of 'dva stack status' on the 5-entry no-order fixture; print the count of distinct sequences, expect 1`
- [ ] The tiebreak matches PrimaryComposeEntry's documented one | verify: `/usr/bin/grep -c 'alphabetically first Name' internal/config/lifecycle_helpers.go` — print the count, expect ≥1
- [ ] State D is no longer silent, or `stack up` honours plan order | verify: `human — state which half shipped; run validate on fixture D and paste the outcome`
- [ ] The suppression still holds where plans really do govern | verify: `go test ./internal/config/ -run TestWarnDuplicateStackOrder`
- [ ] Not vacuous | verify: `human — revert the tiebreak and confirm the determinism test fails`
- [ ] Full suite passes | verify: `make test`

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

- [TASK-081](../done/081-config-discovery-is-split-across-show-and-status.md) — found here. `show`'s new
  stack section sorts by `(Order, Name)` locally for exactly this reason; once half 1 ships it can
  delegate to `SortedStack()` and drop the local tiebreak.
- [TASK-067](../done/067-version-field-rule-stated-three-incompatible-ways.md) — same class: one
  rule stated in mutually incompatible ways, here as two warnings that each undo the other's advice.
