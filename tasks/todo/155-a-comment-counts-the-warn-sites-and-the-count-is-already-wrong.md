---
id: TASK-155
title: "A comment counts the [warn] sites to justify a stream choice, and the count went stale in days"
type: chore
priority: P4
status: todo
effort: S
created-at: 2026-08-03T14:45:00+09:00
source: "TASK-116 finalize verification — the census the fix wrote down"
depends-on: [TASK-116]
scope: "dva repo — internal/config/merge.go:610-612"
---

# Task 155: Stop shipping a census in a comment

## Problem

TASK-116 moved a `[warn]` from stdout to stderr and justified it in a comment
(`internal/config/merge.go:610-612`):

> Of the 24 production `[warn]` sites, 23 already write to stderr and the 24th is a `Sprintf`
> collected for a generated shell script (`cli/console.go:66`). This was the only one on stdout.

Measured 2026-08-03:

```
$ grep -rn '\[warn\]' --include='*.go' cmd internal tools | grep -v _test.go | grep -c Stderr
24
```

Plus `console.go:66`, still the only `Sprintf`, for **25** production sites — not 24. The
comment was correct when written (23 stderr at `b6691dc`); since then `[warn] app start` was
removed and two `--var applies only when running a plan` warnings were added at
`internal/cli/compose.go:150,152`.

## Why it matters

The reasoning in that comment is right and worth keeping: this warning fires during config load,
before any command has produced output, so a stdout write prepends a non-JSON line to a `--json`
document and breaks `dva … | jq`. That argument does not depend on the count. The count is the
only part that rots, and it rotted within days.

A number in a comment is a claim nothing checks — the same shape as
[TASK-154](154-the-ci-suffix-marks-one-of-the-five-targets-ci-actually-runs.md) and
[TASK-112](../_archive/112-check-generate-is-labelled-ci-and-ci-does-not-run-it.md). The
difference is that this one has somewhere better to live: TASK-116 added a test that enforces the
invariant, so the census belongs there, where a new stdout `[warn]` fails the build instead of
quietly making a comment wrong.

## Acceptance criteria

- [ ] `merge.go:610-612` keeps the stream rationale and loses the census, or the census moves to
      the test that enforces it.
- [ ] If it moves: the test derives the number rather than hardcoding it, so adding a `[warn]`
      site does not require editing a literal.
- [ ] Say what the number is at the time of the fix and how it was measured, with the command —
      then a later reader can re-run it rather than trust it.
- [ ] Sweep for the same pattern: a comment stating a count of things in the codebase. Print how
      many were found and how many were stale. A bare "none" is not a result with the denominator
      unstated.
- [ ] `make test` exits 0.
