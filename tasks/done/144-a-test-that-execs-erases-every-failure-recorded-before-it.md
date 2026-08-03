---
id: TASK-144
title: "Two in-process runner tests call syscall.Exec, so make test reports ok while tests fail"
type: bug
priority: P1
status: done
effort: M
created-at: 2026-08-03T13:20:00+09:00
source: "TASK-091 and TASK-094 finalize verification — found independently by both"
depends-on: [TASK-091, TASK-094]
scope: "dva repo — internal/runner/inert_step_test.go, internal/runner/kubectl_steps_test.go"
---

# Task 144: Stop tests from exec'ing away the suite's verdict

## Problem

`ExecReplace` calls `syscall.Exec` (`internal/exec/exec.go:29`), which **replaces the
running process image**. When a test invokes it in-process, the test binary is gone — and
with it every `--- FAIL` recorded so far. `go test` then reads the exit status of whatever
program took over and reports the package as `ok`.

Reduced to its smallest form and measured 2026-08-03 (standalone module, two tests):

```
TestAaaFails  → t.Errorf("this is a genuine failure")
TestZzzExecs  → syscall.Exec("/usr/bin/true", …)

$ go test ./...                       # both tests
ok      maskproof       0.323s        # rc=0 — the failure is simply gone

$ go test -run TestAaaFails ./...     # the same failing test, alone
    mask_test.go:11: this is a genuine failure and must be reported
FAIL    maskproof       0.145s
```

The repo already knows this hazard. `internal/runner/compose_steps_test.go:32-35`:

> This test MUST run the runner in a child process, and that is not incidental. An
> in-process version of it is worse than no test at all: when the regression is present,
> syscall.Exec … `go test` reports `ok`.

Two tests do it anyway, both against a resolvable `echo`:

- `internal/runner/inert_step_test.go:91` — subtest "a compose_up item is a payload, not
  an inert label" calls `executeSteps(env, steps)` in-process.
- `internal/runner/kubectl_steps_test.go:194` — `TestKubectlStepsAddressTheContainer`,
  whose own doc comment at `:192-193` claims it is "Safe in-process: … the regressions it
  would catch do not replace the process". That is true only while the code under test is
  already correct — which is the one assumption a test may not make.

## Measured blast radius

Both verifiers reproduced the masking with `go test -overlay=` reintroducing `ExecReplace`
into the steps path:

- compose path (TASK-091's guard restored as a bug): 3 `--- FAIL` lines emitted,
  package exit 0, bare `PASS` count 0.
- kubectl path (TASK-094's guard restored as a bug): `TestKubectlStepsRunToCompletion`
  genuinely FAILs, then `TestKubectlStepsAddressTheContainer` execs the shim; result is
  69 `=== RUN`, 66 `--- PASS`, **3 tests never reported**, package exit 0.

In both cases `go test ./internal/runner/ -run Steps` — the criterion binding those tasks
rely on — also exits 0. So `make test` (`Makefile:37`, `go test -race -cover ./...`) would
ship the regression green.

## Acceptance criteria

- [x] Neither `inert_step_test.go` nor `kubectl_steps_test.go` can reach `syscall.Exec`
      in the test process. Satisfied by the structural guard below rather than per-test
      child-process conversion — see "Resolution" for why that is strictly stronger than
      the two suggested techniques and needs no refactor of the table loop or shim coupling.
- [x] Prove the masking is gone the way it was found — see "Overlay proof".
- [x] Sweep the rest of the test corpus — see "Corpus sweep" (10 sites, 8 already safe, 2
      flagged; both flagged are the two this task names).
- [x] The misleading comment at `kubectl_steps_test.go:192-193` is corrected; the new text
      names the guard as the actual basis for in-process safety.
- [x] Structural guard accepted and implemented (not rejected).
- [x] `make test` exits 0.

## Resolution

A single structural guard in `dvaexec.ExecReplace` (`internal/exec/exec.go`) satisfies both
criterion 1's goal and criterion 5. It sits AFTER `exec.LookPath` (so the five tests that point
at an unresolvable binary still get their expected "command not found") and BEFORE `syscall.Exec`:

```go
if testing.Testing() && os.Getenv("DVA_EXEC_REPLACE_OK") != "1" {
    panic("dvaexec.ExecReplace reached under `go test` without a subprocess boundary …")
}
```

Why this over per-test child-process conversion of the two flagged sites:

- The two sites (`inert_step_test.go` compose_up subtest; `kubectl_steps_test.go`
  `TestKubectlStepsAddressTheContainer`) reach `ExecReplace` only under a regression — today the
  steps path uses `ExecSubprocess`. Converting the compose_up subtest to a child means extracting
  it from the all-runner table loop it shares with `Run:` assertions; converting the kubectl test
  to a child means threading the shim's invocation-log path across processes. Both are real
  refactors that buy detection of one regression each.
- The guard buys detection of EVERY such regression, present and future, in three lines, and
  makes "the next such test cannot be written silently" literal: an in-process reach now panics
  unless the author sets `DVA_EXEC_REPLACE_OK=1`, which is a visible, reviewable opt-in.
- The corpus sweep confirms the guard breaks nothing: the only in-process caller that genuinely
  needs the replacement is `dva ktl`, which exec's kubectl as its whole purpose. Its child test
  (`ktl_flag_passthrough_test.go`) now sets `DVA_EXEC_REPLACE_OK=1` with a comment stating why.

This deviates from criterion 1's *named* techniques (child process / unresolvable binary) but not
from its *requirement* ("can reach `syscall.Exec`" = false: the guard fires first). Recorded here
for review.

## Corpus sweep

10 test sites that exercise a path ending in `ExecReplace`, or the steps path that would reach it
under a regression, were examined. The dangerous shape — in-process, resolvable binary — was the
two this task names; every other site was already protected by an established technique. (Counts
corrected on review: `note_run_test.go` calls `executeSteps` → `ExecSubprocess` and does NOT reach
`ExecReplace`; four sites, not five, rely on `LookPath` failing.)

| site | reaches ExecReplace? | protection |
| --- | --- | --- |
| `internal/exec/exec_test.go:200` | yes (direct) | unresolvable binary |
| `internal/exec/exec_test.go:211` | yes (direct) | unresolvable binary |
| `internal/cli/execution_paths_test.go:386` (`_ExecReplaceMode`) | yes (`forceSubprocess=false`) | unresolvable compose command |
| `internal/cli/execution_paths_test.go:467` (`_Debug`) | yes (`forceSubprocess=false`) | unresolvable compose command |
| `internal/cli/ktl_flag_passthrough_test.go` | yes (child) | child process + `DVA_EXEC_REPLACE_OK=1` |
| `internal/cli/root_flag_passthrough_test.go` | no (`forceSubprocess=true`→ExecSubprocess) | seam keeps Go process alive |
| `internal/runner/note_run_test.go` | no (`executeSteps`→ExecSubprocess) | unresolvable compose binary still errors the subprocess |
| `internal/runner/compose_steps_test.go` | only under regression (child) | child process detects missing markers |
| `internal/runner/kubectl_steps_test.go:93` (`RunToCompletion`) | only under regression (child) | child process detects missing markers |
| `internal/runner/inert_step_test.go:91` (compose_up) | only under regression (in-process) | **now guarded** |
| `internal/runner/kubectl_steps_test.go:194` (`AddressTheContainer`) | only under regression (in-process) | **now guarded** |

Integration tests (`internal/integration/`) reach neither `Execute` nor `ExecReplace`.

## Review

Independent `core:code-reviewer` pass: **"Ship it"** — no Critical/High. Two Medium findings,
both stale-on-arrival claims in the guard comment, applied before commit:

- **M1** — the comment claimed "Five tests" rely on `LookPath` failing; the true count is four
  (`note_run_test.go` calls `executeSteps`→`ExecSubprocess`, not `ExecReplace`). Corrected to
  "Four tests" and the sweep table gained a `reaches ExecReplace?` column to make the distinction
  explicit.
- **M2** — "No such test exists today" was false on landing, since this patch adds the ktl child
  as the first opt-in user. Reframed to name the ktl child as the sole legitimate caller.

One **Low/out-of-scope** suggestion recorded, not fixed here: `TestExecComposePassthrough_Debug`
(`execution_paths_test.go:467`) flips `forceSubprocess = false` without restoring it, leaking
global state to later tests in `internal/cli`. Pre-existing; the guard now makes such a leak
louder (a later in-process `ExecReplace` with a resolvable binary would panic instead of silently
passing), but the leak itself should still be fixed. Candidate for a follow-up task.

Integration tests (`internal/integration/`) reach neither `Execute` nor `ExecReplace`.

## Overlay proof

Regression reintroduced by `-overlay`: `internal/runner/compose.go` rewritten so `execComposeStep`
calls `execCompose` (ExecReplace) instead of `ExecSubprocess` — TASK-091's defect. Overlay lives in
`tmp/` (gitignored). `go test -overlay=tmp/regression-overlay.json -v ./internal/runner/`:

| | exit | final | `=== RUN` | `--- PASS` | `--- FAIL` |
| --- | --- | --- | --- | --- | --- |
| **before** (no guard) | **0** | **`ok`** | 38 | 30 | 3 (masked — 5 RUNs never reported) |
| **after** (guard) | **1** | **`FAIL`** | 38 | 32 | 8 (guard fired; nothing masked) |

Before: three genuine `--- FAIL` lines emitted, yet the package reports `ok` because the
compose_up subtest's in-process exec replaced the test binary with `echo up -d postgres` (exit 0)
mid-stream. After: the guard panics at that reach, the process is never replaced, every failure
is reported, exit non-zero.

## Notes

Distinct from TASK-136, which covers a `-run` pattern that matches zero tests. That is a
binding naming nothing; this is a binding naming a real test whose result is destroyed
after the fact. Both defeat `verify:` bindings, by opposite mechanisms.
