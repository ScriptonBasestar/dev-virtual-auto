---
id: TASK-111
title: "`make lint` reports 0 issues while `modernize` — already shipped in the pinned golangci-lint — reports 50"
type: chore
priority: P4
effort: M
status: todo
created-at: 2026-07-31T12:45:00+09:00
scope: ".golangci.yml — the `modernize` linter is available in the pinned golangci-lint 2.12.2 and disabled by default; nothing in the config enables it"
---

# Task 111: the gate is green because of what it does not run

## Problem

`make lint` reports `0 issues.` Measured 2026-07-31 on go1.26.4 / golangci-lint 2.12.2:

| gate | result |
| --- | --- |
| `go vet ./...` | clean |
| `staticcheck ./...` run directly | 0 findings |
| `make lint` (golangci-lint) | **0 issues** |

`.golangci.yml` sets no `enable:` list, so the run is golangci-lint's default set — five linters:

```
errcheck  govet  ineffassign  staticcheck  unused
```

`modernize` ships in that same pinned binary and is **disabled by default**:

```
$ golangci-lint help linters | grep -A1 'Disabled by default'
modernize: A suite of analyzers that suggest simplifications to Go code, using
           modern language and library features.
```

Run it and it reports **50 findings — 39 of them in non-test code**, across 13 categories:

| count | finding |
| --- | --- |
| 16 | Replace `m[k]=v` loop with `maps.Copy` |
| 11 | Ranging over `SplitSeq` is more efficient |
| 5 | Loop can be simplified using `slices.Contains` |
| 3 | using `string += string` in a loop is inefficient |
| 3 | `strings.Index` can be simplified using `strings.Cut` |
| 3 | Ranging over `FieldsSeq` is more efficient |
| 3 | `for` loop can be modernized using range over int |
| 1 each | `SplitN`→`Cut`, `IndexByte`→`Cut`, `HasPrefix+TrimPrefix`→`CutPrefix`, `errors.As`→`AsType`, unneeded copy, `slices.Backward` |

## Why it matters

None of these are correctness bugs, and that is the point worth being precise about: **this is not a
claim that the code is broken.** It is a claim about what a green gate licenses a reader to believe.

Two analyzers that are already installed disagree with `0 issues.`: `modernize` at 50, and `gofmt -s`
at 9 files ([TASK-078](078-nine-files-do-not-satisfy-gofmt-and-nothing-checks.md), still open — re-confirmed at 9 today). A gate whose
coverage nobody has stated is a gate whose silence means nothing in particular, which is the same
shape as [TASK-109](../done/109-the-task-link-check-has-been-red-for-22-links-since-the-repo-moved.md)
and [TASK-110](110-23-archive-links-point-into-gitignored-tmp-and-the-checker-cannot-tell.md) — a
check that passes for a reason other than the one assumed.

The `maps.Copy` cluster is the one with a concrete tie to recent work: two of the 16 are the
defensive-copy loops in `internal/config/reserved.go:40` and `:52`, the function whose map iteration
caused [TASK-107](../done/107-command-suggestions-come-out-in-a-different-order-every-run.md).

## Options

- **A — enable `modernize` and take the 50-finding diff.** Honest gate, but a large mechanical change
  across many files, which `work-limits` (`최소한만`) argues against doing in one sweep, and which
  would collide with TASK-078's formatting diff if both land near each other.
- **B — enable it non-test only, fix the 39, exclude `_test.go` in the config.** Smaller blast
  radius; the 11 test findings are the least valuable ones.
- **C — leave it disabled and write the coverage down.** Record in `.golangci.yml` which analyzers
  are deliberately out and why, so `0 issues.` carries a stated scope. Fixes no code and is a
  legitimate answer if the team considers these style-only.

## Decision needed

Which of A / B / C, and — if A or B — whether it lands before or after
[TASK-078](078-nine-files-do-not-satisfy-gofmt-and-nothing-checks.md), since both produce wide mechanical diffs over overlapping files and
interleaving them would make either one hard to review.

## Acceptance criteria

- [ ] The gate's coverage is stated, not implied | verify: print the enabled linter list and, next to it, the finding count of every analyzer deliberately excluded
- [ ] No behaviour changed | verify: `make test` before and after; and for option A/B, confirm the diff contains no change outside the analyzer's own rewrites
- [ ] The number moves or is explained | verify: re-run `modernize ./...` and print the total — it must be 0, or the residue must be listed with a reason
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-078](078-nine-files-do-not-satisfy-gofmt-and-nothing-checks.md) — the other half of the same gap, and the sequencing constraint.
- [TASK-107](../done/107-command-suggestions-come-out-in-a-different-order-every-run.md) — touched two
  of the 16 `maps.Copy` sites while fixing the map-iteration defect in that same function.
