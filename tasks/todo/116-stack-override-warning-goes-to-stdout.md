---
id: TASK-116
title: "The `stack_override` warning is the one production `[warn]` printed to stdout, so it corrupts `--json` output"
type: fix
priority: P4
effort: S
status: todo
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/config/merge.go:605 — fmt.Printf where every other production [warn] uses fmt.Fprintf(os.Stderr, ...)"
---

# Task 116: one `fmt.Printf` in a file of `fmt.Fprintf(os.Stderr, …)`

## Problem

`internal/config/merge.go:605`:

```go
fmt.Printf("[warn] stack_override %q: %v\n", k, err)
```

Every other `[warn]` on a production path in this repository writes to stderr. This one writes to
stdout, and it fires during config merge — before any command has produced its own output.

The consequence is not the misplaced line itself, it is what it does to machine-readable output.
`dva … --json` is meant to put exactly one JSON document on stdout. With a malformed
`stack_override`, stdout instead begins:

```
[warn] stack_override "web": <error>
{
  "…": …
}
```

`jq` fails on the first line. The command still exits 0, so a script has no signal other than the
parse failure — and a script that pipes to `jq` and ignores its exit status silently gets nothing.

## Scope

One line. The reason it is filed separately rather than folded into another task is that it is the
kind of finding that gets fixed in passing, without the reasoning above being written down, and then
reintroduced the next time someone adds a warning.

The general rule this instance violates is worth stating in the fix: **diagnostics go to stderr;
stdout belongs to the command's output.** DVA has `--json` on many commands, so the rule is
load-bearing here, not stylistic.

## Verify the claim before fixing

`grep -rn '\[warn\]' internal/ --include="*.go"` and confirm the split: this site on stdout, all
other production sites on stderr. Test files may legitimately differ. If a second stdout site
exists, this task covers it too — the criterion below counts, so it will say so.

## Proposed fix

```go
fmt.Fprintf(os.Stderr, "[warn] stack_override %q: %v\n", k, err)
```

Check whether `merge.go` already imports `os` before adding it.

Consider also whether this warning should be surfaced through the same channel as
`internal/config/validate_warnings.go`, which the package CLAUDE.md describes as "경고는 에러가
아닌 출력으로만 표시". If there is an established warning channel, using it beats a raw `Fprintf` —
but only if it also writes to stderr. Check rather than assume.

## Acceptance criteria

- [ ] No production `[warn]` reaches stdout | verify: `grep -rn '\[warn\]' internal/ --include="*.go" | grep -v _test | grep -c 'fmt.Printf'` — must be 0, and print the total number of `[warn]` sites alongside it so a zero from an empty search is distinguishable
- [ ] The warning still appears | verify: `human — run dva against a fixture with a malformed stack_override; confirm the line is on stderr and still readable`
- [ ] stdout stays parseable | verify: `human — same fixture with --json; confirm 'dva … --json | jq .' succeeds`
- [ ] Full suite passes | verify: `make test`
- [ ] Lint clean | verify: `make lint`

## Related

- [TASK-079](../done/079-json-flag-does-not-cover-failures.md) — the same invariant
  from the other end: that task stopped DVA appending a *second* JSON document to stdout, this one
  stops it prepending a non-JSON line to the first.
- [TASK-114](114-output-package-has-no-tests-and-drops-write-errors.md) — the third member of the
  set. All three are about stdout being a typed channel that DVA does not consistently treat as one.
