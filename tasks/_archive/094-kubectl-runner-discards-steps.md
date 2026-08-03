---
id: TASK-094
title: "`KubectlRunner.Execute` never reads `Steps`, so `runner: kubectl` + `steps:` runs none of them and nothing warns"
type: fix
priority: P2
effort: M
status: done
created-at: 2026-07-31T00:00:00+09:00
closed-at: 2026-07-31T00:00:00+09:00
scope: "internal/runner/kubectl.go — 0 references to Steps in the whole file; internal/config/validate_warnings.go:722-741 treats HasSteps() as callable regardless of runner"
verified-at: 2026-08-03T13:20:00+09:00
archived-at: 2026-08-03T13:20:00+09:00
verification-summary: |
  The fix itself is real and correctly built. KubectlRunner.Execute now branches on Steps
  (internal/runner/kubectl.go:24-26) into executeSteps, which delegates to the extracted shared
  runStepLoop (internal/runner/steps.go:20) and issues one dvaexec.ExecSubprocess per command
  (kubectl.go:66) — never ExecReplace. All three step tables fan out over 3 runners via
  stepRunners (step_keys_test.go:33). Measured on the real binary: `dva validate` exits 0 and
  `dva run seed --dry-run` on the task's repro emits no bare `--`. TestNewRunnerSelection still
  passes with 3 subtests. Probe 1 (delete the Steps branch) was re-run via `go test -overlay=`
  and fails with the exact exit-97 message the task records, so the new test is genuinely
  non-vacuous against the defect TASK-094 targets.
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

This is the same defect class as [TASK-085](../_archive/085-interaction-steps-silently-drop-compose-keys.md) and
[TASK-089](../_archive/089-note-suppresses-run-on-the-interaction-path-only.md), which both resolved
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
  [TASK-091](091-compose-steps-stop-after-the-first-command.md) gave: `syscall.Exec` does
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
   before the real binary.

## Acceptance criteria

- [x] Steps run under the kubectl runner | verify: `go test ./internal/runner/ -run Steps` — the runners table must have 3 entries, not 2; print the count
- [x] Each step is a separate exec | verify: PATH-shadowed shim log must show one invocation per step, and print the number of lines
- [x] Not a loop over `syscall.Exec` | verify: `grep -c ExecReplace internal/runner/kubectl.go` inside any steps loop must be 0
- [x] The empty-command exec is gone | verify: human — a `pod:` + `steps:` config must never produce argv ending in a bare `--`
- [x] Existing kubectl dispatch is unchanged | verify: `go test ./internal/runner/ -run TestNewRunnerSelection`
- [x] Not vacuous | verify: human — revert the runner hunk alone and confirm the new test fails
- [x] Full suite passes | verify: `make test`

## Reproduction

```yaml
version: "0.1.44"
interaction:
  seed:
    runner: kubectl
    pod: myapp-0
    steps:
      - step: load fixtures
        run: psql -f fixtures.sql
```

`dva validate` → exit 0, no warning. `dva run seed` → `fixtures.sql` never loads.

> The repro originally said `version: "1"`, which dva rejects outright
> (`expected MAJOR.MINOR.PATCH or MAJOR.MINOR`) — the fixture never reached the runner at all.
> Corrected on close.

---

## Resolution

Option A, with one deliberate deviation from its literal wording, recorded below.

### Measured before/after, on real binaries

Both binaries were run against the same fixture — a `pod: web` + three-step interaction — with a
`kubectl` shim first on a `PATH` rebuilt as `shim:/bin:/usr/bin`. The shim prints argv one
argument per line, so the boundaries are visible rather than inferred from a space-joined string.
The real `/opt/homebrew/bin/kubectl` is not on that PATH at all.

| | pre-fix (`d39d30e`) | post-fix |
| --- | --- | --- |
| `dva validate` | `✅ dva.yml is valid`, exit 0 | unchanged |
| kubectl invocations | **1** | **3** (one per step) |
| argv | `exec --tty --stdin web --` — 5 args, nothing after `--` | `exec web -- sh -c <cmd>` — 6 args |
| step labels printed | none | `→ load fixtures` / `→ warm the cache` / `→ check health` |
| exit status | 0 | 0 |
| steps actually run | **0 of 3** | 3 of 3 |

The pre-fix run is the whole defect in one line: a single `kubectl exec web --` with an empty
command, exit 0, and no output naming any of the three steps the user declared.

`argv[6]=<psql -f fixtures.sql>` is **one** argument, not three — `sh -c` receives the command
line intact. Checked explicitly, because the space-joined form the shim printed first could not
distinguish the correct grouping from a broken one.

### The deviation: one shared loop instead of a third copy

Option A says "give `KubectlRunner` an `executeSteps`, matching the siblings." Matching them
literally meant a **third** copy of the same ~50-line loop. That duplication is not incidental to
this task — it is its cause. Five tasks, one shape:

| task | what diverged between the copies |
| --- | --- |
| TASK-083 | a label with no `run:` printed nothing |
| TASK-085 | five of seven payload keys were unhandled |
| TASK-089 | `note:` suppressed the run |
| TASK-091 | the loop stopped after the first command |
| TASK-094 | one runner had no loop at all |

So the loop was extracted to `runStepLoop` in the new `internal/runner/steps.go`, and all three
runners now delegate to it, passing only a closure describing how *that* runner hands a command
off. The behavioural contract each runner had is unchanged; what changed is that there is now one
place for it to be true.

Evidence the extraction preserves behaviour: the existing step tables — which exercise both
pre-existing runners across all seven payload keys, note handling, inert labels, and ordering —
pass unmodified. They were not adjusted to fit the refactor; only the two-entry `runners` maps
they built were replaced by a shared `stepRunners` helper that returns three.

`grep -c 'Steps'`: `local.go` 4, `docker_compose.go` 4, `kubectl.go` 5 (was 2 / 2 / **0**).

### Both non-vacuity probes were false passes first

This is the part worth keeping. The criterion "revert the runner hunk alone and confirm the new
test fails" was run twice and **passed vacuously both times** before the tests were fixed.

**Probe 1 — remove the `Steps` branch from `Execute`.** Result: `ok`. The single-command path ends
in `ExecReplace` → `syscall.Exec`, which replaced the *test binary* with the shim; the shim exited
0, and `go test` reported that as the test's own success. Fixed by making the shim `exit 97` when
its argv ends in a bare `--` — precisely the argv the defect produces.

**Probe 2 — swap `ExecSubprocess` → `ExecReplace` inside the new steps loop.** Result: `ok` again.
The `exit 97` guard does not cover this one: the argv is legitimate (`exec web -- psql …`), so the
shim exits 0 and the replaced test binary inherits that status.

The fix for probe 2 was already in this package. `compose_steps_test.go` solves the identical
problem with a child-process re-exec, and its doc comment states the rule outright: *"An in-process
version of it is worse than no test at all."* `TestKubectlStepsRunToCompletion` was rewritten on
that pattern — the parent re-execs the test binary with a mode selector in the environment, so the
replacement happens to a child and the parent survives to judge what it printed and what status it
returned.

Both probes now fail, and the failure output names the mechanism:

```
probe 1: child "execute" exited with exit status 97 — the steps path replaced the process
         instead of returning; output: shim: kubectl exec with no command after --

probe 2: kubectl invoked 1 times, want 3 — one per step; child output was:
           → step one
         kubectl exec myapp-0 -- STEP-ONE-MARKER
```

Probe 2's output is the regression stated exactly: step one runs, the process is replaced, and
steps two and three never come into existence.

### Criterion 1's verify command does not select what it claims

`go test ./internal/runner/ -run Steps` selects `TestComposeStepsRunToCompletion`,
`TestKubectlStepsRunToCompletion` and `TestKubectlStepsAddressTheContainer` — **not**
`TestStepWithoutRunIsReported`, whose name contains `Step`, not `Steps`. The runners table the
criterion refers to lives in that test, so the command as written could never have measured it.

Measured with a pattern that does select them — all three tables, each with all three runners:

```
TestStepWithoutRunIsReported     : 3 entries -> docker_compose kubectl local
TestComposeKeysOnInteractionPath : 3 entries -> docker_compose kubectl local
TestNoteDoesNotSuppressRun       : 3 entries -> docker_compose kubectl local
```

`make test` passes in full, under `-race`.

### Left open, deliberately

- **`DockerComposeRunner.buildStepArgs(env, …)`** — `env` is unused; the diagnostic is
  pre-existing and predates this task. Not touched inside a behaviour fix.
- **`stringsseq` advisory** on `range strings.Split(...)` in the new test. The repo uses
  `strings.Split` at 12 sites and `strings.SplitSeq` at 0; adopting the newer form in one test
  file would make it the outlier. A repo-wide modernization is its own decision, not this fix's.
- **`warnUnreachableCommands` still counts `HasSteps()` as reachable without asking which runner
  will run it.** That is now harmless for kubectl, since every runner honours steps — but the
  validator's reasoning is still runner-blind, and a fourth runner added without a steps path
  would be classified as fine in exactly the same way.

## Related

- [TASK-085](../_archive/085-interaction-steps-silently-drop-compose-keys.md) — same class, resolved by
  implementing in every runner. That decision is the precedent for option A.
- [TASK-089](../_archive/089-note-suppresses-run-on-the-interaction-path-only.md) — same class again,
  one runner behaving differently from the others on a step key.
- [TASK-091](091-compose-steps-stop-after-the-first-command.md) — why a steps loop must
  not use `ExecReplace`.
