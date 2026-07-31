---
id: TASK-121
title: "`runner.Explain` returns nothing, so `dva run --explain --json` cannot report a failed write"
type: fix
priority: P3
effort: S
status: todo
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/runner/runner.go:77 Explain (no return value), :98 discards output.PrintJSON's error; two callers in internal/cli/run.go"
---

# Task 121: the one caller left that cannot pass the error on

[TASK-114](../done/114-output-package-has-no-tests-and-drops-write-errors.md) made
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

- [ ] The silent path is reproduced first | verify: `human — a full filesystem under stdout (a 1 MB disk image works; see TASK-114's resolution), then 'dva run <name> --explain --json > /Volumes/tiny/out'; record exit code and bytes delivered`
- [ ] Explain propagates the write error | verify: `go test ./internal/runner/ -run 'Explain' -v`
- [ ] Both callers propagate it | verify: `grep -n 'runner.Explain' internal/cli/run.go` — neither line may discard the result
- [ ] No caller reports success on a failed write | verify: `grep -rn '_ = output.Print' --include="*.go" internal/` — every remaining hit must carry a comment saying why it cannot mask a failure
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-114](../done/114-output-package-has-no-tests-and-drops-write-errors.md) — made the
  error real; this is the caller that cannot receive it.
