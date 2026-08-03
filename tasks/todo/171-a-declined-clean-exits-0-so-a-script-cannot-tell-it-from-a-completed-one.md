---
id: TASK-171
title: "a declined `clean` exits 0, so a script cannot tell it from a completed one"
type: bug
priority: P3
status: todo
effort: S
created-at: 2026-08-03T17:30:00+09:00
source: "TASK-170 — measured while confirming the prompt still guards the real path"
depends-on: [TASK-170]
scope: "dva repo — internal/cli/compose.go"
---

# Task 171: Make a declined `clean` distinguishable from one that ran

## Problem

`cleanCmd`'s confirmation prompt returns `nil` when the answer is not `y`
(`internal/cli/compose.go`, the `Aborted.` branch). Measured after TASK-170:

```
$ dva clean --volumes </dev/null
This will remove all containers, networks, and VOLUMES (data loss!).
Continue? [y/N] Aborted.
$ echo $?
0
```

A caller that checks the exit code — which is the only thing a script can check — is told
the clean succeeded. Nothing was removed.

Two different situations are being collapsed into that one silent zero:

1. **A person typed `n`.** They meant it, and rc 0 is arguably right: the command did what
   they asked. This is the case worth being careful about changing.
2. **`fmt.Scanln` hit EOF**, because there is no terminal — a pipe, a CI runner, a Makefile
   recipe. Nobody declined anything. There was no way to answer, and the command reports
   success for work it did not do.

Case 2 is the one with no defensible reading. `--force` exists precisely for this situation,
and the fix is to say so rather than to guess an answer on the operator's behalf.

TASK-170 removed this trap from the `--dry-run` path by exempting the preview from the
prompt entirely. The real path still has it.

## Acceptance criteria

- [ ] `dva clean --volumes </dev/null` (no terminal, no `--force`) fails rather than
      silently succeeding, and its message names `--force` as the way to proceed
      non-interactively.
      Verify: `dva clean --volumes </dev/null; test $? -ne 0`
- [ ] An interactive `n` keeps whatever exit code the task decides on, and that decision is
      written down with its reasoning. Changing it is a compatibility question — a wrapper
      script today may well treat `n` as "fine, carry on".
      Verify: `human — state the decision and the reason in the Result section`
- [ ] EOF is distinguished from an explicit decline at the call site, not inferred. `Scanln`
      returns an error on EOF; the current code discards it (`_, _ = fmt.Scanln(&answer)`).
      Verify: `grep -n 'fmt.Scanln' internal/cli/compose.go` shows the error being read
- [ ] A test covers both arms — EOF and an explicit `n` — through `cleanCmd.RunE`, asserting
      on the returned error rather than only on output. `internal/cli/clean_prompt_test.go`
      has the stdin and stream capture helpers already.
      Verify: `go test ./internal/cli/ -run Clean -count=1`
- [ ] `make test` exits 0.
      Verify: `make test`

## Notes

Check the other direction before changing the interactive answer: does anything in
`examples/`, the docs, or the dogfood workflows run `dva clean` and branch on its exit code?
If so, that is the compatibility surface.

Worth deciding at the same time whether an EOF failure should be a general rule for
confirmation prompts or stay local to this one. There is still only one prompt in the
codebase, so the same argument TASK-170 made against a shared helper applies — but if a
second prompt is ever added, this is the behaviour it should inherit.
