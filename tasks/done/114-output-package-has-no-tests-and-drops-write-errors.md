---
id: TASK-114
title: "`internal/output` has 0.0% coverage, drops every write error, and its doc comment states a guarantee the code does not provide"
type: fix
priority: P3
effort: S
status: done
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/output/output.go — the whole package is 47 lines with no _test.go; :33 and :44 discard the fmt error; :10-12 documents 'set only on a successful write'"
---

# Task 114: the package that decides whether stdout is parseable is the one with no tests

## Measured

```
go test ./internal/output/ -cover
  github.com/ScriptonBasestar/dva/internal/output    coverage: 0.0% of statements
ls internal/output/*_test.go
  zsh: no matches found
grep -rn 'output.PrintJSON\|output.PrintYAML' --include="*.go" . | wc -l
  20
```

20 call sites across the CLI, zero tests. Every `--json` and `--yaml` path in DVA goes through
these two functions.

## Problem 1 — the write error is discarded

```go
func PrintJSON(data any) error {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(bytes))   // returns (int, error); both dropped
	stdoutHasDocument = true
	return nil
}
```

`PrintYAML` is identical at `:44`. `fmt.Println` fails on a closed or full stdout — `dva … --json |
head -1` gives EPIPE, a full disk gives ENOSPC. In every such case `PrintJSON` returns `nil` and
the command exits 0 having emitted a truncated document or none at all.

## Problem 2 — the doc comment names a guarantee that is not enforced

`internal/output/output.go:10-12`:

```go
// stdoutHasDocument records that one of the printers below has already put a document on
// stdout. It is set only on a successful write, so a marshalling failure — which prints
// nothing — leaves stdout still empty as far as callers are concerned.
```

The second half is true: a marshalling failure returns before the flag is set. The first half is
not — the flag is set after an *attempted* write, whatever the write returned. The comment says
"write" where the code only checks "marshal".

This is not a cosmetic wording problem, because a real caller depends on the flag:

`internal/cli/root.go:325-337` — `emitFailureJSON` suppresses the failure envelope when
`StdoutHasDocument()` is true, on the reasoning that appending a second JSON document would give
`jq` a stream it cannot read as one object. If a partial write set the flag, stdout holds a
*truncated* document, the envelope is suppressed, and the consumer gets malformed JSON with no
error object and — because of problem 1 — exit 0.

The two defects compose. Either alone is survivable; together they turn a broken pipe into a
silent success carrying invalid output.

## Note on the deliberately-dropped error

`emitFailureJSON` drops `PrintJSON`'s error on purpose and says why (`root.go:327-329`): its map
holds only strings and an int and cannot fail to marshal, and both callers print to stderr and exit
1 immediately after. That reasoning is sound **for marshalling** and is not what this task
challenges. It is worth noticing that the comment reasons about marshal failure only — the same
blind spot as `output.go:10-12`. Fixing the printers does not require touching it.

## Proposed fix

1. Capture and return the error from `fmt.Println` / `fmt.Print`.
2. Set `stdoutHasDocument` only when the write actually succeeded, making the existing comment true
   rather than rewriting the comment to match the code. If the write partially succeeded, the flag
   should still be set — stdout is dirty either way — so the two cases must be distinguished
   deliberately, not by accident. Whichever is chosen, the comment must say it.
3. Add the missing tests. The package is 47 lines; there is no reason for it to be the untested one.

Decide point 2 before writing code — it is the only real design question here.

## Acceptance criteria

- [x] Write failures are reported | verify: `go test ./internal/output/ -run 'WriteError' -v` — a printer given a failing writer must return non-nil
- [x] The stdoutHasDocument contract matches its comment | verify: `go test ./internal/output/ -run 'HasDocument' -v` — assert both the success and the write-failure case
- [x] Round-trip correctness | verify: `go test ./internal/output/ -run 'PrintJSON|PrintYAML' -v` — output unmarshals back to the input
- [x] Coverage is no longer zero | verify: `go test ./internal/output/ -cover` — print the percentage
- [x] Not vacuous | verify: `human — disable each fix in turn, confirm the matching test fails for the stated reason, revert`
- [x] Full suite passes | verify: `make test`

## Related

- [TASK-079](../done/079-json-flag-does-not-cover-failures.md) — introduced
  `stdoutHasDocument` and its consumer. This task is about the half of the contract that was
  documented but not implemented.
- [TASK-120](../done/120-printyaml-panics-where-printjson-returns-an-error.md) — found by
  the tests this task added.
- [TASK-121](../done/121-explain-cannot-report-a-failed-json-write.md) — the one caller that
  cannot receive the error this task made real.

---

# Resolution

## The stated trigger was wrong

The task claimed `dva … --json | head -1` gives EPIPE, `PrintJSON` returns nil, and the
command exits 0. Measured, that is not what happens. Two separate corrections:

| attempt | what actually happens |
|---|---|
| `prog >&-` (parent closes fd 1) | `fmt.Println` returns `n=6 err=<nil>`. Since Go 1.21 the runtime reopens fds 0/1/2 on `/dev/null` at startup, so the write **succeeds**. No failure at all. |
| `prog \| true` with a 300 KB write | probe exits **141**. Go raises SIGPIPE and the process dies before `fmt.Println` returns, because `os.File` records at construction whether its descriptor is 1 or 2 and EPIPE on those is fatal by design. |

So a broken pipe is not the trigger: it kills the process loudly. The defect needs an error
that is **not** EPIPE on fd 1 — ENOSPC on a regular file. Reproducing it took a 1 MB HFS+
disk image, filled to leave 4096 bytes (the smallest free amount HFS+ will report), and a
config padded so `dva show --json` emits 7020 bytes.

## Measured, real binary, identical volume conditions

| | before | after |
|---|---|---|
| exit code | **0** | **1** |
| bytes on stdout | **0** | 101 |
| stderr | *(empty)* | `ERROR: write /dev/stdout: no space left on device` |
| stdout parses as one JSON object | n/a — empty | **yes** |

The 101 bytes are not a truncated `show` document. The write landed nothing, so
`stdoutHasDocument` stayed false, `emitFailureJSON` was free to run, and its envelope —
small enough to fit the remaining 4096 bytes — is what reached stdout:

```json
{ "error": { "exit_code": 1, "message": "write /dev/stdout: no space left on device" } }
```

The two defects composed in the other direction as well: TASK-079 built the failure
envelope, but it could never fire on a write failure, because `PrintJSON` always claimed
success. Fixing the dropped error is what let the existing machinery work.

## The design question (proposed fix, point 2)

Resolved as **key the flag on the byte count, not on the error**:

```go
n, err := io.WriteString(w, s)
if n > 0 {
	stdoutHasDocument = true
}
return err
```

The flag has one consumer and it asks whether stdout is dirty. A write that failed after
delivering part of a document has dirtied it just as thoroughly as one that succeeded, so
`err == nil` is the wrong predicate: it would let a failure envelope be appended to a
truncated document and hand a consumer two half-objects. Three outcomes, three rows in
`TestStdoutHasDocumentTracksBytesDelivered`, and they must not collapse into two.

Honest limit: the partial case was never observed in the wild. HFS+ returned `n == 0` on
both real runs. It is reachable only from a writer that stops short deliberately, and the
production comment now says so rather than implying it was measured.

## Two departures from the proposed fix

1. **A seam, not a global.** Testing a failing writer needs one. Rather than a test-only
   package variable, the write moved into an unexported `printDocument(w io.Writer, s string)`.
   `os.Stdout` is still read at call time inside `PrintJSON`/`PrintYAML` — required, not
   stylistic: `internal/cli/show_test.go:153` captures output by assigning a pipe to
   `os.Stdout` and unmarshals what `PrintJSON` wrote, and a writer resolved once at package
   init would write straight past it to the real stdout. Six test files across four packages
   do the same thing.
2. **The write-failure tests use a real errno, not only a fake.** `brokenStdout` points
   `os.Stdout` at a pipe whose read end is already closed. That is genuine EPIPE on a real
   `os.File`, and it is survivable precisely because Go keys the fatal-SIGPIPE decision on
   the descriptor number rather than on which variable holds the file — the same fact that
   makes `| head -1` fatal in production makes it testable here.

## Counts

| check | value |
|---|---|
| `go test ./internal/output/ -cover` | **92.9%**, from 0.0% |
| `PrintJSON` / `printDocument` / `StdoutHasDocument` / `ResetStdoutDocument` | 100.0% each |
| `PrintYAML` | 75.0% — see below |
| `make test` | exit **0** |
| `make lint` | exit **0**, 0 issues |

## Mutation testing

Three mutants, each reverted and confirmed byte-identical with `diff -q`:

| mutant | killed by |
|---|---|
| `n, _ := …; return nil` (the original dropped error) | `TestPrintJSONReturnsWriteError`, `TestPrintYAMLReturnsWriteError`, and both failing subtests of `TestStdoutHasDocumentTracksBytesDelivered`. The rebuilt binary also reproduced the original defect exactly: exit 0, 0 bytes stdout, 0 bytes stderr. |
| flag keyed on `err == nil` | `…/a_write_that_lands_part_of_the_document_sets_the_flag` |
| `stdoutHasDocument = true` unconditionally (the original) | `…/a_write_that_lands_nothing_leaves_the_flag_clear` |

## Left open, deliberately

- **`PrintYAML` panics** on a value it cannot marshal — `yaml.v3` raises a plain string
  panic and re-panics anything that is not its own error type, so that error return can
  never carry this class of failure. It is why `PrintYAML` sits at 75% while everything
  else reaches 100%: the uncovered statement is unreachable. Filed as **TASK-120**, not
  fixed here, because recovering it converts a deliberate fail-fast into a soft error and
  that is a decision rather than a mechanical repair. The test says so instead of asserting
  the panic — writing the defect down as the requirement is the failure mode TASK-115 was
  about.
- **`runner.Explain` cannot propagate the error**, having no return value. Filed as
  **TASK-121**. Fixing it is a signature change plus two callers in `internal/cli/run.go`,
  which is outside a task scoped to `internal/output/output.go`.
- The two other dropped errors — `root.go:334` and `validate_json.go:123` — were checked
  and left alone. Both exit 1 with the message on stderr regardless, so neither can turn a
  failed write into a reported success.
