---
id: TASK-084
title: "`dva stack up` walks a different entry sequence each run, and the warning that would catch it is suppressed by the very state dva's advice steers you into"
type: fix
priority: P2
effort: M
status: done
created-at: 2026-07-30T00:00:00+09:00
scope: "internal/config — lifecycle_helpers.go, validate_warnings.go; internal/cli — show.go"
verified-at: 2026-08-03T12:30:00+09:00
archived-at: 2026-08-03T12:30:00+09:00
verification-summary: |
  Verified against the built binary (bin/dva) and current source, read-only; working tree untouched.
  Half 1: lessByOrderName (lifecycle_helpers.go:121) is the single comparator for all five listings;
  grep bindings return exactly the stated 2, and 1/0. show.go carries no local sort — stackViews
  delegates to SortedStack. 20 separate `dva stack status` processes on a 5-entry no-order fixture
  yield 1 distinct sequence (alpha,bravo,charlie,delta,echo-entry), and `dva stack up` x6 executes in
  that same order every run.
  Half 2: warnDuplicateStackOrder filters the tied group by entriesNamedByPlans rather than skipping
  on `len(c.Plans) > 0`. Measured through `dva validate`: state A warns without the plan clause;
  state C emits only the unchanged legacy-order warning; state D is fully silent (exit 0); state D′
  warns about exactly charlie, delta, echo-entry. Sweep of all 16 examples/*.yml: 0 emit it.
  Message no longer says "undefined" and no such wording survives in docs/ or internal/.
  All four verify-bound tests exist with substantive assertions and pass; config, cli and lifecycle
  packages green. No TODO/FIXME in the three touched files and no open follow-up task.
---

# Task 084: A startup sequence that is not the same twice

## Problem

As found; half 1 has since fixed it, and the measurement is kept because it is what makes half 2 a
real question. `SortedStack()` sorted on `Order` alone:

```go
sort.Slice(entries, func(i, j int) bool { return entries[i].Order < entries[j].Order })
```

`sort.Slice` is not stable and there is no tiebreaker, so entries sharing an `Order` come out in Go's
randomized map-iteration order. `NewOrchestrator` computes this slice once (`orchestrator.go:63`) and
`Up`, `Down`, `Stop`, `Restart` and `Status` all read it, so the sequence every stack operation walks
is unspecified whenever two entries share an order — including the default where none declares it.

Measured on 0.1.44, five `script` entries with no `order:`, 20 runs of `dva stack status` (read-only,
and the same `o.entries` slice `up` filters): **5 distinct sequences**, all rotations of
`alpha,bravo,charlie,delta,echo-entry`. Rotations rather than shuffles, because pdqsort on an
all-equal comparator preserves input order — so what leaks through is the map's randomized starting
bucket. A startup failure caused by ordering may therefore not reproduce on the next run. To
reproduce: three or more `plugin: script` entries with `script: {up: echo up-<name>}` and no
`order:`, then `for i in $(seq 1 20); do dva stack status; done`.

### The warning that should catch this is suppressed by the state dva recommends

Four configs, all `dva validate` exit 0:

| config | order warning emitted |
| --- | --- |
| A — 5 entries, no `order:`, no plans | `entries … have order 0 (default); set explicit order values` |
| B — A + explicit `stack.*.order` | `⚠ 'stack.*.order' detected — execution order should move to 'plans.*.entries[].order'` |
| C — B + a plan | same as B; `warnLegacyStackOrder` only tests `Order > 0` |
| D — A + a plan naming all five | **silent** |

Following A's advice lands in B, where dva calls that advice legacy and points at plans. Following
*that* lands in D, the one silent state — and D is where `dva stack up` rotates, because
`warnDuplicateStackOrder` suppressed itself once any plan existed. Its own test said why
(`validate_warnings_test.go:199`, "Plan order owns sequencing when plans exist") — false for this
command, whose help states it "does NOT consult plans or default_plan". So the advice funnel ended
at the one configuration where the hazard was real and unannounced; verified on D, silent at
validate with 4 distinct sequences in 12 runs.

### Related, and not the same problem

`dva up <plan>` never consults stack `order:` — `NewPlanOrchestrator`
(`internal/lifecycle/plan_orchestrator.go:17-28`) walks the plan's own entries in declaration order,
each carrying its own `order` and `runner` (`config.go:77-79`). That path is deterministic, which is
why half 2 is a real question rather than an obvious yes: two commands read `stack` for different
purposes and only one of them is unstable.

## Half 1: shipped

`lessByOrderName(a, b *LifecycleEntry)` now holds the rule once and all five call sites read it.
`SortedStack` gained the tiebreak it was the only one of the five to lack; `show.go`'s local copy,
which existed only to compensate, is deleted, so the listing and `dva stack up` cannot drift apart.

Measured through the binary, since a within-process loop cannot: Go's map seed is per-process, so 200
in-process calls may share one seed. 20 separate `dva stack status` runs on the no-order fixture:

| binary | runs | distinct sequences |
| --- | --- | --- |
| with the tiebreak | 20 | **1** |
| tiebreak reverted, rebuilt | 20 | **5** — rotations of the YAML declaration order |

The 1 means nothing without the 5: a stable sort and a lucky one look identical. State D measured
too — 20 runs, 1 sequence — so the hazard is gone there; whether its silence should stay is half 2.

### Three of the five call sites had no test at all

Measured with `-coverpkg=./internal/config/` across the whole suite, not per package:
`PrimaryKubectlConfig`, `ComposeEntries` and `KubectlEntries` were at **0%** — rewired with nothing
able to catch a transcription slip, and `PrimaryKubectlConfig`'s old form was an inlined min
(`e.Order < best.Order || (e.Order == best.Order && e.Name < best.Name)`), not a sort comparator, so
agreement needed case analysis on all three order relations rather than a glance.
`TestEntryListingsShareOneComparator` now covers them (0% → 100/100/88.9%); reverting the tiebreak
fires 26 assertions across 10 runs. **Collecting duplicated logic is not a safe refactor when the
copies are untested** — the copies looking alike is what makes it feel safe.

## Half 2: shipped — the suppression narrowed

Decided: `warnDuplicateStackOrder` narrows its exemption, rather than `dva stack up` learning to read
plans. The second would make the warning's premise true by contradicting the command's own help, and
is ambiguous when two plans name one entry at different orders — no "which plan wins" answer that
does not invent a rule.

The states, measured through `dva validate`, after the first attempt was corrected (below):

| state | now |
| --- | --- |
| A — no `order:`, no plans | warns, without the plan clause |
| C — explicit orders + plan | legacy-order warning, unchanged |
| D — no `order:` + a plan naming every tied entry | silent — the plan *is* the position |
| D′ — no `order:` + a plan naming some of them | warns, naming only the unnamed ones |

The message also had to change, which was not in the plan. It said execution order "is undefined" —
true when filed, false after half 1, the trap in fixing a hazard without re-reading what was written
about it. The concern is no longer that the sequence is unstable but that nobody chose it, so the
wording names the sequence instead of calling it unknowable.

That claim is factual, so it was executed rather than reasoned about: `dva stack up` on the
no-order fixture walks alpha, bravo, charlie, delta, echo — identical across 6 runs.

### The first attempt broke three shipped examples

"Stops exempting configs that declare a plan" was implemented literally — the check deleted — and
`dva validate` then told three of dva's own examples they had not chosen a sequence. They had:
`examples/applications.yml`'s `local-full` declares infra 10 → api 20 → worker 30 → web 40, the shape
`docs/40-declarative-stack-and-plans.md:263` prescribes, order living in the plan layer rather than
in `stack.*.order`. Found by sweeping all 16 examples; the suite stayed green.

Both extremes were too coarse. `len(c.Plans) > 0` hid entries no plan mentions; no check at all
warned about entries a plan positions. What is checkable is whether a plan **names** the entry —
being listed is a position, since a plan walks its entries in declaration order — so the tied group
is filtered rather than skipped. 0 of 16 examples warn; a fixture whose plan names 2 of 5 entries
warns about exactly the other 3.

## Non-goals

- Not changing `Down`'s relationship to `Up` order; whether teardown should reverse is separate.
- Not deprecating `stack.*.order` — `warnLegacyStackOrder` already advertises that direction; this
  task only stops the intermediate states from being silent or unstable.

## Acceptance criteria

- [x] Equal-order entries come out in a fixed sequence | verify: `go test ./internal/config/ -run TestSortedStackIsDeterministic`
- [x] The sequence is stable across processes, not just within one | verify: `human — 20 runs on the no-order fixture: 1 distinct sequence, against 5 from the same binary with the tiebreak reverted`
- [x] The tiebreak matches PrimaryComposeEntry's documented one | verify: `/usr/bin/grep -c 'alphabetically first Name' internal/config/lifecycle_helpers.go` — print the count, expect ≥1 (got 2)
- [x] One comparator, not five | verify: `/usr/bin/grep -c 'Order < ' internal/config/lifecycle_helpers.go internal/cli/show.go` — print both counts, expect 1 and 0
- [x] The listings that had no test now have one | verify: `go test ./internal/config/ -run TestEntryListingsShareOneComparator`
- [x] An entry no plan names is not silent | verify: `human — dva validate on a 5-entry fixture whose plan names alpha and bravo warns about "entries charlie, delta, echo-entry … and no plan names them" — those three only`
- [x] An entry a plan does name stays silent | verify: `human — 0 of 16 examples/*.yml emit it (3 did before the narrowing); state D's fixture is silent`
- [x] The mode-isolation exemption still holds | verify: `go test ./internal/config/ -run TestWarnDuplicateStackOrder`
- [x] The warning states the sequence rather than calling it undefined | verify: `human — dva stack up on the no-order fixture walked alpha, bravo, charlie, delta, echo, identical across 6 runs`
- [x] The plan clause appears only with plans | verify: `human — mutation: emit it unconditionally, and the no-plans assertion fails from the same fixture`
- [x] Not vacuous | verify: `human — reverting the tiebreak fails the determinism test and show's TestShowStackOrderIsStableAcrossRenders, the only proof the delegation is real; an always-empty plan-coverage set and reporting the whole group each fail too`
- [x] Full suite passes | verify: `make test`

## Related

- [TASK-081](../todo/081-config-discovery-is-split-across-show-and-status.md) — found here. Its
  stack section carried a local `(Order, Name)` tiebreak for this reason; half 1 removed it, keeping
  `TestShowStackOrderIsStableAcrossRenders` at the `show` layer because the rendered listing is what
  a reader compares against `dva stack up`.
- [TASK-067](../todo/067-version-field-rule-stated-three-incompatible-ways.md) — same class: one rule
  stated in incompatible ways, here two warnings that each undo the other's advice.
