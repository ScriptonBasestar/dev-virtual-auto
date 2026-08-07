---
id: TASK-158
title: "Explain's text branch exits 0 on a failed write, and the comment says that cannot happen"
type: bug
priority: P3
effort: S
created-at: 2026-08-03T15:00:00+09:00
source: "TASK-121 finalize verification — the sibling branch, and the reason recorded for skipping it"
depends-on: [TASK-121]
scope: "dva repo — internal/runner/runner.go:77-84, :108-131"
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
    result: planWriter err on text Explain; PlanWriter tests PASS
verification-summary: |
  quality-review pass; re-checked deliverables. planWriter err on text Explain; PlanWriter tests PASS. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 158: Either report the text plan's write errors or record the real reason not to

## Problem

TASK-121 made `Explain`'s JSON branch return the write error. The text branch still drops it.
Measured 2026-08-03 on `bin/dva` v0.1.44, one fixture, stdout pointed at a read-only fd so every
write returns `EBADF`:

```
$ ( exec 3</dev/null 1>&3; dva run hello --explain --json )
ERROR: write /dev/stdout: bad file descriptor        → exit 1
$ ( exec 3</dev/null 1>&3; dva run hello --explain )
                                                     → exit 0, 0 bytes delivered
```

Exit 0 having delivered nothing is the failure this task's parent was filed to remove,
surviving on the branch beside it.

## The comment is the part that is actually wrong

Scoping the text branch out is a defensible call — it means threading errors through ten bare
`fmt.Print*` calls (`runner.go:108-131`) for a human-facing path. But `runner.go:77-84` records a
different reason, and it does not hold:

> that branch is human-facing, so a closed downstream pipe already kills the process via SIGPIPE
> — a silent success needs the write to succeed-and-be-lost, which a tty or a regular file does
> not produce.

A silent success does not require the write to succeed. `EBADF` makes it *fail*; the error is
dropped; the command exits 0. The enumeration "a tty or a regular file" omits the failing-fd
case — which is the case the JSON branch's own test is built on. So the comment tells the next
reader that this exposure does not exist, using the same file's other branch as the
counter-example.

TASK-121's own file flags this as an open judgement call (lines 51-54); nothing in
`tasks/todo|blocked|decision|plan` tracked it.

## Acceptance criteria

- [x] Pick one and record why:
      (A) the text branch propagates its write errors, like the JSON branch; or
      (B) it stays as-is and `runner.go:77-84` states the real reason — blast radius across ten
      print calls on a human-facing path — with no claim about which failure modes are reachable.
      **(A)** — see "The choice, and the argument that decided it" below.
      Verify: `human — the decision and its reasoning are in the Result section`
- [x] Under either, the reproduction above appears in the Resolution with both exit codes, so
      the exposure is written down rather than described.
      Verify: `human — before/after table in the Result section`
- [x] Under A: a test covers the text branch with a failing writer, mirroring the JSON one.
      Verify: `go test ./internal/runner/ -run 'Explain|PlanWriter' -count=1`
- [x] Under B: a test or comment records that exit 0 with zero bytes is a known outcome, so it is
      not rediscovered as a bug.
      **Not applicable** — this criterion is conditional on (B), and (A) was taken. Exit 0 with
      zero bytes is no longer an outcome, so there is nothing to record as known.
- [x] Say whether any other `fmt.Print*`-only path has the same shape, with a count.
      Verify: `human — the survey and its counts are in the Result section`
- [x] `make test` exits 0.
      Verify: `make test`

## Notes

Distinct from [TASK-146](146-dry-run-shows-a-blank-command-for-a-steps-only-interaction.md)
(`Explain` is blind to `steps:`), which is about what the plan says, not whether it arrives.

## Result

Option **A**. The text branch reports its write errors, and the comment that said it did not need
to is gone rather than rewritten — its claim was not merely imprecise, it was refuted by the
branch beside it in the same function.

### The choice, and the argument that decided it

The comment's reasoning was that a human-facing branch is covered by SIGPIPE, so a dropped write
error cannot become a silent success. SIGPIPE covers `EPIPE` on the process's real fd 1 and
nothing else. Point stdout at a read-only descriptor and every write returns `EBADF` — an
ordinary error, no signal, no exit 141. The enumeration "a tty or a regular file" omitted exactly
the case the JSON branch's own test is built on.

What remained was the cost argument: an `if err != nil` after each print on a path where nobody
reads exit codes. That cost is real and it is what `planWriter` removes. It accumulates the first
error and skips the rest, so the branch pays for error handling once instead of 24 times, and
`Explain` ends `return p.err` instead of `return nil`. Sticky-first rather than last, because the
error that explains the failure is the one that happened first; stop-on-error, so a broken stdout
is not hammered once per line.

Writes go through an `io.Writer` field rather than `fmt.Print*`'s implicit stdout. That is what
makes the branch testable at all — `os.Stdout` is read at call time into the writer, preserving
the live resolution `fmt.Print*` had, so the existing `brokenStdout(t)` helper still reaches it.

### Reproduction, `bin/dva` v0.1.44, one fixture, stdout on a read-only fd

```
                                              before          after
( exec 3</dev/null 1>&3; dva run hello --explain --json )
  ERROR: write /dev/stdout: bad file descriptor  rc=1            rc=1
( exec 3</dev/null 1>&3; dva run hello --explain )
  before: 0 bytes, no message                    rc=0            rc=1
  after:  ERROR: write /dev/stdout: bad file descriptor

dva run hello --explain            (healthy stdout, control)
  plan printed in full                           rc=0            rc=0
```

The `--json` column is the unchanged control: it proves the harness produces a real write failure
and that the fix did not reach the branch TASK-121 already fixed. The healthy-stdout row is the
other control — making the failure path report must not make the ordinary path report too.

### Other paths with the same shape, with a count

"Same shape" means a function that returns `error`, writes its payload with bare `fmt.Print*`,
and so can return `nil` having delivered nothing. Measured over non-test Go in `internal/` and
`cmd/`, **after** this change — so `Explain` is excluded from every row below, having been the
23rd:

| | count |
|---|---|
| bare `fmt.Print*` call sites (comment lines excluded) | 142 across 23 files |
| named `func … ) error` bodies containing them | **10** (74 sites) |
| `RunE:` closures containing them | **12** (38 sites) |
| sites in functions with no error to return | 30 |

So **22 error-returning entry points still have this shape**, and `Explain` is the only one
fixed here — it is the one TASK-121 named. The largest remaining by far is
`internal/cli/show.go:showText` at 42 prints; `dva show` can print nothing and exit 0 the same
way. That is a separate task, not a silent omission: this one was scoped to the branch its parent
left open, and widening it would have made the falsification below meaningless.

The pre-change figure is worth recording because it double-checks the fix's own arithmetic:
`runner.go` held **24** bare `fmt.Print*` calls before this commit (`git show HEAD~1` confirms
it), independently reproducing the 13-in-`Explain` + 11-in-`explainSteps` count the comments and
tests cite.

Two nearby error-discarding sites were checked and are *not* instances:
`internal/cli/root.go:emitFailureJSON` and `validate_json.go:(*validateReport).fail` both drop a
write error deliberately, on paths already reporting a prior failure, where a second error would
displace the first. Documented there, and sound.

### Falsification

Each reverts one mechanism and fires a different test:

| # | Break | Fails |
|---|---|---|
| F1 | `Explain` ends `return nil` again | `TestExplainTextBranchPropagatesWriteError` (both rows) |
| F2 | `planWriter` keeps the *last* error and keeps writing after a failure | `TestPlanWriterKeepsTheFirstErrorAndStopsWriting` — `writes == 6`, and the returned error is not the first one |
| F3 | `explainSteps` reverted to raw `fmt.Print*` | `TestExplainStepsRecordsItsWriteError` |

F3 is the one that taught something. The first attempt at it used a regex that missed some call
sites, left one `p.printf` alive in the function, and therefore did *not* reproduce the bug —
which reads exactly like a weak test. It was the falsification that was incomplete, not the test.
Redone as a full function-body revert (11 → 0 `p.print` calls, verified by count), it fires
correctly — and notably `TestExplainTextBranchPropagatesWriteError/step-driven` does **not** catch
it, because that path writes through `Explain`'s own prints before reaching `explainSteps`. The
narrower test is doing work the end-to-end one cannot.

### Test replaced rather than deleted

`TestExplainTextBranchReturnsNil` asserted the opposite contract. It was not wrong when written —
TASK-121 scoped the text branch out on purpose and pinning the asymmetry was the right way to
stop it drifting. What did not survive was the stated reason. The asymmetry is gone, so the test
that pinned it is replaced by `TestExplainTextBranchPropagatesWriteError`, with the history in
its doc comment, plus `TestExplainTextBranchReturnsNilOnAHealthyWriter` to keep the nil case
covered — asserting on the plan's actual output, so it cannot pass by writing nothing.

### Gates

```
make test        exit 0   (internal/runner coverage 68.3%)
gofmt -l         0 files
go vet ./...     exit 0
make build       ok
make doc-check   OK       (test_funcs_found 1067→1070, unmatched_run 0)
```

### Changed

- `internal/runner/runner.go` — `planWriter` (sticky first error, stops on failure); `Explain`'s
  text branch and `explainSteps` write through it; `Explain` returns `p.err`; the SIGPIPE comment
  replaced with what is actually true
- `internal/runner/runner_explain_test.go` — `TestExplainTextBranchPropagatesWriteError` (2 rows)
  replaces `TestExplainTextBranchReturnsNil`; `TestExplainTextBranchReturnsNilOnAHealthyWriter`;
  `TestPlanWriterKeepsTheFirstErrorAndStopsWriting`; `TestExplainStepsRecordsItsWriteError`;
  `failingWriter`
