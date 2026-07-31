---
id: TASK-116
title: "The `stack_override` warning is the one production `[warn]` printed to stdout, so it corrupts `--json` output"
type: fix
priority: P4
effort: S
status: done
resolved-at: 2026-07-31T00:00:00+09:00
resolution: "Moved the one production [warn] on stdout to stderr; regression test captures both streams and mutation-fails on either half"
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

- [x] No production `[warn]` reaches stdout | verify: `grep -rn '\[warn\]' internal/ --include="*.go" | grep -v _test | grep -c 'fmt.Printf'` — must be 0, and print the total number of `[warn]` sites alongside it so a zero from an empty search is distinguishable — **0 on stdout, 24 print/format sites total (23 stderr + 1 collected Sprintf)**
- [x] The warning still appears | verify: `human — run dva against a fixture with a malformed stack_override; confirm the line is on stderr and still readable` — **measured, not delegated; see Resolution**
- [x] stdout stays parseable | verify: `human — same fixture with --json; confirm 'dva … --json | jq .' succeeds` — **measured: jq OK while the warning fires**
- [x] Full suite passes | verify: `make test` — exit 0, FAIL count 0, 5 packages ok
- [x] Lint clean | verify: `make lint` — `0 issues.`, gofmt 213 files 0 unformatted

## Resolution

`internal/config/merge.go:605` now writes to stderr. The comment above it records the count so the
next person does not have to re-derive it.

### The established-channel question, answered rather than assumed

The task said to check `validate_warnings.go` before writing a raw `Fprintf`. Checked:
`ValidateWarnings()` returns `[]string` and its one caller, `cli/validate.go:96`, prints them with
`fmt.Fprintf(os.Stderr, "[warn] semantic: %s\n", w)`. So the established channel also targets
stderr — but it is a *validate-time* collection, and this warning fires during config load inside
`mergeEnvironmentProfile`, which the comment at `:601-603` notes cannot return errors. Routing it
through that channel would mean re-deriving the merge conflict at validate time, a much larger
change than the defect warrants. Raw `Fprintf` to stderr, matching `validate.go:96`'s exact form.

### Measured, with the fixture that actually reaches the path

Getting there took two corrections. `mergeEnvironmentProfile` is only reached when *two* configs
define the same environment profile with a `stack_overrides` entry for the same key — a single file
with a conflicting override errors out through `Load` instead. And `modules:` entries resolve to
`.sb/dva/<name>.yml`, not to a path relative to `dva.yml`:

```
ERROR: loading module `module.yml`: open …/task-116/.sb/dva/module.yml.yml: no such file
```

With `tmp/task-116/dva.yml` (`environments.dev.stack_overrides.api.plugin: compose`) plus
`.sb/dva/module.yml` (same key, `plugin: helm`):

```
$ dva ls 2>/dev/null | grep -c warn
0                                        # stdout clean

$ dva ls 2>&1 1>/dev/null | grep stack_override
[warn] stack_override "api": cannot override plugin type for stack entry "": …

$ dva ls --json 2>/dev/null | jq . >/dev/null && echo OK
OK                                       # while the warning fires on stderr
```

### Regression test and what it actually catches

`TestStackOverrideConflictWarnsOnStderrNotStdout` (`internal/config/merge_test.go`) calls
`mergeEnvironmentProfile` with a compose→helm conflict, capturing both streams through pipes.

Mutation-tested by restoring `fmt.Printf`. Both assertions fire independently:

```
config merge wrote 120 bytes to stdout, which corrupts --json output:
[warn] stack_override "api": cannot override plugin type …
the warning did not reach stderr; got "" — a conflict must still be reported
```

The second assertion is the one that matters for durability: it stops a future "fix" that deletes
the warning instead of moving it. Green run logs `stdout=0 bytes, stderr=120 bytes`.

### Observed but deliberately not fixed

The inner error names an **empty** entry: `for stack entry ""`, because the `LifecycleEntry` values
under `stack_overrides` carry no `Name`. The outer message still names the key (`"api"`), so the
user can identify the offending override — the message is imprecise, not useless. Recorded here
rather than filed, since fixing it means deciding whether `stack_overrides` entries should have
`Name` backfilled at parse time, which is a config-semantics question and not a stdout question.

## Related

- [TASK-079](../done/079-json-flag-does-not-cover-failures.md) — the same invariant
  from the other end: that task stopped DVA appending a *second* JSON document to stdout, this one
  stops it prepending a non-JSON line to the first.
- [TASK-114](../todo/114-output-package-has-no-tests-and-drops-write-errors.md) — the third member of the
  set. All three are about stdout being a typed channel that DVA does not consistently treat as one.
