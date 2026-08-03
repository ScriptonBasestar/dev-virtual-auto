---
id: TASK-146
title: "--dry-run prints an empty Command line for a steps-only interaction and names no step"
type: bug
priority: P2
status: todo
effort: S
created-at: 2026-08-03T13:20:00+09:00
source: "TASK-094 finalize verification — the shape 094 fixed on the exec path, surviving on the explain path"
depends-on: [TASK-094]
scope: "dva repo — internal/runner/runner.go (Explain)"
---

# Task 146: Make the execution plan describe steps

## Problem

`Explain` (`internal/runner/runner.go:85-131`) is steps-blind. `grep -c Steps
internal/runner/runner.go` → **0**. It prints `Command`, `Description`, `Runner`,
`Service`, `Compose Method`, `Pod`, `Shell Mode` and `Arguments` — and nothing about the
`steps:` list, which is where the work actually lives for a steps-only interaction.

On TASK-094's own repro fixture, an interaction with two declared steps:

```
$ dva run seed --dry-run
Command:                  <- blank; there is no single command
Runner: kubectl
Pod: myapp-0
Shell Mode: true
```

Two steps, neither named. The blank `Command:` line is worse than an omission — it invites
the reading that nothing will run.

This is exactly the shape TASK-094 fixed on the execution path, where `KubectlRunner`
discarded steps outright. `--dry-run` is the command a user reaches for *precisely* when
they do not trust what is about to happen, so it is the last place the declared work should
be invisible.

## Acceptance criteria

- [ ] `dva run <steps-only> --dry-run` lists each step, in order, with whatever the step
      will actually execute — `run:`, `cmd:`, `compose_up:`, `echo:` and the rest.
- [ ] `Command:` is not printed blank. Either it is omitted when there is no single
      command, or it states that the interaction is step-driven.
- [ ] All three runners (local, docker_compose, kubectl) produce the same shape, since
      TASK-094 unified the execution path — the explain path should not re-fork it.
- [ ] A step carrying `note:` renders it the same way the executing path does; see
      TASK-141, which owns the note-rendering convention.
- [ ] A test pins the dry-run output for a steps-only interaction and fails without the
      change. Prove the `-run` pattern matches a real test — and read TASK-144 first: an
      in-process test that reaches `ExecReplace` can erase its own failure.
- [ ] `make test` exits 0.

## Notes

Related but out of scope: `warnUnreachableCommands`
(`internal/config/validate_warnings.go:818`) calls `hasExecutionTarget()`, which at
`validate_warnings.go:364-366` counts `HasSteps()` without asking which runner will run it.
TASK-094 recorded this deliberately; it is harmless while all three runners honour steps,
and stops being harmless the moment a fourth runner does not.
