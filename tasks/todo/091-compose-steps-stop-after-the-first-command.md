---
id: TASK-091
title: "On the compose path only the first command of an interaction ever runs — syscall.Exec replaces the process, so every later step is silently skipped with exit 0"
type: fix
priority: P1
effort: M
status: todo
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/runner/compose.go execCompose → ExecReplace; internal/runner/docker_compose.go executeSteps loop. Contrast internal/runner/local.go:44,50,88 which already distinguishes the two cases"
---

# Task 091: `steps:` on the compose path runs step one and stops

## Problem

`execCompose` (`compose.go:51`) ends in `dvaexec.ExecReplace`, which is `syscall.Exec` — it
**replaces the running process**. `DockerComposeRunner.executeSteps` calls it from inside two
nested loops:

```go
for i, step := range steps {          // ← never reaches step 2
    ...
    for _, c := range cmds {          // ← never reaches command 2
        if err := execCompose(env, r.Opts.Config, args); err != nil {
```

`syscall.Exec` does not return on success. So neither loop can ever perform a second iteration.
Everything after the first command is unreachable code, and dva exits **0**.

`LocalRunner` already draws the distinction correctly — `ExecReplace` for a single command
(`local.go:50`), `ExecSequential` (a real subprocess) for command lists and steps
(`local.go:44`, `:88`). Replacing the process is the right behaviour for `dva run shell`, where
tty and signal passthrough matter. It cannot be right for a sequence.

## Evidence (measured on 0.1.44 + the TASK-089 fix, one fixture)

The compose command is set to `echo`, so the exec'd process prints its own argv and nothing
touches docker:

| invocation | path | 1st marker | 2nd marker | exit |
| --- | --- | --- | --- | --- |
| `dva run localsteps` — 2 steps (control) | local | 1 | **1** | 0 |
| `dva run composesteps` — 2 steps | compose | 1 | **0** | **0** |
| `dva run localtwo` — 1 step, 2 commands (control) | local | 1 | **1** | 0 |
| `dva run composetwo` — 1 step, 2 commands | compose | 1 | **0** | **0** |

The local rows are the controls: the identical YAML shape runs both halves, so a two-step
interaction is a legitimate config and not an unsupported one. The compose rows drop the second
half in both dimensions — across commands *and* across steps.

The compose output is literally:

```
  → step one
compose exec app sh -c STEP-ONE-CMD
```

That line is `echo` printing its own arguments — direct proof the process was replaced. Note that
`→ step two`'s **label never prints either**, so there is no trace in the output that a second
step existed at all.

Fixture:

```yaml
stack:
  infra:
    order: 1
    default_runner: compose
    runners:
      compose:
        command: "echo"       # harmless stand-in; prints the argv it was exec'd with
        files: []
interaction:
  composesteps:
    service: app              # `service:` routes to DockerComposeRunner
    steps:
      - step: "step one"
        run: "STEP-ONE-CMD"
      - step: "step two"
        run: "STEP-TWO-CMD"   # never runs, never printed, exit 0
  localsteps:                 # CONTROL — same shape, no service:, both steps run
    steps:
      - step: "step one"
        run: "echo STEP-ONE-LOCAL"
      - step: "step two"
        run: "echo STEP-TWO-LOCAL"
```

## Why this outranks the other silent-drop tasks

[TASK-085](085-interaction-steps-silently-drop-compose-keys.md),
[TASK-086](086-parallel-steps-discard-their-note.md) and
[TASK-089](../done/089-note-suppresses-run-on-the-interaction-path-only.md) each drop *one key*.
This drops **all remaining work** in the interaction, no matter how it is written, and reports
success. A provisioning sequence that looks like it completed has actually performed only its
first command.

## Proposed fix

`executeSteps` must not use a process-replacing call. Give `execCompose` a subprocess sibling —
the argv construction is identical, only the final call differs:

```go
return dvaexec.ExecSubprocess(env, composeCmd, fullArgs, false)   // instead of ExecReplace
```

Keep `ExecReplace` on the single-command path (`docker_compose.go:55`), which is where tty
passthrough is wanted and where exactly one command runs. Extract the shared argv building so the
two cannot drift.

The same question should be asked of `internal/cli/compose.go:769,786` and
`internal/cli/kubectl.go:35,67` — four more `ExecReplace` call sites, not audited here.

## Non-goals

- Not changing single-command behaviour. `dva run shell` must keep replacing the process.
- Not changing `LocalRunner`, which is the reference implementation for this distinction.

## Acceptance criteria

- [ ] A two-step compose interaction runs both steps | verify: `dva run composesteps` on the fixture — print both marker counts; both must be 1, today 1 and 0
- [ ] A two-command compose step runs both commands | verify: `dva run composetwo` — same, today 1 and 0
- [ ] Every step's label is printed | verify: same run — `step two` must appear in the output; today it does not
- [ ] The single-command path still replaces the process | verify: `human — confirm dva run <a service: interaction with command:> still hands over the tty; ExecReplace must remain at docker_compose.go:55`
- [ ] A failing step still aborts the sequence with a non-zero exit | verify: fixture step 1 returns non-zero — dva must exit non-zero and not run step 2
- [ ] Covered by a test | verify: `go test ./internal/runner/ -run TestComposeStepsRunToCompletion`
- [ ] Not vacuous | verify: `human — restore ExecReplace in the steps path and confirm only the new test fails`
- [ ] TASK-089's tests still pass | verify: `go test ./internal/runner/ -run 'TestNoteDoesNotSuppressRun|TestStepWithoutRunIsReported'` — 8 + 9 subtests
- [ ] Full suite passes | verify: `make test`

## Left open

- **`KubectlRunner` never reads `Steps` at all.** Measured as a code reading, not end-to-end:
  `kubectl.go` scores **0** for `Cmd.Steps` and **0** for `executeSteps`, against 2 and 3 in
  `local.go` and 2 and 5 in `docker_compose.go` (negative control `.NoSuchField` = 0 everywhere).
  So an interaction with `pod:` and `steps:` appears to discard the steps entirely. Not confirmed
  on the binary because `kubectl` is on PATH here and the probe would contact a real cluster —
  it needs a fixture that cannot reach one. File separately once measured.

## Related

- [TASK-089](../done/089-note-suppresses-run-on-the-interaction-path-only.md) — found while writing
  its test, which could not let the compose runner reach execution for exactly this reason.
- [TASK-085](085-interaction-steps-silently-drop-compose-keys.md) — the same silent-drop family.
