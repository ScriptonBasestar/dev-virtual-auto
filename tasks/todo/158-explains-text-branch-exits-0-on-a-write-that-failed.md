---
id: TASK-158
title: "Explain's text branch exits 0 on a failed write, and the comment says that cannot happen"
type: bug
priority: P3
status: todo
effort: S
created-at: 2026-08-03T15:00:00+09:00
source: "TASK-121 finalize verification — the sibling branch, and the reason recorded for skipping it"
depends-on: [TASK-121]
scope: "dva repo — internal/runner/runner.go:77-84, :108-131"
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

- [ ] Pick one and record why:
      (A) the text branch propagates its write errors, like the JSON branch; or
      (B) it stays as-is and `runner.go:77-84` states the real reason — blast radius across ten
      print calls on a human-facing path — with no claim about which failure modes are reachable.
- [ ] Under either, the reproduction above appears in the Resolution with both exit codes, so
      the exposure is written down rather than described.
- [ ] Under A: a test covers the text branch with a failing writer, mirroring the JSON one.
- [ ] Under B: a test or comment records that exit 0 with zero bytes is a known outcome, so it is
      not rediscovered as a bug.
- [ ] Say whether any other `fmt.Print*`-only path has the same shape, with a count.
- [ ] `make test` exits 0.

## Notes

Distinct from [TASK-146](146-dry-run-shows-a-blank-command-for-a-steps-only-interaction.md)
(`Explain` is blind to `steps:`), which is about what the plan says, not whether it arrives.
