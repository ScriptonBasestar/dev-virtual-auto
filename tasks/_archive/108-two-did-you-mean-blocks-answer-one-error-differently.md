---
id: TASK-108
title: "One unknown command prints two `Did you mean` blocks that disagree with each other"
type: fix
priority: P3
effort: S
status: done
created-at: 2026-07-31T11:45:00+09:00
completed-at: 2026-07-31T13:20:00+09:00
scope: "internal/cli/root.go:229-240 — dva prints its own suggestion block below the one cobra already printed"
verified-at: 2026-08-03T13:55:00+09:00
archived-at: 2026-08-03T13:55:00+09:00
verification-summary: |
  suggestCommands and its call site are gone; no production caller remains (grep over *.go finds
  the name only in explanatory comments). levenshtein is retained and still live at
  internal/cli/stack.go:494 and internal/cli/provision.go:405 (task text cites the pre-drift
  lines 452/386), with root.go:389-391 documenting why it survived.
  Measured on ./bin/dva v0.1.44 in an empty dir: sta/stak/stat/versoin/compeltion each print
  exactly one `Did you mean this?` block, rc=1; hlep and nosuchthing print none. 30 runs of
  sta, versoin and stat each produced 1 distinct stderr.
  The one documented cost is real and pinned: reimplementing the deleted <=2-levenshtein scan
  over config.ReservedCommands shows `hlep -> [help]` before, nothing now, and
  TestCobraNeverSuggestsHelp (with its versoin->version control) fails loudly if cobra ever
  starts offering help. internal/cli: 600 RUN / 600 PASS / 0 FAIL under -v.
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
| order | stable | shuffled every run ([TASK-107](107-command-suggestions-come-out-in-a-different-order-every-run.md)) |

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

## Decision

**A** — keep cobra's block, delete dva's. Decided by the operator 2026-07-31.

## Acceptance criteria

- [x] One block, not two | verify: `dva sta` in an empty directory; print the full stderr and count the `Did you mean` headers — must be 1
- [x] Nothing useful is lost | verify: print the suggestions for `sta`, `stak`, `stat`, `hlep` before and after; note any name that stops being offered
- [x] The remaining block is stable | verify: 30 runs, print the count of distinct blocks — must be 1
- [x] Full suite passes | verify: `make test`

## Resolution

`suggestCommands` and its call site are gone. `levenshtein` stays — `stack.go:452` and
`provision.go:386` use it for near-miss stack entries and plan names, which cobra knows nothing
about. Net: 71 insertions, 99 deletions across three files.

**Blocks per error, measured against a before-binary:**

| input | before | after |
| --- | --- | --- |
| `sta` | 2 | **1** |
| `stak` | 2 | **1** |
| `stat` | 2 | **1** |
| `versoin` | 2 | **1** |
| `hlep` | 1 | **0** |
| `nosuchthing` | 0 | 0 |

**Names offered, before vs after** — the union is unchanged except on one input:

| input | before | after |
| --- | --- | --- |
| `sta` | ktl ssh stack status stop | ktl ssh stack status stop |
| `stak` | stack stop | stack stop |
| `stat` | stack status stop | stack status stop |
| `versoin` | version | version |
| `hlep` | **help** | **— nothing** |

Stability: 30 runs of `dva sta` and `dva versoin` each produced **1** distinct block. Exit codes
unchanged at 1.

## The cost, which the task file predicted backwards

This task argued for A on the grounds that cobra has "a wider net." That is true for `sta` and
`stat`, where cobra reaches `status` and dva could not. It is **false for `help`**, and the
before/after table is what surfaced it — criterion 2 exists for exactly this.

Cobra can never suggest `help`. From the pinned `cobra@v1.10.2/command.go`:

```go
func (c *Command) IsAvailableCommand() bool {
	if len(c.Deprecated) != 0 || c.Hidden { return false }
	if c.HasParent() && c.Parent().helpCommand == c { return false }   // <- here
	...
}
```

and `SuggestionsFor` only considers commands where `IsAvailableCommand()` holds. So the help command
is structurally excluded, by cobra's design, and `dva hlep` now prints an error with no suggestion at
all where it used to answer `dva help`.

This was checked rather than assumed: `dva compeltion` still gets a cobra block, so the gap is
specific to `help`, not to edit distance. `TestCobraNeverSuggestsHelp` pins it — with a `versoin →
version` control, so it cannot pass merely because nothing is ever suggested — and fails loudly if a
future cobra starts offering `help`, at which point root.go's comment needs updating.

**One name, on one class of typo, is the whole price.** If that turns out to matter, B is still
available and the block that was deleted is one commit away.

## What replaced TASK-107's test file

`suggest_commands_test.go` tested the deleted function, so it was rewritten against
`rootCmd.SuggestionsFor`. Two details make the direct call faithful to the CLI path, both learned by
reading the cobra source rather than guessing:

- `InitDefaultHelpCmd()` must be called, because cobra registers `help` and `completion` lazily
  inside `Execute()` — without it the command set under test is smaller than the real one.
- `SuggestionsFor` compares against `c.SuggestionsMinimumDistance` with **no default applied**; it is
  `findSuggestions`, the runtime caller, that substitutes 2 when the field is `<= 0`. A test that
  skipped this would measure a stricter cutoff than any user experiences.

`TestSuggestionsAreStable` keeps TASK-107's guarantee alive against the new owner: 200 calls, one
ordering. Cobra's order is registration order, not alphabetical — `stop ktl ssh stack status` — which
is stable but not sorted, so the assertion pins equality against the first call rather than against
a sorted list.

## Related

- [TASK-107](107-command-suggestions-come-out-in-a-different-order-every-run.md) — the ordering half,
  found in the same measurement.
- [TASK-098](../_archive/098-stack-status-and-unknown-subcommand-exit-zero.md) — the previous defect in
  this same error path, which is why root.go:229 carries a comment about what `suggestCommands` can
  and cannot see.
