---
id: TASK-114
title: "`internal/output` has 0.0% coverage, drops every write error, and its doc comment states a guarantee the code does not provide"
type: fix
priority: P3
effort: S
status: todo
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

- [ ] Write failures are reported | verify: `go test ./internal/output/ -run 'WriteError' -v` — a printer given a failing writer must return non-nil
- [ ] The stdoutHasDocument contract matches its comment | verify: `go test ./internal/output/ -run 'HasDocument' -v` — assert both the success and the write-failure case
- [ ] Round-trip correctness | verify: `go test ./internal/output/ -run 'PrintJSON|PrintYAML' -v` — output unmarshals back to the input
- [ ] Coverage is no longer zero | verify: `go test ./internal/output/ -cover` — print the percentage
- [ ] Not vacuous | verify: `human — disable each fix in turn, confirm the matching test fails for the stated reason, revert`
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-079](../done/079-json-flag-does-not-cover-failures.md) — introduced
  `stdoutHasDocument` and its consumer. This task is about the half of the contract that was
  documented but not implemented.
