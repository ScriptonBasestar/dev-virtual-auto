---
id: TASK-145
title: "dva's own --flag=value leaks into the docker argv, and a post-`--` literal is eaten"
type: bug
priority: P2
status: todo
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

- [ ] One shared token classifier decides what is a DVA flag, handling `--flag`,
      `--flag=value`, `--flag value`, and the `--` terminator — and all four consumers use
      it. Print the call-site count; a second implementation left behind fails this.
- [ ] The three commands above are re-measured and the actual argv printed for each. No
      DVA-owned flag appears in the `docker` argv in either position.
- [ ] Everything after `--` reaches `docker` verbatim, including tokens that spell a DVA
      flag. `dva stack log infra -- --debug --tail=5` forwards both.
- [ ] `--debug=true` either enables debug or is rejected — silently accepting it and doing
      nothing is the half of this bug that hides the other half.
- [ ] A table-driven test covers all four token shapes × the pre-command and post-command
      positions, and fails without the change. Prove the `-run` pattern matches real tests.
- [ ] `make test` exits 0.

## Notes

Worth checking whether cobra's own flag parsing can be delegated to here rather than
re-implemented — four hand-written consumers is the actual defect, and the `=` form is
only the symptom that surfaced first.
