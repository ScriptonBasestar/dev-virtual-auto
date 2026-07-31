---
id: TASK-108
title: "One unknown command prints two `Did you mean` blocks that disagree with each other"
type: fix
priority: P3
effort: S
status: todo
created-at: 2026-07-31T11:45:00+09:00
scope: "internal/cli/root.go:229-240 — dva prints its own suggestion block below the one cobra already printed"
---

# Task 108: two answers, one question, neither labelled

## Problem

A single unknown command produces two suggestion blocks. Measured verbatim, `dva sta` in an empty
directory at 4ccedb9:

```
ERROR: unknown command "sta" for "dva"

Did you mean this?
	stop
	ktl
	ssh
	stack
	status


Did you mean?
  dva stop
  dva ssh
  dva ktl
  dva stack
rc=1
```

The first block is cobra's built-in `SuggestionsMinimumDistance` output; the second is dva's own
`suggestCommands`. They differ in three ways at once:

| | cobra's block | dva's block |
| --- | --- | --- |
| header | `Did you mean this?` | `Did you mean?` |
| entries | 5 | 4 |
| includes `status` | yes | **no** — `levenshtein("sta","status")` is 3, over dva's ≤2 cutoff |
| format | bare name, tab-indented | `dva <name>`, two-space indented |
| order | stable | shuffled every run ([TASK-107](../done/107-command-suggestions-come-out-in-a-different-order-every-run.md)) |

So the tool answers its own question twice, with different content, and gives the reader nothing to
decide which answer to trust. Neither block says where it came from.

## Why it matters

`SOUL.md` 신념 3 — "하나의 동작에는 하나의 소유자만 둔다" — one owner per behaviour. Suggesting a
command is one behaviour with two owners here, and the disagreement over `status` is what that
duplication produces: the more useful suggestion appears only in the block dva did not write.

## Options

- **A — drop dva's block, tune cobra's.** Cobra already does this, stably, with a wider net. Set
  `SuggestionsMinimumDistance` on the root command and delete `suggestCommands` and its call site.
  Loses the `dva <name>` prefix, which is the one thing dva's block does better for a reader who
  needs a copy-pasteable command.
- **B — drop cobra's block, keep dva's.** `rootCmd.DisableSuggestions = true`, keep the richer
  formatting, widen the distance cutoff to match what cobra was finding. Keeps the copy-pasteable
  form; means owning a suggestion algorithm that cobra maintains for free.
- **C — keep both, make them agree.** Most work, least benefit; still two blocks for one error.

## Decision needed

A or B. They trade the same thing in opposite directions — cobra's maintained algorithm against
dva's more actionable formatting — and the choice is a product call about that error message, not a
correctness question.

Note that [TASK-107](../done/107-command-suggestions-come-out-in-a-different-order-every-run.md) fixes the
ordering of dva's block independently: it is worth fixing under either option, because under A the
block goes away and the fix costs nothing, and under B it is required.

## Acceptance criteria

- [ ] One block, not two | verify: `dva sta` in an empty directory; print the full stderr and count the `Did you mean` headers — must be 1
- [ ] Nothing useful is lost | verify: print the suggestions for `sta`, `stak`, `stat`, `hlep` before and after; note any name that stops being offered
- [ ] The remaining block is stable | verify: 30 runs, print the count of distinct blocks — must be 1
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-107](../done/107-command-suggestions-come-out-in-a-different-order-every-run.md) — the ordering half,
  found in the same measurement.
- [TASK-098](../done/098-stack-status-and-unknown-subcommand-exit-zero.md) — the previous defect in
  this same error path, which is why root.go:229 carries a comment about what `suggestCommands` can
  and cannot see.
