---
id: TASK-107
title: "`Did you mean?` lists the same suggestions in a different order every run — 16 distinct orderings in 30 invocations"
type: fix
priority: P3
effort: S
status: done
created-at: 2026-07-31T11:45:00+09:00
completed-at: 2026-07-31T11:52:00+09:00
scope: "internal/cli/root.go:397-405 — suggestCommands ranges config.ReservedCommands(), which is a map[string]bool"
---

# Task 107: the suggestion list is shuffled by the map seed

## Problem

`suggestCommands` builds its result by ranging a map:

```go
for cmd := range config.ReservedCommands() {   // map[string]bool
	if levenshtein(input, cmd) <= 2 {
		suggestions = append(suggestions, cmd)
	}
}
```

Go randomizes map iteration, so the same typo produces the same *set* in a different *order* on
every invocation.

## Measured (bin/dva at 4ccedb9, `dva sta` in an empty directory)

| block | distinct orderings over 30 runs |
| --- | --- |
| cobra's own `Did you mean this?` | **1** |
| dva's `Did you mean?` (`suggestCommands`) | **16** |

Cobra's list is stable; dva's is not. Sample of consecutive runs, same binary, same directory:

```
Did you mean?      Did you mean?      Did you mean?
  dva stop           dva stack          dva ktl
  dva ssh            dva stop           dva ssh
  dva ktl            dva ssh            dva stop
  dva stack          dva ktl            dva stack
```

## Why it matters

The list is read top-down and the first entry carries the most weight, so shuffling it changes the
advice a user acts on — while looking like a settled answer. It also makes the output untestable by
comparison and undiffable in a bug report.

`SOUL.md` 신념 2 — "같은 설정과 입력은 같은 실행 순서를 만들어야 한다" — is the same clause that decided
[TASK-104](104-a-literal-key-that-spells-a-composite-key-deletes-one-command.md), and this
repo has already settled this defect class once before: `FormatConflictWarnings` had it, and
`TestFormatConflictWarningsIsStableAndCoversEveryConflict` in `internal/config/reserved_test.go`
exists precisely because "the same config produced a message about a different command from run to
run".

P3 rather than P2 because nothing executes differently — only the ordering of advice.

## Fix

Sort the result before returning. Alphabetical is the minimal change that closes the filed defect.

Ranking by edit distance (closest first) would arguably serve the reader better, but it is a
different feature, and for the measured input every candidate ties at the same distance anyway, so
it would degenerate to alphabetical here. Noted, not done — see Left open.

## Acceptance criteria

- [x] The order is stable | verify: `dva sta` 30× in an empty directory; print the count of distinct `Did you mean?` blocks — must be 1
- [x] The set is unchanged | verify: print the suggestions before and after; the same names must appear, only reordered
- [x] Not vacuous | verify: revert the sort and confirm the test fails; print the failure
- [x] Full suite passes | verify: `make test`

## Resolution

One `sort.Strings(suggestions)` in `internal/cli/root.go`, plus three tests in
`internal/cli/suggest_commands_test.go`.

### Measured, against a baseline binary built from `7b1669e`

| | fixed | baseline |
| --- | --- | --- |
| distinct `Did you mean?` blocks over 30 runs of `dva sta` | **1** | **18** |
| the names offered | `ktl ssh stack stop` | `ktl ssh stack stop` |

The set is identical; only the order changed. The block now reads:

```
Did you mean?
  dva ktl
  dva ssh
  dva stack
  dva stop
```

### Non-vacuity

Reverting `sort.Strings` in a copy of the tree fails both assertions:

```
--- FAIL: TestSuggestCommandsIsStable
    run 0 returned ["stop" "ssh" "ktl" "stack"]; the initial call returned ["stop" "ktl" "stack" "ssh"]
--- FAIL: TestSuggestCommandsKeepsTheWholeSet
    suggestCommands("sta") = ["ktl" "stack" "stop" "ssh"], want ["ktl" "ssh" "stack" "stop"]
```

The first attempt at this probe deleted the call but not the now-orphaned `sort` import, so the
package failed to **build**. A build failure is not an assertion failure and proves nothing about
the test; the probe was redone with a compiling revert.

### What did not catch it

`TestSuggestCommands_KnownSimilar`, `_NoMatch` and `_ExactMatch` already existed and **all three
pass under the unsorted implementation** — they assert membership, not order. That is why the defect
survived: the function was tested, just not for the property that was broken.

### Left open

Ranking by edit distance instead of alphabetically. For every input measured here the candidates tie
at the same distance, so it would produce the same list; it is a different feature and is not filed.

## Related

- [TASK-104](104-a-literal-key-that-spells-a-composite-key-deletes-one-command.md) — the same
  map-iteration defect class, in the command tree instead of the suggestion list.
- [TASK-108](../todo/108-two-did-you-mean-blocks-answer-one-error-differently.md) — found in the same
  measurement: this list is printed directly below cobra's, and the two disagree.
