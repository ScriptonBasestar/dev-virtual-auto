---
id: TASK-089
title: "A step with both `note:` and `run:` executes under `dva provision` and silently does not under `dva run`"
type: fix
priority: P2
effort: S
status: done
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/runner/local.go, internal/runner/docker_compose.go — the Note branch continues; internal/cli/provision.go:125-131 falls through"
---

# Task 089: the same item means two different things depending on who runs it

## Problem

Both runners treat `note:` as *instead of* the step's work:

```go
if step.Note != "" {
    fmt.Printf("  → %s: %s\n", label, step.Note)
    continue                                       // ← local.go and docker_compose.go
}
```

`internal/cli/provision.go:125-131` treats it as *before* the work — it prints the note and falls
straight through to the compose/run execution below, with no `continue`.

So an item carrying both keys runs on one path and not the other. Nothing reports the difference,
and both exit 0.

## Evidence (measured on 0.1.44, one fixture)

| invocation | note shown | command executed | exit |
| --- | --- | --- | --- |
| `dva run runonly` — `run:` only (control) | n/a | **yes** (`RUNONLY-EXECUTED` ×1) | 0 |
| `dva run noteandrun` — `note:` + `run:` | yes | **no** (`BOTH-RUN-EXECUTED` ×0) | 0 |
| `dva provision seqboth` — the same two keys | yes | **yes** (`PROV-RUN-EXECUTED` ×2) | 0 |

Row 1 is the control that rules out "the runner is broken": strip the note and the identical
command executes. Row 3 is the control that rules out "this shape is simply unsupported": the
provision path accepts it and does the obvious thing. Row 2 is the defect — the note suppresses
the command, silently, with a success exit code.

(`×2` in row 3 is the echoed `$ echo …` line plus its output, which is how the provision path
renders a command it runs.)

```yaml
interaction:
  noteandrun:
    steps:
      - step: "a step carrying both a note and a run"
        note: "BOTH-NOTE-SHOWN"
        run: "echo BOTH-RUN-EXECUTED"     # dva run noteandrun → never executes
provision:
  seqboth:
    - step: "provision step with both note and run"
      note: "PROV-NOTE-SHOWN"
      run: "echo PROV-RUN-EXECUTED"       # dva provision seqboth → executes
```

## Which one is right

`provision.go` is. A note is documentation attached to a step, not a replacement for it — that is
what the key's own description implies and what the only other implementation already does. The
runners' `continue` also makes `note:` silently *destructive* in a way no other key is: adding a
comment to a working step stops it working.

There is a competing reading — `note:` marks a step as a message-only placeholder — but it does
not survive the fixture above, because under that reading `provision` would be the buggy one, and
`provision` is the path where `note:` is actually documented and used.

## Proposed fix

Drop the `continue` in both runners so the note prints and execution proceeds, matching
`provision.go`. Two lines.

Deliberately paired with [TASK-086](../todo/086-parallel-steps-discard-their-note.md): that task adds the
note to the paths that never print it, this one stops it swallowing work on the paths that do.
Doing either alone leaves `note:` meaning something different in three places instead of two.

Check while there: `hooks.go` prints this class of message to **stderr** while `provision.go` uses
stdout. Not this task's decision, but the fix touches the same lines.

## Non-goals

- Not changing what a note-only step does. An item with `note:` and no payload must keep printing
  its note and running nothing — that is the documented shape and TASK-083's tests pin it.
- Not changing note formatting. The runners' one-line `label: note` and provision's indented
  multi-line block can stay different; this is about execution, not rendering.

## Acceptance criteria

- [x] A note no longer suppresses a run on the interaction path | verify: `dva run noteandrun` on the fixture — `BOTH-RUN-EXECUTED` count must be >=1; today 0 — **measured 1**
- [x] A note-only step still runs nothing | verify: `go test ./internal/runner/ -run TestStepWithoutRunIsReported` — the 9 subtests from TASK-083 must still pass, print the count — **9 passing**
- [x] The note is still printed | verify: same run — `BOTH-NOTE-SHOWN` count must stay >=1, so "fixed" cannot mean "dropped the note instead" — **measured 1**
- [x] Both runners agree | verify: `go test ./internal/runner/ -run TestNoteDoesNotSuppressRun` — must drive both runners from one table, as the TASK-083 test does — **8 subtests, 4 per runner, one table**
- [x] The provision path is unchanged | verify: `dva provision seqboth` — `PROV-RUN-EXECUTED` count must stay 2 and `PROV-NOTE-SHOWN` 1 — **2 and 1**
- [x] Not vacuous | verify: `human — restore the continue in local.go alone; only the local subtest may fail, proving the table tests each runner separately` — **restored the `continue` in local.go only: exactly `local/a_note_does_not_suppress_the_run` failed, all four docker_compose subtests stayed green**
- [x] Full suite passes | verify: `make test` — **all packages ok**

## Resolution

Both runners now fall through instead of `continue`ing. A `noted` flag suppresses the plain
label print when the note line already carried it, so a note+run step is still named exactly once:

```go
noted := step.Note != ""
if noted {
    fmt.Printf("  → %s: %s\n", label, step.Note)
}
cmds := step.RunCommands()
...
if len(cmds) == 0 {
    continue          // a note-only step still executes nothing
}
if !noted {
    fmt.Printf("  → %s\n", label)
}
```

Covered by `internal/runner/note_run_test.go` — `TestNoteDoesNotSuppressRun`, four subtests per
runner from one table.

**Testing the compose runner needed care, and the reason is worth keeping.** `execCompose` ends in
`ExecReplace` → `syscall.Exec`, which on success *replaces the running process*. A test that let
`DockerComposeRunner.executeSteps` reach a real binary would have deleted the test binary
mid-run. The test points its compose command at a name that cannot resolve, so `exec.LookPath`
fails *before* the exec: the attempt is observable as an error, and nothing is ever executed.
That is also why the two runners need different evidence — the local runner's subprocess returns
and its output is capturable, the compose runner's does not return at all.

### Found while fixing, not fixed here

`docker_compose.go` calls `execCompose` inside `for _, c := range cmds`. Since `syscall.Exec`
never returns on success, only the **first** command of a multi-command step on the compose path
can run — the loop cannot reach a second iteration. Not touched here because it is a different
defect with a different fix, and this task's scope was the `note:` branch.

Since confirmed on the binary and filed as
[TASK-091](../todo/091-compose-steps-stop-after-the-first-command.md) — it is worse than the
reading suggested: the truncation spans *steps* as well as commands, so a two-step compose
interaction runs step one and exits 0 without ever printing step two's label.

## Related

- [TASK-086](../todo/086-parallel-steps-discard-their-note.md) — the mirror image: paths that never print
  the note at all. Fix together.
- [TASK-085](../todo/085-interaction-steps-silently-drop-compose-keys.md) — the third disagreement between
  the runner and provision paths over the same `ProvisionItem` keys. Three now, all found in the
  TASK-083 audit; worth asking whether the two paths should share one step executor rather than
  being reconciled key by key.
