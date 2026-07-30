---
id: TASK-085
title: "`compose_up`/`compose_exec`/`compose_run` in an interaction step are silently discarded — `dva run` prints nothing and exits 0"
type: fix
priority: P2
effort: M
status: todo
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/runner — local.go, docker_compose.go executeSteps; contrast with internal/cli/provision.go which implements all three keys"
---

# Task 085: an interaction step with a compose key does nothing, silently

## Problem

`ProvisionItem` carries `compose_up`, `compose_exec` and `compose_run`. `internal/cli/provision.go`
implements all three. **Neither runner does.** `LocalRunner.executeSteps` and
`DockerComposeRunner.executeSteps` handle `Note` and `RunCommands()` and nothing else, so a step
carrying only a compose key falls through their emptiness check and `continue`s.

The result is a step that produces *zero bytes of output* and exits 0.

Measured on 0.1.44 with one fixture holding all three shapes:

| the item | invoked as | output | exit |
| --- | --- | --- | --- |
| `step:` + `run: "echo CONTROL-RAN"` | `dva run viarun` | `→ a control that definitely runs` / `CONTROL-RAN` | 0 |
| `step:` + `compose_up: [postgres]` | `dva run viacompose` | **0 bytes** | **0** |
| the same `compose_up` item | `dva provision --dry-run` | `[dry-run] $ docker compose -f … up -d postgres` | 0 |

Rows 1 and 3 are the controls: the runner path executes `run:` fine, and the provision path
executes the very same `compose_up` item fine. Only the combination — a compose key on the
interaction path — vanishes. Not even the step label prints.

```yaml
interaction:
  viacompose:
    steps:
      - step: "start db via compose_up"
        compose_up: [postgres]      # dva run viacompose → no output, exit 0
```

Verified by reading both loops: `sed -n '54,100p' internal/runner/local.go` and the equivalent in
`docker_compose.go` contain **zero** references to `ComposeUp`/`ComposeExec`/`ComposeRun`. (A
naive grep appears to find one in `docker_compose.go`; it is the substring `ComposeRun` inside
`DockerComposeRunner`. Match on the field access, not the name.) The positive control is
`internal/cli/provision.go`, which references `ComposeUp` 6 times, and `internal/cli/hooks.go`,
which references it twice.

## Why TASK-083 did not cover this

[TASK-083](../done/083-a-step-without-run-announces-work-it-never-does.md) made a step with **no payload**
report itself. A `compose_up` item *has* a payload, so `ProvisionItem.IsInert()` correctly returns
false and no notice fires — reporting it as "a label with no run:" would be a false statement
about a config that is legitimate and works elsewhere. The two are the same defect class (silent
no-op, exit 0) with opposite causes: 083 was a config mistake nobody reported, this is a config
*correctness* that one execution path never implemented.

## Proposed fix

Two directions; pick one, do not do both.

1. **Implement the three keys in both runners**, matching `provision.go`'s behaviour. Highest
   fidelity — the same YAML then means the same thing wherever it appears — but it moves compose
   invocation into `internal/runner`, which currently has no compose dependency in `LocalRunner`.
2. **Reject them at validation time on the interaction path.** If a compose key is only meaningful
   under `provision:`, say so: a `validate` error naming the key and the supported location. Cheap
   and honest, but it makes a currently-"valid" config fail, and `schema.json` allows the key in
   both places because `provision_item` is one definition shared by both.

Direction 1 is the better default — the fixture in
`internal/integration/testdata/fixtures/provision-profiles/dva.yml:27-32` shows the
`step`+`parallel`+`compose_up` shape is intended usage — but 2 is defensible if compose on the
interaction path is genuinely out of scope for the runners.

## Non-goals

- Not changing `provision.go`, which already behaves correctly.
- Not touching `IsInert`. A compose key is a payload; that classification is correct and TASK-083's
  tests pin it.

## Acceptance criteria

- [ ] An interaction step with `compose_up` either runs or is rejected — never silent | verify: `human — run the fixture below; state which direction was chosen`
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
  viacompose:
    steps:
      - step: "start db via compose_up"
        compose_up: [postgres]
provision:
  default:
    - step: "start db via compose_up"
      compose_up: [postgres]
```

`dva run viacompose` → nothing, exit 0. `dva provision --dry-run` → shows the docker command.
Same item, same file.

## Related

- [TASK-083](../done/083-a-step-without-run-announces-work-it-never-does.md) — same class, opposite cause;
  found while measuring its call sites.
- [TASK-086](086-parallel-steps-discard-their-note.md) — the other half of the same survey: the
  parallel provision path drops `note:`.
