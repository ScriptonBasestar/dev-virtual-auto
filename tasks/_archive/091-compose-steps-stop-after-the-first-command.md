---
id: TASK-091
title: "On the compose path only the first command of an interaction ever runs — syscall.Exec replaces the process, so every later step is silently skipped with exit 0"
type: fix
priority: P1
effort: M
status: done
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/runner/compose.go execCompose → ExecReplace; internal/runner/docker_compose.go executeSteps loop. Contrast internal/runner/local.go:44,50,88 which already distinguishes the two cases"
verified-at: 2026-08-03T13:20:00+09:00
archived-at: 2026-08-03T13:20:00+09:00
verification-summary: |
  Fix is real and end-to-end verified on ./bin/dva against fixtures under the scratchpad.
  compose.go now splits composeArgv (shared argv) from execCompose (ExecReplace, single
  command, called only at docker_compose.go:39) and execComposeStep (ExecSubprocess, called
  from the steps loop at docker_compose.go:74). `dva run composesteps` prints both labels and
  both markers; `composetwo` runs both commands; the `false`-backed failfast fixture exits 1,
  names the step and the exact command, and never starts step two. All four compose rows now
  match their local controls. TestComposeStepsRunToCompletion (compose_steps_test.go:41) passes
  with 3 subtests, and its non-vacuity was re-proven independently: in a scratchpad copy of HEAD
  with ExecReplace restored, 2 of 3 subtests fail. Task-file line references (docker_compose.go:55,
  :101) have drifted to :39 and :74 through TASK-129/132; the symbols are intact.
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

[TASK-085](../_archive/085-interaction-steps-silently-drop-compose-keys.md),
[TASK-086](../_archive/086-parallel-steps-discard-their-note.md) and
[TASK-089](../_archive/089-note-suppresses-run-on-the-interaction-path-only.md) each drop *one key*.
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

- [x] A two-step compose interaction runs both steps | verify: `dva run composesteps` on the fixture — print both marker counts; both must be 1, today 1 and 0 — **1 and 1**
- [x] A two-command compose step runs both commands | verify: `dva run composetwo` — same, today 1 and 0 — **1 and 1**
- [x] Every step's label is printed | verify: same run — `step two` must appear in the output; today it does not — **count 1**
- [x] The single-command path still replaces the process | verify: `human — confirm dva run <a service: interaction with command:> still hands over the tty; ExecReplace must remain at docker_compose.go:55` — **`docker_compose.go:55` still calls `execCompose`, and `compose.go` holds exactly one `ExecReplace` (line 61) against one `ExecSubprocess` (line 70)**
- [x] A failing step still aborts the sequence with a non-zero exit | verify: fixture step 1 returns non-zero — dva must exit non-zero and not run step 2 — **exit 1, `STEP-TWO-LABEL` count 0, error names the step and the exact command**
- [x] Covered by a test | verify: `go test ./internal/runner/ -run TestComposeStepsRunToCompletion` — **3 subtests, all passing**
- [x] Not vacuous | verify: `human — restore ExecReplace in the steps path and confirm only the new test fails` — **restored: 2 of the 3 subtests failed with the child's truncated output attached; the whole runner package was otherwise green. See "Why the test re-executes itself" below — the first attempt at this test was vacuous**
- [x] TASK-089's tests still pass | verify: `go test ./internal/runner/ -run 'TestNoteDoesNotSuppressRun|TestStepWithoutRunIsReported'` — 8 + 9 subtests — **21 PASS lines = 15 leaves + 4 group parents + 2 tops, i.e. the same 9 + 8 as before**
- [x] Full suite passes | verify: `make test` — **all packages ok under `-race`; 0 FAIL lines in `internal/runner`**

## Resolution

`compose.go` now separates *what to run* from *how to hand off*. `composeArgv` builds the argv;
`execCompose` ends in `ExecReplace` and `execComposeStep` in `ExecSubprocess`. The steps loop in
`docker_compose.go:101` calls the latter. Single-command behaviour at `docker_compose.go:55` is
untouched.

Measured after the fix — the compose rows now match their local controls exactly:

| invocation | path | 1st marker | 2nd marker | `step two` label | exit |
| --- | --- | --- | --- | --- | --- |
| `dva run localsteps` (control) | local | 1 | 1 | 1 | 0 |
| `dva run composesteps` | compose | 1 | **1** | **1** | 0 |
| `dva run localtwo` (control) | local | 1 | 1 | n/a | 0 |
| `dva run composetwo` | compose | 1 | **1** | n/a | 0 |
| `dva run failfast` (compose command = `false`) | compose | — | — | 0 | **1** |

The last row matters as much as the others: "runs to completion" must not mean "swallows errors".
Step one fails, dva exits 1, the error names the step *and* the exact command, and step two never
starts.

### Why the test re-executes itself

**The first version of this test was vacuous, and it passed.** It drove `executeSteps` in-process
against `echo`. With `ExecReplace` restored — the bug fully present — `go test` printed `ok`. The
verbose run showed why: `=== RUN` for the first subtest, then `echo`'s output, then `ok`, with
**no PASS or FAIL line anywhere**. `syscall.Exec` had replaced the *test binary* with `echo`,
which exited 0, and `go test` read that exit status as success.

This generalises, and it is the reason worth keeping: **any in-process Go test that lets
`ExecReplace` succeed silently passes, no matter what it asserts.** The assertions are never
reached; there is no stack left to reach them with.

`compose_steps_test.go` therefore re-executes its own binary with a mode variable
(`DVA_COMPOSE_STEPS_CHILD`) and asserts on the child's captured output. The destruction happens in
the child; the parent survives to observe that the output simply stops early. With the bug
restored the parent now reports a real failure and prints the truncated child output as evidence:

```
  → step one
compose exec app STEP-ONE-MARKER      ← and nothing further; no PASS, no FAIL, exit 0
```

The third subtest stays in-process deliberately: it points the compose command at an unresolvable
binary, so `exec.LookPath` fails *before* any replacement can occur, and fail-fast semantics can be
asserted directly. TASK-089's test uses the same technique for the same reason.

## Left open

- **`KubectlRunner` never reads `Steps` at all.** Measured as a code reading, not end-to-end:
  `kubectl.go` scores **0** for `Cmd.Steps` and **0** for `executeSteps`, against 2 and 3 in
  `local.go` and 2 and 5 in `docker_compose.go` (negative control `.NoSuchField` = 0 everywhere).
  So an interaction with `pod:` and `steps:` appears to discard the steps entirely. Not confirmed
  on the binary because `kubectl` is on PATH here and the probe would contact a real cluster —
  it needs a fixture that cannot reach one. File separately once measured.

## Related

- [TASK-089](../_archive/089-note-suppresses-run-on-the-interaction-path-only.md) — found while writing
  its test, which could not let the compose runner reach execution for exactly this reason.
- [TASK-085](../_archive/085-interaction-steps-silently-drop-compose-keys.md) — the same silent-drop family.
