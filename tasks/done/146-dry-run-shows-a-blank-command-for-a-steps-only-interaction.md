---
id: TASK-146
title: "--dry-run prints an empty Command line for a steps-only interaction and names no step"
type: bug
priority: P2
status: done
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

- [x] `dva run <steps-only> --dry-run` lists each step, in order, with what it will execute —
      `explainSteps` prints each step's `run:`, `cmd:`, `compose_up:`, `compose_exec:`,
      `compose_run:` and `echo:` in runStepLoop's dispatch order.
- [x] `Command:` is not printed blank — a steps-only interaction prints
      `Command: (step-driven — see Steps below)`.
- [x] All three runners produce the same shape — `Explain` reads `cmd.Steps` directly and never
      dispatches by runner, so the rendering is runner-independent by construction (the explain
      path does not re-fork what execution unified).
- [x] A `note:` step renders `  → label: note`, the same line the executing path prints.
- [x] A test pins the output (`TestExplainListsStepsForStepDrivenInteraction`) and fails without
      the change — it asserts the `(step-driven)` line, the `Steps:` section, and each step's
      label/run/note, none of which the reverted code prints. The `-run` pattern names a real
      test. Explain never reaches `ExecReplace`, so the TASK-144 in-process hazard does not apply.
- [x] `make test` exits 0.

## Resolution

Added `explainSteps(cmd)` to `internal/runner/runner.go`, called from `Explain`'s text branch
when `len(cmd.Steps) > 0`. It mirrors `runStepLoop`'s labels and dispatch order (compose keys
short-circuit, then `run:`, then `echo:`/`cmd:`) so the dry-run plan shows the declared work
instead of a blank `Command:` line. The `Command:` line itself now switches on the three real
cases: a single command, step-driven, or genuinely empty. JSON `--dry-run` is left unchanged;
the criteria are text-focused, and a JSON `steps` key would be a follow-up of the same shape.

`note:` rendering reuses the exec path's form rather than TASK-141's hook-side `writeNote`
(which splits on newlines for a different surface); the step path — what this mirrors — prints
the single `  → label: note` line.

## Review

Independent `core:code-reviewer` pass: **No Critical; one High caught before finalizing.**

- **H1 (High, fixed)** — `explainSteps` listed every payload independently, so a step carrying
  `compose_up:` AND `run:` printed both, but the exec path short-circuits: `runComposeStepKeys`
  returns handled and the loop skips `run:`/`echo:`/`cmd:`. The dry-run would have told a user
  `run: seed` will execute when it will not — exactly the misreading this task exists to prevent.
  Replaced the independent `if` blocks with a `switch` mirroring `runComposeStepKeys` (first
  compose key wins) plus the short-circuit (compose → no run/echo/cmd).
- **M1 (fixed)** — the first test covered only `run:`/`note:`. Added `TestExplainStepsMirrorDispatch`
  pinning the compose short-circuit (the H1 regression), the inert-step marker, and the `step N`
  label fallback.
- **M2 (deferred)** — JSON `--dry-run` still reports `"command": ""` for steps-only. Same bug,
  JSON surface; out of scope here (criteria are text-focused). Candidate follow-up task.
- L1–L4 (dead `Raw` branch kept for parity with steps.go; substring ordering; wording;
  genuinely-empty `Command:` line preserved for TASK-165's scope) — accepted as-is.

## Notes

Related but out of scope: `warnUnreachableCommands`
(`internal/config/validate_warnings.go:818`) calls `hasExecutionTarget()`, which at
`validate_warnings.go:364-366` counts `HasSteps()` without asking which runner will run it.
TASK-094 recorded this deliberately; it is harmless while all three runners honour steps,
and stops being harmless the moment a fourth runner does not.
