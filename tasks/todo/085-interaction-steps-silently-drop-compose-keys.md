---
id: TASK-085
title: "Five `ProvisionItem` keys are silently discarded in an interaction step — `dva run` prints nothing and exits 0"
type: fix
priority: P2
effort: M
status: todo
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/runner — local.go, docker_compose.go executeSteps; contrast with internal/cli/provision.go which implements all five keys"
---

# Task 085: an interaction step using anything but `run:` or `note:` does nothing, silently

## Problem

`LocalRunner.executeSteps` and `DockerComposeRunner.executeSteps` handle exactly two things:
`Note` and `RunCommands()`. Every other payload key on `ProvisionItem` falls through their
emptiness check and `continue`s, producing *zero bytes of output* and exit 0.

`RunCommands()` (`config.go`) covers only `Raw` and `Run` — it does **not** fold in `Echo` or
`Cmd` — so the legacy keys are dropped by the same gap.

A field-by-path sweep, counted as `step.X`/`s.X` field accesses (a raw `.Cmd` grep is useless
here: 32 of docker_compose.go's hits are `r.Cmd`, the runner's own field):

| key | local | docker_compose | provision | hooks | compose.go |
| --- | --- | --- | --- | --- | --- |
| `run:` (via `RunCommands()`) | 1 | 1 | 3 | 1 | 1 |
| `note:` | 2 | 2 | 2 | 2 | **0** |
| `compose_up:` | **0** | **0** | 6 | 2 | **0** |
| `compose_exec:` | **0** | **0** | 6 | 2 | **0** |
| `compose_run:` | **0** | **0** | 6 | 2 | **0** |
| `echo:` | **0** | **0** | 8 | **0** | **0** |
| `cmd:` | **0** | **0** | 9 | **0** | **0** |

Negative control: a non-existent `.NoSuchField` scores 0 everywhere, so the counting method can
report absence.

Measured end-to-end on 0.1.44, one fixture per row:

| the item | invoked as | output | exit |
| --- | --- | --- | --- |
| `step:` + `run:` (control) | `dva run control` | label + `CTRL-RAN`, 36 bytes | 0 |
| `step:` + `compose_up: [postgres]` | `dva run viacompose` | **0 bytes** | **0** |
| the same `compose_up` item | `dva provision --dry-run` | `[dry-run] $ docker compose -f … up -d postgres` | 0 |
| `cmd: "echo VIACMD-RAN"` | `dva run viacmd` | **0 bytes** | **0** |
| the same `cmd:` item | `dva provision legacycmd` | `$ echo PROV-CMD-RAN` + its output | 0 |
| `echo: "VIAECHO-SHOWN"` | `dva run viaecho` | **0 bytes** | **0** |
| the same `echo:` item | `dva provision legacyecho` | `PROV-ECHO-SHOWN` | 0 |

Every odd row is the defect; every even row is the control proving the same key works on the
provision path. The `run:` control proves the runner itself executes. Not even the step label
prints in the failing cases.

```yaml
interaction:
  viacompose:
    steps:
      - step: "start db via compose_up"
        compose_up: [postgres]      # dva run viacompose → no output, exit 0
  viacmd:
    steps:
      - cmd: "echo VIACMD-RAN"      # dva run viacmd → no output, exit 0
```

Two substring traps make a naive grep useless here, both hit while measuring this: `ComposeRun`
matches inside `Docker**ComposeRun**ner`, and `.Cmd` matches `r.Cmd`. Match on the qualified field
access (`step.X`), and read the matched line before believing the count.

## Why TASK-083 did not cover this

[TASK-083](../done/083-a-step-without-run-announces-work-it-never-does.md) made a step with **no payload**
report itself. A `compose_up` item *has* a payload, so `ProvisionItem.IsInert()` correctly returns
false and no notice fires — reporting it as "a label with no run:" would be a false statement
about a config that is legitimate and works elsewhere. The two are the same defect class (silent
no-op, exit 0) with opposite causes: 083 was a config mistake nobody reported, this is a config
*correctness* that one execution path never implemented.

## Proposed fix

Two directions; pick one, do not do both.

1. **Implement the five keys in both runners**, matching `provision.go`'s behaviour. Highest
   fidelity — the same YAML then means the same thing wherever it appears — but it moves compose
   invocation into `internal/runner`, which currently has no compose dependency in `LocalRunner`.
   `echo:` and `cmd:` cost nothing here: they are a print and a shell command, with no new
   dependency at all, so they can land even if the compose keys go the other way.
2. **Reject them at validation time on the interaction path.** If these keys are only meaningful
   under `provision:`, say so: a `validate` error naming the key and the supported location. Cheap
   and honest, but it makes a currently-"valid" config fail, and `schema.json` allows the keys in
   both places because `provision_item` is one definition shared by both.

Direction 1 is the better default — the fixture in
`internal/integration/testdata/fixtures/provision-profiles/dva.yml:27-32` shows the
`step`+`parallel`+`compose_up` shape is intended usage — but 2 is defensible if compose on the
interaction path is genuinely out of scope for the runners.

Whichever is chosen, the deeper question is worth answering once: this is the fourth key-by-key
disagreement between the runner and provision paths ([TASK-086](086-parallel-steps-discard-their-note.md),
[TASK-089](../done/089-note-suppresses-run-on-the-interaction-path-only.md), and this task's two groups).
Reconciling them one key at a time is how the count got to four. A shared step executor would
close the class.

## Non-goals

- Not changing `provision.go`, which already behaves correctly.
- Not touching `IsInert`. These are payloads; the classification is correct — `IsInert` already
  checks `Echo` and `Cmd` explicitly — and TASK-083's tests pin it.

## Acceptance criteria

- [ ] An interaction step with `compose_up` either runs or is rejected — never silent | verify: `human — run the fixture below; state which direction was chosen`
- [ ] `cmd:` and `echo:` in an interaction step are no longer 0 bytes | verify: `dva run viacmd` and `dva run viaecho` on the fixture — print the byte count of each; both are 0 today
- [ ] Both runners agree | verify: `go test ./internal/runner/ -run TestComposeKeysOnInteractionPath`
- [ ] The `run:` control still executes unchanged | verify: `go test ./internal/runner/ -run TestStepWithoutRunIsReported` — must stay at 9 passing subtests
- [ ] `provision`'s handling is untouched | verify: `go test ./internal/cli/ -run Provision` — print the count of tests selected, must be non-zero
- [ ] Every `examples/*.yml` still validates | verify: `for f in examples/*.yml; do …; done` — print files swept AND failures; expect 16 and 0, with a deliberately broken control proving the sweep can fail
- [ ] Not vacuous | verify: `human — revert the change and confirm the new test fails`
- [ ] Full suite passes | verify: `make test`

## Reproduction fixture

```yaml
version: "0.1.44"
interaction:
  control:
    steps:
      - step: "control: modern run key"
        run: "echo CTRL-RAN"
  viacompose:
    steps:
      - step: "start db via compose_up"
        compose_up: [postgres]
  viacmd:
    steps:
      - cmd: "echo VIACMD-RAN"
  viaecho:
    steps:
      - echo: "VIAECHO-SHOWN"
provision:
  default:
    - step: "start db via compose_up"
      compose_up: [postgres]
  legacycmd:
    - cmd: "echo PROV-CMD-RAN"
  legacyecho:
    - echo: "PROV-ECHO-SHOWN"
```

Each `dva run` target above produces 0 bytes and exit 0 except `control`. Each `dva provision`
profile carrying the same key works. Same keys, same file.

## Related

- [TASK-083](../done/083-a-step-without-run-announces-work-it-never-does.md) — same class, opposite cause;
  found while measuring its call sites.
- [TASK-086](086-parallel-steps-discard-their-note.md) — the other half of the same survey: the
  parallel provision path drops `note:`.
