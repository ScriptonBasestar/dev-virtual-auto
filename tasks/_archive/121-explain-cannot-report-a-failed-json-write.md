---
id: TASK-121
title: "`runner.Explain` returns nothing, so `dva run --explain --json` cannot report a failed write"
type: fix
priority: P3
effort: S
status: done
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/runner/runner.go:77 Explain (no return value), :98 discards output.PrintJSON's error; two callers in internal/cli/run.go"
verified-at: 2026-08-03T15:00:00+09:00
archived-at: 2026-08-03T15:00:00+09:00
verification-summary: |
  Exercised the error path on ./bin/dva rather than reading it. With stdout dup'd from a
  read-only fd (writes return EBADF, no SIGPIPE confound, no pipe/head involved):
    `dva run hello --explain --json` → exit 1, stderr "ERROR: write /dev/stdout: bad file
    descriptor", 0 bytes on stdout. The pre-fix shape (exit 0, 0 bytes) is gone.
  The unit test is real, not a stub: brokenStdout (runner_explain_test.go:81-96) swaps the
  os.Stdout variable for a pipe with a closed read end and TestExplainJSONBranchPropagates-
  WriteError asserts Explain(&cmd, true) != nil. Under -v: 7 RUN / 7 PASS / 0 FAIL, so the
  TASK-144 exec-erasure hazard does not apply here (no ExecReplace in this package's tests).
  Both cobra RunE call sites return the error verbatim. The two surviving `_ = output.Print`
  sites are production paths that provably still exit 1, each with the required comment.
---

# Task 121: the one caller left that cannot pass the error on

[TASK-114](../_archive/114-output-package-has-no-tests-and-drops-write-errors.md) made
`output.PrintJSON` report write failures instead of swallowing them. Of the twenty call
sites, seventeen already returned the error and two drop it with a stated reason. This is
the third, and it drops it because it has nowhere to put it.

```go
func Explain(cmd *ResolvedCommand, jsonOutput bool) {     // runner.go:77 — no return value
	…
	_ = output.PrintJSON(plan)                            // runner.go:98
	return
}
```

`dva run <name> --explain --json` on a stdout that cannot be written therefore prints
nothing and exits 0 — the same shape TASK-114 removed from every other path, still present
on this one. TASK-114 could not fix it inside its own scope: the repair is a signature
change plus both callers, not an edit to `internal/output`.

## The other two dropped errors are fine, for the record

- `internal/cli/root.go:334` (`emitFailureJSON`) — documented at `root.go:326-329`. Both
  callers print the same message to stderr and exit 1 immediately after, so a lost write
  error cannot turn into a silent success.
- `internal/cli/validate_json.go:123` (`validateReport.fail`) — returns `err` to the caller
  regardless, so the command still exits 1.

Neither reports success on a failed write. `Explain` does.

## Proposed fix

1. `func Explain(cmd *ResolvedCommand, jsonOutput bool) error`.
2. Return `output.PrintJSON(plan)` on the JSON branch; return `nil` from the text branch —
   or, if the text branch's `fmt.Println` calls are worth checking too, say so explicitly
   rather than leaving the asymmetry unremarked.
3. Update `internal/cli/run.go:54` and `:97` to propagate it. Both sit in cobra `RunE`
   bodies, so there is somewhere for it to go.

Step 2 is the only judgement call: the text branch has roughly a dozen bare `fmt.Printf`
calls with the same exposure, and fixing the JSON branch alone leaves `--explain` without
`--json` still silent. Decide whether this task covers both branches or only the one the
`--json` contract names.

## Acceptance criteria

- [x] The silent path is reproduced first | verify: `human — a full filesystem under stdout (a 1 MB disk image works; see TASK-114's resolution), then 'dva run <name> --explain --json > /Volumes/tiny/out'; record exit code and bytes delivered`
- [x] Explain propagates the write error | verify: `go test ./internal/runner/ -run 'Explain' -v`
- [x] Both callers propagate it | verify: `grep -n 'runner.Explain' internal/cli/run.go` — neither line may discard the result
- [x] No caller reports success on a failed write | verify: `grep -rn '_ = output.Print' --include="*.go" internal/` — every remaining hit must carry a comment saying why it cannot mask a failure
- [x] Full suite passes | verify: `make test`

## Related

- [TASK-114](../_archive/114-output-package-has-no-tests-and-drops-write-errors.md) — made the
  error real; this is the caller that cannot receive it.
- [TASK-158](../todo/158-explains-text-branch-exits-0-on-a-write-that-failed.md) — ⚠️ the text
  branch, and the judgement call left open below. Measured for archival: with stdout on a
  read-only fd, `--explain --json` exits 1 with the write error and `--explain` exits 0 having
  delivered nothing. The scoping decision holds; the reason recorded at `runner.go:77-84` does
  not — `EBADF` makes the write fail rather than succeed-and-be-lost, so the exposure the comment
  calls unreachable is the one reproduced here.

## Resolution

`Explain` returns `error`. The JSON branch returns `output.PrintJSON(plan)`; both call sites in
`internal/cli/run.go` now `return runner.Explain(...)`. The text branch returns `nil` by design.

### Reproduction (criterion 1): why a unit test, not a disk image

The criterion asks for a full-filesystem repro on a real binary. It is satisfied by the
`brokenStdout` in-process test rather than another 1 MB HFS+ image, for three measured reasons:

1. **TASK-114 already ran the disk-image experiment.** `ac93d81` measured `output.PrintJSON`
   against a 1 MB volume held at 4096 bytes free and recorded exit 0 / 0 bytes / 0 bytes **before**
   and exit 1 / 101 bytes / one parseable JSON envelope **after**. That is the same write path this
   function calls — `printDocument → io.WriteString(os.Stdout, …)`. Re-running it here exercises the
   same code, so it would re-prove TASK-114 rather than TASK-121.
2. **A pipe is not the defect's trigger.** `dva run paddedcmd --explain --json | head -1` looks like
   a repro but is not: Go decides a fatal SIGPIPE from the descriptor NUMBER, and stdout is fd 1, so
   the process dies at exit 141 instead of returning an error. Measured under TASK-114.
3. **TASK-121's change is the gap between the error and the exit code, and `brokenStdout` tests
   exactly that gap.** It swaps the `os.Stdout` variable for a pipe whose read end is closed (a fresh
   descriptor the runtime does not treat as fatal), the write returns EPIPE, and the test asserts
   `Explain(&cmd, true)` returns non-nil. Before this fix that returns `nil`.

What `brokenStdout` does not reproduce is the *byte-count* semantics of a partial ENOSPC write. That
property belongs to `internal/output` and is covered there; `Explain` is a pass-through.

A disk image was built and tried (1 MB HFS+, `tmp/task-121/dva.yml` padded to a 7390-byte plan to
clear the floor). macOS HFS+ reserves blocks beyond the 4096-byte reportable floor and the image
would not accept a byte-precise fill, so the repro was taken in-process. The fixture and the
before/after binaries (`$SP/dva-before121`, `bin/dva`) were built; the image was detached and removed.

### Measured

- `go test ./internal/runner/ -run 'Explain' -v` → exit 0, 7 PASS lines (4 existing + 3 new)
- `grep -n 'runner.Explain' internal/cli/run.go` → two hits, both `return runner.Explain(...)`,
  neither discards
- `grep -rn '_ = output.Print' --include="*.go" internal/` → two code hits remain (`root.go:334`
  `emitFailureJSON`, `validate_json.go:123` `validateReport.fail`), each already commented to say why
  it cannot mask a failure; the `runner.go` hit is gone. One test-file hit (`failure_json_test.go:57`)
  which is not a production path.
- `make test` → exit 0, 6 packages ok
- `make lint` → exit 0, 0 issues

### Mutation testing

One mutant, reverted and confirmed byte-identical with `diff -q`:

| mutant | killed by | what the failure said |
|---|---|---|
| JSON branch restored to `_ = output.PrintJSON(plan)` + `return nil` | `TestExplainJSONBranchPropagatesWriteError` | `Explain(--json) = nil, want the write error from output.PrintJSON` |

A second mutant — restoring `runner.Explain(resolved, jsonOutput); return nil` at one call site — is
not unit-testable because it lives in a cobra `RunE`, but criterion 3's `grep` catches it: the fix is
verified by the absence of a bare `runner.Explain(` line. `TestExplainTextBranchReturnsNil` pins the
text branch's `nil` so a future pass that "fixes" it by returning the last `fmt` error cannot do so
silently.
