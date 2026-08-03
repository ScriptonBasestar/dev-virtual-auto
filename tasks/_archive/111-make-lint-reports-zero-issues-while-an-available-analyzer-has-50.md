---
id: TASK-111
title: "`make lint` reports 0 issues while `modernize` — already shipped in the pinned golangci-lint — reports 50"
type: chore
priority: P4
effort: M
status: done
decision: "A — enable modernize for all code including tests, take the full diff"
created-at: 2026-07-31T12:45:00+09:00
closed-at: 2026-07-31T12:30:00+09:00
scope: ".golangci.yml — the `modernize` linter is available in the pinned golangci-lint 2.12.2 and disabled by default; nothing in the config enables it"
verified-at: 2026-08-03T14:30:00+09:00
archived-at: 2026-08-03T14:30:00+09:00
verification-summary: |
  Re-measured, not taken from the record. golangci-lint 2.12.2 runs 7 linters with
  modernize among them; modernize-only over ./... is a real `0 issues.` — proven live by a
  scratchpad control module that trips mapsloop+rangeint on the same binary.
  `make lint` exits 0 through all four stages (vet, gofmt-s over 240 files, golangci-lint,
  gopls -severity=hint).
  The rewrites are physically present: 17 `maps.Copy`, 15 `SplitSeq`/`FieldsSeq`, 17
  `slices.Contains`, 1 `slices.Backward`, 11 `strings.Cut*` across cmd/internal/tools.
  The two hand-fixed items survive: internal/config/merge.go:3-7 is a single merged import
  block, and internal/cli/root_flag_passthrough_test.go:266-274 is
  `return slices.Contains(strings.Fields(line), arg)` with the comment explaining why that one
  line was not delegated to `--fix`.
  Tests counted by marker, not exit code (TASK-144): 1242 PASS / 0 FAIL / 1 SKIP over 1243 RUN.
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

Run it and it reports **50 findings — 39 of them in non-test code**, across 13 categories
(this count did not survive re-measurement — see [The count was 47](#the-count-was-47-not-50)):

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

Two analyzers that are already installed disagreed with `0 issues.`: `modernize` at 50, and
`gofmt -s` at 9 files. The second half has since been closed —
[TASK-078](../_archive/078-nine-files-do-not-satisfy-gofmt-and-nothing-checks.md) formatted the nine
files in `621d55a` and added `make fmt-check` to the Makefile and to CI, so `make lint` now depends
on `fmt-check` and formatting is genuinely gated. **`modernize` at 50 is what remains.** A gate whose
coverage nobody has stated is a gate whose silence means nothing in particular, which is the same
shape as [TASK-109](109-the-task-link-check-has-been-red-for-22-links-since-the-repo-moved.md)
and [TASK-110](110-23-archive-links-point-into-gitignored-tmp-and-the-checker-cannot-tell.md) — a
check that passes for a reason other than the one assumed.

The `maps.Copy` cluster is the one with a concrete tie to recent work: two of the 16 are the
defensive-copy loops in `internal/config/reserved.go:40` and `:52`, the function whose map iteration
caused [TASK-107](../_archive/107-command-suggestions-come-out-in-a-different-order-every-run.md).

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

Which of A / B / C. **The sequencing question is settled**: gofmt first, modernize second, separate
commits — decided 2026-07-31, and TASK-078's half has landed, so a `modernize` diff can no longer be
confused with a formatting diff. Whichever option is chosen, it starts from a tree that is already
`gofmt -s` clean and gated.

## Acceptance criteria

- [x] The gate's coverage is stated, not implied | verify: `.golangci.yml` names `modernize` under `enable:` with the reason; no analyzer is now deliberately excluded, so there is no residual gap to report

  ⚠️ The second clause was false when written and is false now. `.golangci.yml`
  `settings.govet.disable:` names `fieldalignment` and `shadow` (`:34`, `:36`), and
  `exclusions.rules` holds two more. Analyzers *were* deliberately excluded; what was true is
  that `modernize` was no longer one of them. [TASK-126](../done/126-the-lint-gate-still-hides-analyzers-task-111-said-it-had-none-left.md)
  measured 8 findings behind that clause, [TASK-127](../done/127-the-record-that-closed-the-coverage-gap-had-two-of-its-own.md)
  found 6 more behind `exclusions.presets`, and [TASK-130](../done/130-the-lint-gate-is-a-strict-subset-of-what-an-editor-sees.md)
  closed the gopls divergence. The gap is closed; only this sentence went uncorrected.
- [x] No behaviour changed | verify: `make test` before and after; every rewritten user-facing path exercised against the real binary (see Resolution)
- [x] The number moves or is explained | verify: `golangci-lint run --default=none --enable=modernize ./...` — `0 issues.`
- [x] Full suite passes | verify: `make test`
- [x] The gate is green for the right reason | verify: `make lint` — `0 issues.` with modernize enabled, after fixing the 6 staticcheck findings the rewrites introduced

## Resolution

Option **A** — `modernize` enabled in `.golangci.yml` for all code, tests included. B was rejected
for the reason the task itself implies: answering "the gate is green because of what it does not
run" by adding one more thing it does not run reproduces the defect at a smaller size. The 8 test
findings were not worth buying a permanent stated exclusion.

### The count was 47, not 50

Re-measured on the exact tree the task was filed against (`b6c273f^`, via a throwaway
`git worktree`, so no branch was touched):

| | filed as | measured at `b6c273f^` | measured at HEAD |
| --- | --- | --- | --- |
| total | 50 | **47** | 47 |
| non-test | 39 | 39 | 39 |
| test | 11 | **8** | 8 |

The code did not change — the headline was a miscount. An earlier draft of this section claimed
"the task's own category table sums to 47"; it does not, it sums to 50, so the recount was replaced
with a per-analyzer measurement:

| analyzer | measured | filed as | |
| --- | --- | --- | --- |
| `mapsloop` | 16 | 16 | |
| `stringsseq` (SplitSeq) | 11 | 11 | |
| `slicescontains` | 5 | 5 | |
| `stringscut` (`strings.Index`) | 3 | 3 | |
| `stringsbuilder` | 3 | 3 | |
| `rangeint` | 3 | 3 | |
| `stringsseq` (FieldsSeq) | **2** | 3 | −1 |
| `stringscutprefix` | 1 | 1 | |
| `stringscut` (`strings.IndexByte`) | 1 | 1 | |
| `slicesbackward` | 1 | 1 | |
| `forvar` | 1 | 1 | |
| `stringscut` (`SplitN`) | **0** | 1 | −1 |
| `errorsastype` | **0** | 1 | −1 |
| **total** | **47** | 50 | −3 |

The three phantoms are named rather than summarised because one of them is not obviously a
transcription slip. **`errors.As` does exist in this tree** — `internal/lifecycle/compose_error_test.go:56`
calls it — and modernize still reports nothing for it under the pinned golangci-lint 2.12.2. So that
row is not "a call site that was miscounted"; it is a rewrite the pinned toolchain does not offer,
which suggests the 50 was measured with a different modernize than the one the repo actually gates
on. Recorded because "50 → 0" would have credited this task with three findings that never existed,
and because the gate's behaviour is a property of the pinned binary — the same lesson the task is
about, arriving through its own measurement.

### 46 fixed by the tool, 1 by hand

`golangci-lint run --default=none --enable=modernize --fix ./...` applied 46. It refused the 47th:

```
conflicting edits from modernize and modernize on internal/cli/root_flag_passthrough_test.go
```

Two of its own analyzers (`slicescontains` and `stringsseq`) both wanted `hasArg`'s loop, and
golangci-lint skipped the file rather than merge them. Worth recording is what one of the two
candidate edits it printed would have produced:

```go
 func hasArg(line, arg string) bool {
-	for _, f := range strings.Fields(line) {
-		if f == arg { return true }
-	}
-	return false
+	return false
 }
```

A predicate that always answers "no". Three things independently stopped it, and the middle one
was measured rather than assumed: golangci-lint declined to apply either edit; substituting
`return false` by hand **fails 4 of 4 subtests**, because the call site's `present` assertions are
positive (`if !hasArg(...)`); and the line was ultimately written by hand as
`return slices.Contains(strings.Fields(line), arg)`.

So this is not "the autofixer nearly broke the suite" — the suite was not vulnerable. It is the
reason one line of 47 was not delegated to a tool.

### Fixing one analyzer's findings created another's

With modernize's output in place, `make lint` went **red**: 6 new `staticcheck` QF1012 findings in
`stack.go` and `provision.go`. modernize's `stringsbuilder` rule had rewritten `msg += fmt.Sprintf(…)`
into `msg.WriteString(fmt.Sprintf(…))`, which staticcheck then flags in favour of
`fmt.Fprintf(&msg, …)`. Fixed by hand; the final form is shorter than either the original or
modernize's intermediate.

This is the sharpest illustration of the task's own thesis. Enabling one analyzer did not simply
subtract findings — it moved the tree into a state a *different, already-enabled* analyzer objected
to. A gate's output is a property of the gate's composition, not of the code alone.

### One thing no gate caught

modernize inserted its import into `internal/config/merge.go` as a **second import declaration**:

```go
import "maps"

import "fmt"
```

Legal Go, and `gofmt -s` does not merge separate import declarations — so
[TASK-078](../_archive/078-nine-files-do-not-satisfy-gofmt-and-nothing-checks.md)'s new `make fmt-check`
reports `213 files checked, 0 unformatted` over it. Merged by hand. Noted because it is a live
example of the same lesson one level down: the format gate is green here for a reason unrelated to
this file being well-formed.

### Verification

| check | result |
| --- | --- |
| `golangci-lint --enable=modernize` residue | `0 issues.` |
| `make lint` (vet + fmt-check + golangci-lint) | `0 issues.` |
| `make fmt-check` | `213 files checked, 0 unformatted` |
| `make build` + `dva version` | `0.1.44`, binary answers |
| `make test` | pass, `-race`, all packages |

Coverage on the rewritten packages is 53–66%, so a passing suite is not on its own evidence that
30 files of real rewrites are equivalent. Two further checks were run.

**Every hunk was read in the main session** against the specific hazard of the rule that produced
it. Three review agents were dispatched to duplicate this per package area and did not report before
the commit; nothing below rests on them. The hazards checked, and what each check found:

| rule | the hazard | result |
| --- | --- | --- |
| `mapsloop` (16) | destination nil, or the loop was *filtered* / transformed keys | all 16 unconditional; every destination provably non-nil at the call — `make(...)` immediately above, or `NewEnvironment` which seeds `Vars` before returning |
| `stringsseq` (13) | the slice was indexed, reused, or `len()`-ed after the loop | none were; `validate.go` keeps the name `lines` for what is now an `iter.Seq`, which is misleading but not wrong |
| `slicescontains` (5) | loop body had side effects, or comparison semantics differ | all were bare `==` early-return predicates |
| `stringscut` (4) | the not-found branch changes meaning | `Cut`'s `after` is exactly `s[i+len(sep):]`, and every site returns the same zero value when `ok` is false |
| `stringscutprefix` (1) | `HasPrefix`/`TrimPrefix` disagree on empty prefix | prefix is a non-empty const |
| `slicesbackward` (1) | body mutates the slice being ranged | `migrate.go` mutates `out`, never `edits` |
| `rangeint` (3) | loop variable used in the body | unused in all three, or the rewrite would not compile |
| `forvar` (1) | pre-1.22 per-iteration capture semantics | `go 1.26.4` directive; the `command := command` copy is dead |

Two cosmetic leftovers were accepted rather than hand-edited, to keep the diff attributable to the
tool: the `iter.Seq` still called `lines`, and a redundant `e := v` that `slices.Backward` left in
`migrate.go`.

**Every rewritten user-facing path exercised against the real binary**, because the `strings.Builder`
rewrites build error text that no unit test asserts on:

| path | rewrite | result |
| --- | --- | --- |
| `dva stack up --bogus` | `rejectUnknownFlags` Builder | full message + accepted-flag list |
| `dva stack up infraa` | `validateStackNames` Builder | message + `Did you mean? dva stack up infra` |
| `dva stack up zzzzzz` | same, no-suggestion branch | message, suggestion block correctly absent |
| `dva provision bootstrapp` | `resolveProvisionProfile` Builder | message + suggestion |
| `dva provision zzzzzz` | same, no-suggestion branch | message, no suggestion block |
| `dva provision bootstrap --dry-run` | `writeNote` `SplitSeq` | 3-line note, per-line indent preserved |

The precedence-critical hunks were reviewed in the main session rather than delegated: all eight
`maps.Copy` sites in `config.go` and `merge.go` are unconditional copies, each immediately preceded
by a nil-guard that allocates the destination, and `mergeRunnerConfig` still copies base before
other so `other` wins. No filter or key/value transform was absorbed into a `maps.Copy`.

## Related

- [TASK-078](../_archive/078-nine-files-do-not-satisfy-gofmt-and-nothing-checks.md) — the other half of the same gap, and the sequencing constraint.
- [TASK-112](112-check-generate-is-labelled-ci-and-ci-does-not-run-it.md) — the third instance of a
  gate that does not cover what its greenness implies.
- [TASK-107](../_archive/107-command-suggestions-come-out-in-a-different-order-every-run.md) — touched two
  of the 16 `maps.Copy` sites while fixing the map-iteration defect in that same function.
