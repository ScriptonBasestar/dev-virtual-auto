---
id: TASK-145
title: "dva's own --flag=value leaks into the docker argv, and a post-`--` literal is eaten"
type: bug
priority: P2
status: done
effort: M
created-at: 2026-08-03T13:20:00+09:00
source: "TASK-092 finalize verification — 092's Left open, untracked"
depends-on: [TASK-092]
scope: "dva repo — internal/cli root flag consumers (consumeRootPersistentFlags and family)"
---

# Task 145: Teach the root-flag consumers the `=` form, in one place

## Problem

`consumeRootPersistentFlags` strips DVA's own persistent flags before the remainder is
handed to `docker`. It matches flag tokens **exactly**, so the `--flag=value` spelling
walks straight through. Measured 2026-08-03 on `bin/dva` v0.1.44:

```
$ dva stack log infra --debug=true --tail=5
… logs --debug=true --tail=5          # DVA's flag handed to docker

$ dva --debug=true stack log infra --tail=5
… logs --debug=true infra --tail=5    # same leak from the pre-command position
```

`--debug=true` does not even turn debug on, so the user gets neither the flag's effect nor
a diagnosis — just an unexplained `docker` error.

The inverse bug is in the same family. Everything after `--` is supposed to pass through
untouched, and does not:

```
$ dva stack log infra -- --debug --tail=5
… logs -- --tail=5                    # the literal --debug was eaten
```

## Why TASK-092 stopped here

TASK-092 fixed the leak it was scoped to and recorded this deliberately under "Left open":
the exact-match limitation is not local to `consumeRootPersistentFlags`.
`applyRootPersistentFlagsFromArgs`, `parseDvaFlags` and `consumeDryRunFlag` share it, so a
one-site fix would leave three siblings disagreeing about what a flag is — which is the
condition that produced this bug.

Only `tasks/done/092-…` and `tasks/done/103-…` mention it; nothing in `tasks/todo/`,
`tasks/blocked/`, `tasks/decision/` or `tasks/plan/` tracks it.

## Acceptance criteria

- [x] One shared token classifier decides what is a DVA flag, handling `--flag`,
      `--flag=value`, `--flag value`, and the `--` terminator — and all four consumers use
      it. Print the call-site count; a second implementation left behind fails this.
- [x] The three commands above are re-measured and the actual argv printed for each. No
      DVA-owned flag appears in the `docker` argv in either position.
- [x] Everything after `--` reaches `docker` verbatim, including tokens that spell a DVA
      flag. `dva stack log infra -- --debug --tail=5` forwards both.
- [x] `--debug=true` either enables debug or is rejected — silently accepting it and doing
      nothing is the half of this bug that hides the other half.
- [x] A table-driven test covers all four token shapes × the pre-command and post-command
      positions, and fails without the change. Prove the `-run` pattern matches real tests.
- [x] `make test` exits 0.

## Notes

Worth checking whether cobra's own flag parsing can be delegated to here rather than
re-implemented — four hand-written consumers is the actual defect, and the `=` form is
only the symptom that surfaced first.

Answered under Result: cobra cannot be delegated to, because these four consumers only
exist where cobra has been switched off. What was shared instead is one level down.

## Result

`internal/cli/flagtoken.go` — four token-level helpers (`splitFlagToken`, `dvaFlagEnd`,
`flagBoolValue`, `flagValue`) used from **16 call sites** across all four consumers:

```
$ grep -c 'dvaFlagEnd\|splitFlagToken\|flagBoolValue\|flagValue(' internal/cli/*.go   # minus flagtoken.go and tests
16
$ grep -n '^func applyRootPersistentFlagsFromArgs\|^func consumeRootPersistentFlags\|^func parseDvaFlags\|^func consumeDryRunFlag' internal/cli/*.go
internal/cli/compose.go:669:func parseDvaFlags(...)
internal/cli/compose.go:739:func consumeDryRunFlag(...)
internal/cli/root.go:257:func applyRootPersistentFlagsFromArgs(...)
internal/cli/root.go:293:func consumeRootPersistentFlags(...)
```

`dvaFlagEnd` is called exactly four times, once per consumer — no consumer decides where
DVA's flags stop on its own any more. The two surviving `case "--debug":` lines are switch
arms on the classifier's output, not a second implementation; no hand-rolled
`strings.HasPrefix(a, "--mode=")` matching is left anywhere.

**The shared piece is at the token level, not the argv level.** The obvious refactor —
one `scanRootFlags(args)` — cannot be written, because the four consumers own genuinely
different flag subsets. `consumeRootPersistentFlags` must *leave `--dry-run` alone* (the
compose passthrough owns it, TASK-092's carve-out), while `parseDvaFlags` must *consume*
it. An argv-level helper would have to know every subset. The decision that actually
repeated four times was "what does this one token name?", so that is what was extracted.

**Only the boundary consumer drops `--`, and only it rejects a bad bool.** Three of the
four hand their output back into DVA, where a later consumer still needs the terminator —
so they keep it. `consumeRootPersistentFlags`'s four call sites (`compose.go:43`,
`compose.go:656`, `kubectl.go:32`, `stack.go:320`) all feed an *external* argv, so it is
the one place `--` is eaten (eaten once, per POSIX) and the last code that still knows
`--debug=notabool` is DVA's rather than docker's.

The redundant `consumeDryRunFlag` pre-pass was **removed** from `parseDvaFlags`: it was a
second walk with its own idea of where `--` was, and `parseDvaFlags`'s own switch already
handled `--dry-run`. Two scans with two terminator opinions is the shape this whole task
is about.

### Re-measured, 2026-08-03, rebuilt `bin/dva`, argv-recording `docker` shim

```
$ dva stack log infra --debug=true --tail=5
ARGV: [compose] [-f] [<compose.yml>] [logs] [--tail=5]
$ dva --debug=true stack log infra --tail=5
ARGV: [compose] [-f] [<compose.yml>] [logs] [--tail=5]
$ dva stack log infra -- --debug --tail=5
ARGV: [compose] [-f] [<compose.yml>] [logs] [--debug] [--tail=5]
$ dva compose logs --json=true
ARGV: [compose] [-f] [<compose.yml>] [logs]
$ dva stack log infra --debug=false --tail=5
ARGV: [compose] [-f] [<compose.yml>] [logs] [--tail=5]
$ dva stack log infra --debug=notabool
ERROR: invalid boolean value "notabool" for --debug
```

Row 3 is criterion 3: `--debug` now survives the terminator and the terminator itself is
gone. Rows 1/5 are criterion 4's two halves — the token is stripped either way, and the
*value* decides whether debug is on (`TestRootFlagShapesNeverReachDocker/=false_is_obeyed`
asserts the second, which is what catches a fix that strips and sets `true`).

Note what the second measurement shows about the pre/post-command distinction: it does not
survive cobra. For a `DisableFlagParsing` command, `Find` hands RunE every token that is
not part of the command path, so `dva --debug=true stack log infra --tail=5` arrives as
`["--debug=true", "infra", "--tail=5"]` — the pre-command flag is simply *first*. That is
why the fix is position-insensitive and why the test table drives both spellings anyway.

### Tests

`internal/cli/flagtoken_test.go` (new) and the reworked
`TestConsumeRootPersistentFlags`. The row `{"=true form is left alone", …}` — which
encoded this defect as intended behavior — is now `{"=true form applied and stripped", …}`.

```
$ go test ./internal/cli/ -run 'TestSplitFlagToken|TestRootFlagShapesNeverReachDocker|TestParseDvaFlagValueShapes|TestMalformedBoolIsRejectedAtTheBoundary|TestConsumeRootPersistentFlags' -v | grep -c '^=== RUN'
42                    # 5 top-level + 37 subtests
$ go test ./internal/cli/ -run 'TestThisNameDoesNotExist' -v | grep -c '^=== RUN'
0                     # contrast: an unmatched pattern also exits 0
```

**Three falsifications, chosen so their failure sets are disjoint** — a fix whose halves
fail together has not been shown to have halves.

1. `splitFlagToken` returns `(a, "", false)` unconditionally (no `=` split) → only the
   `=value` rows fail; the bare-flag and terminator rows stay green. Printed
   `logs --debug=true infra --tail=5`, byte-identical to the Problem section's
   2026-08-03 measurement.
2. `dvaFlagEnd` compares against `"-----FALSIFY"` instead of `"--"` → only the two
   terminator rows fail. Printed `logs -- --tail=5` — the other measured baseline, the
   literal eaten and the terminator kept.
3. `flagBoolValue` ignores its value and returns `(true, true)` → only the
   value-*reading* rows fail (`=false is obeyed`, `=1 form`, `bool with false value`)
   while every `=true` row stays green. This is the one that matters: it is the exact
   shape of a fix that strips `--debug=X` and turns debug on regardless.

Falsification 1 was first attempted with a perl regex, which deleted the `if` body and
left the loop malformed — a build failure is not a falsification, because it proves the
compiler noticed, not the test. Restored from backup and redone with a literal swap.

### Gates

`make test` rc=0 · `go vet` rc=0 · `gofmt` clean · `make doc-check` OK.
(`make lint`'s golangci step is skipped for the known GOTOOLCHAIN drift, unrelated.)
