---
id: TASK-094
title: "`KubectlRunner.Execute` never reads `Steps`, so `runner: kubectl` + `steps:` runs none of them and nothing warns"
type: fix
priority: P2
effort: M
status: todo
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/runner/kubectl.go — 0 references to Steps in the whole file; internal/config/validate_warnings.go:722-741 treats HasSteps() as callable regardless of runner"
---

# Task 094: the third runner in the same three-runner family still drops `steps:`

## Problem

`internal/runner/kubectl.go` contains **zero** references to `Steps`. Its siblings both branch
on it as their first statement:

| runner | `Steps` references | first branch |
| --- | --- | --- |
| `internal/runner/local.go` | 2 | `if len(cmd.Steps) > 0 { return r.executeSteps(...) }` |
| `internal/runner/docker_compose.go` | 2 | `if len(cmd.Steps) > 0 { return r.executeSteps(...) }` |
| `internal/runner/kubectl.go` | **0** | — no such branch, and no `executeSteps` method exists |

Negative control: `grep -c 'Cmd.NoSuchField' internal/runner/kubectl.go` → 0, so the 0 above is a
real absence and not a broken pattern.

This is the same defect class as [TASK-085](../done/085-interaction-steps-silently-drop-compose-keys.md) and
[TASK-089](../done/089-note-suppresses-run-on-the-interaction-path-only.md), which both resolved
by implementing the keys in every runner rather than by documenting the gap.

## What actually happens

For an interaction with `pod:` set, `steps:` non-empty and `command:` empty, `Execute` builds
`["exec", "--tty", "--stdin", <pod>, "--"]` and hands it to `dvaexec.ExecReplace`, which
`syscall.Exec`s. `Command` is empty so the `if cmd != ""` append is skipped, and `commandArgs`
returns nil. The process is replaced by `kubectl exec <pod> --` with **nothing after `--`**.

Every command under `steps:` is discarded. dva prints nothing about them and asserts nothing —
whatever kubectl then does with an empty command is kubectl's own validation, not dva's.

## Nothing catches it at config time

- The JSON schema has no `not`/`oneOf` pairing `pod` with `steps` — `schema.json`'s `pod`
  property is a plain string.
- `warnUnreachableCommands` (`validate_warnings.go:722-741`) treats `cmd.HasSteps()` as
  sufficient to call a command reachable, **regardless of which runner will run it** — so this
  config is affirmatively classified as fine.
- `warnInertProvisionSteps` only fires for a labelled step with no payload; these steps have real
  `run:` commands, so they are not inert.

`dva validate` therefore exits 0 on a config whose declared work will never run.

## Existing coverage

None. `internal/runner/runner_test.go:123-129` asserts only that `ResolvedCommand{Pod: …}`
dispatches to `*KubectlRunner`; it never sets `Steps` and never calls `Execute`. The three tables
that exercise step behaviour (`note_run_test.go`, `inert_step_test.go`, `step_keys_test.go`) each
build a `runners` map of exactly two entries — `LocalRunner.executeSteps` and
`DockerComposeRunner.executeSteps` — because there is no third method to add.

## Options

- **A — give `KubectlRunner` an `executeSteps`,** matching the siblings. Each step becomes its own
  `kubectl exec`, using `ExecSubprocess` rather than `ExecReplace` for the same reason
  [TASK-091](../done/091-compose-steps-stop-after-the-first-command.md) gave: `syscall.Exec` does
  not return, so it cannot be called in a loop.
- **B — reject `pod:` + `steps:` at validate time.** Cheaper, but it removes a combination users
  can reasonably expect to work and that both other runners support.

A is consistent with how TASK-085 and TASK-089 were resolved. **Decision needed** before
implementing — see Related.

## Testing without a cluster

`kubectl` is on PATH here and points at a **real cluster**, so no criterion may invoke it
directly. Two safe routes:

1. Unit-level: assert `KubectlRunner` has an `executeSteps` and that it is reached, the way
   `step_keys_test.go` already does for the other two runners — no process is spawned.
2. End-to-end: `internal/exec/exec.go` resolves the binary by name through `exec.LookPath`, so a
   directory containing a harmless `kubectl` shim placed first on `PATH` intercepts the call
   before the real binary. Fixture already written at
   `scratchpad/t092-kubectlsteps/` (dva.yml + `fake-bin/kubectl`), not yet executed.

## Acceptance criteria

- [ ] Steps run under the kubectl runner | verify: `go test ./internal/runner/ -run Steps` — the runners table must have 3 entries, not 2; print the count
- [ ] Each step is a separate exec | verify: PATH-shadowed shim log must show one invocation per step, and print the number of lines
- [ ] Not a loop over `syscall.Exec` | verify: `grep -c ExecReplace internal/runner/kubectl.go` inside any steps loop must be 0
- [ ] The empty-command exec is gone | verify: human — a `pod:` + `steps:` config must never produce argv ending in a bare `--`
- [ ] Existing kubectl dispatch is unchanged | verify: `go test ./internal/runner/ -run TestNewRunnerSelection`
- [ ] Not vacuous | verify: human — revert the runner hunk alone and confirm the new test fails
- [ ] Full suite passes | verify: `make test`

## Reproduction

```yaml
version: "1"
interaction:
  seed:
    runner: kubectl
    pod: myapp-0
    steps:
      - step: load fixtures
        run: psql -f fixtures.sql
```

`dva validate` → exit 0, no warning. `dva run seed` → `fixtures.sql` never loads.

## Related

- [TASK-085](../done/085-interaction-steps-silently-drop-compose-keys.md) — same class, resolved by
  implementing in every runner. That decision is the precedent for option A.
- [TASK-089](../done/089-note-suppresses-run-on-the-interaction-path-only.md) — same class again,
  one runner behaving differently from the others on a step key.
- [TASK-091](../done/091-compose-steps-stop-after-the-first-command.md) — why a steps loop must
  not use `ExecReplace`.
