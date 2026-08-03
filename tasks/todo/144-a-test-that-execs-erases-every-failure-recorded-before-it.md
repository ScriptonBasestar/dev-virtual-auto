---
id: TASK-144
title: "Two in-process runner tests call syscall.Exec, so make test reports ok while tests fail"
type: bug
priority: P1
status: todo
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

- [ ] Neither `inert_step_test.go` nor `kubectl_steps_test.go` can reach `syscall.Exec`
      in the test process. Use the technique already established in
      `compose_steps_test.go`: run in a child process, or point the runner at a binary
      `exec.LookPath` cannot resolve (`compose_steps_test.go:71` does the latter).
- [ ] Prove the masking is gone the way it was found: reintroduce the regression via
      `go test -overlay=`, and show the package now exits non-zero. Print
      `=== RUN` / `--- PASS` / `--- FAIL` counts before and after — a bare `ok` is exactly
      the output this task exists to distrust.
- [ ] Sweep the rest of the test corpus for the same shape: every test that can reach
      `ExecReplace` without a subprocess boundary. State the number found and checked —
      this task was discovered on two paths by exercising them, not by enumeration.
- [ ] The misleading comment at `kubectl_steps_test.go:192-193` is corrected or removed;
      "safe because the code is currently correct" is not a safety argument.
- [ ] Consider a structural guard: a test helper that fails loudly if `ExecReplace` is
      invoked with `testing.Testing()` true, so the next such test cannot be written
      silently. If rejected, record why.
- [ ] `make test` exits 0.

## Notes

Distinct from TASK-136, which covers a `-run` pattern that matches zero tests. That is a
binding naming nothing; this is a binding naming a real test whose result is destroyed
after the fact. Both defeat `verify:` bindings, by opposite mechanisms.
