---
id: TASK-299
title: "dva restart is a silent no-op when a process ignores SIGTERM past the 5s wait"
type: bug
priority: P2
effort: S-M
exec-tier: standard
created-at: 2026-09-04T00:00:00+09:00
source: "TASK-294 independent review follow-up — found while confirming the stale-PID restart fix"
scope: "internal/lifecycle/process.go ProcessPlugin.haltProcess only, and its callers (Stop/Restart) as needed for the exit-code option"
status: done
depends-on: []
---

# Task 299: dva restart is a silent no-op when a process ignores SIGTERM past the 5s wait

## Summary

TASK-294 (`tasks/done/294-fix-restart-stale-pid-false-success.md`) fixed `dva restart` reporting
false success on a stale-PID race by making `haltProcess` block, bounded by `haltExitTimeout`
(5s, `internal/lifecycle/process.go`), until the SIGTERM'd process actually exits before `Up`
runs. An independent reviewer confirmed that fix is correct but found a narrower residual gap on
the timeout path itself.

Root cause, confirmed by reading `internal/lifecycle/process.go` (`haltProcess`, ~lines 174-179):

```go
if !waitForProcessExit(ctx, pid, haltExitTimeout) {
    fmt.Fprintf(os.Stderr, "[warn] %s (pid %d) did not exit within %s of stop\n", name, pid, haltExitTimeout)
}
// PID file preserved — process can be restarted by up
return nil
```

When a process doesn't exit within `haltExitTimeout`, `haltProcess` prints a warning and returns
`nil` anyway — `Stop` "succeeds". `Restart` (`Orchestrator.Restart`, Stop then Up) proceeds to
`Up`, whose already-running check reads the still-present PID file, finds the still-live PID via
`IsProcessRunning`, logs `"already running"`, and starts nothing. Net effect: for a process that
ignores SIGTERM for more than 5 seconds, `dva restart` exits 0 without having actually restarted
anything.

This narrows the false-success class TASK-294 fixed — there is now a warning on stderr, and the
old process is left alive rather than silently dead — but does not eliminate it: a caller that
only checks the exit code (scripts, CI) still sees success while nothing happened. Confirmed via
`grep -rn "SIGKILL\|Kill(" internal/lifecycle` that no escalation path exists anywhere in the
process plugin today — `terminateProcessGroup` (`internal/lifecycle/process_group_unix.go`) only
ever sends `SIGTERM`.

## Recommended direction

This is an open choice for whoever implements it — the options trade off differently and none is
obviously correct without a product decision on how `dva` should treat a process that refuses to
stop:

1. **SIGKILL escalation.** After the existing `haltExitTimeout` wait elapses, send `SIGKILL` to
   the process group and do a second, shorter bounded wait for the kernel to actually reap it.
   Pro: `restart`/`stop` become reliable — the entry really is gone or really is running, never an
   ambiguous third state. Con: forcibly kills a process that may have been mid-graceful-shutdown
   (e.g. flushing writes, draining connections) for a caller that expected a plain `dva restart`
   to be non-destructive; also needs a decision on how long the second wait should be.
2. **Make the timeout/escalation behavior configurable** (e.g. a `dva.yml` field or a CLI flag on
   `stop`/`restart`) rather than hardcoding either "always wait and warn" or "always escalate".
   Pro: lets operators opt into SIGKILL for services they know are safe to hard-kill, without
   changing default behavior for everyone else. Con: another knob to document and test; still
   needs a sane default for the no-config case.
3. **At minimum, make the exit code non-zero on this path**, without solving the underlying
   stuck-process case at all. Pro: smallest change, immediately fixes the "CI believes success"
   problem this card is really about, composes with either (1) or (2) if done later. Con: doesn't
   fix the actual operational problem — the operator still has to intervene manually to get the
   entry running again.

These are not mutually exclusive — (3) is compatible with adding (1) or (2) in the same change or
as a follow-up. Whoever picks this up should decide the scope explicitly rather than defaulting
to the first option read here.

## Completion Criteria

- [x] A regression test reproduces the current gap against pre-fix code (i.e. is genuinely
      falsifiable): a process-plugin entry backed by a helper process that ignores `SIGTERM`
      past `haltExitTimeout`, restarted via the orchestrator's `Restart`, currently exits/reports
      success while the original process is still running and no new process was started
      | verify: `grep -Eq '^func TestProcessPlugin_Restart_HungProcess' internal/lifecycle/process_test.go && go test ./internal/lifecycle -count=1`
- [x] The chosen direction (from the three options above, or a documented combination) is
      implemented and the regression test above reflects the new, intended behavior for a hung
      process — either it is actually terminated and replaced, or `dva restart`/`dva stop`
      reports a non-zero exit for this case, or both, per the decision made | verify: `human — confirm the implemented behavior matches the direction documented in this card's "Recommended direction" or an explicitly recorded amendment to it`
- [x] No change to behavior for the common case where the process exits within
      `haltExitTimeout` — `TestProcessPlugin_Restart_LeavesProcessRunning` and
      `TestRunPlanRestartLeavesNativeProcessRunning` (both from TASK-294) continue to pass
      unmodified | verify: `go test ./internal/lifecycle -run TestProcessPlugin_Restart_LeavesProcessRunning -count=1 -v && go test ./internal/cli -run TestRunPlanRestartLeavesNativeProcessRunning -count=1 -v`
- [x] Repository gates pass | verify: `make lint && make test && make test-integration && make commit-check`

## Non-goals

- Not re-litigating TASK-294's fix for the sub-5s race — that fix is correct and closed; this
  card is only about what happens past the existing `haltExitTimeout` boundary.
- Not picking the escalation strategy in advance — this card documents the tradeoffs; the
  implementer or a human decides which of options (1)/(2)/(3) (or combination) to build.
- Not extending this to the `compose`/`kubectl`/`helm` plugins — their stop/down semantics don't
  go through `haltProcess`; this is process-plugin (native runner) specific, matching TASK-294's
  scope.

## Implementation note

Implemented **option 3 only**: when `haltProcess`'s bounded wait for SIGTERM
(`waitForProcessExit`, `haltExitTimeout` = 5s) expires without the process exiting,
`haltProcess` now returns `fmt.Errorf("stop %s: pid %d did not exit within %s", ...)` instead of
`nil` (`internal/lifecycle/process.go`, in the `if !waitForProcessExit(...)` branch), keeping the
existing `[warn] ... did not exit within ...` stderr line unchanged. That error already propagates
correctly with no further wiring needed: `Orchestrator.Stop` collects it via
`errors.Join(stopErrs...)`, and `Orchestrator.Restart` returns immediately on a non-nil `Stop`
error without calling `Up` — so a hung process now makes `dva stop`/`dva restart` exit non-zero
and, for `restart`, also skips starting a (pointless, since the original is still occupying the
PID-file slot) replacement, rather than silently reporting success.

Chose option 3 over 1 (SIGKILL escalation) and 2 (configurable timeout/escalation) for the reason
already recorded on this card when the task was assigned: it is the smallest change that directly
fixes the actual failure mode this card exists for (a caller checking only the exit code believing
`restart` succeeded when nothing happened), it is non-destructive — it never forces a process that
might be mid-graceful-shutdown to die — and it does not foreclose adding 1 or 2 later, since
neither would need to change this error-return point. Options 1 and 2 are deliberately left
unimplemented as follow-up candidates, not attempted here.

Regression test: `TestProcessPlugin_Restart_HungProcess` (`internal/lifecycle/process_test.go`)
drives a real `Orchestrator` against a stack entry backed by
`trap '' TERM; while true; do sleep 0.05; done` — a process that discards SIGTERM outright — and
asserts `Restart` returns an error mentioning "did not exit within", that the original process is
still alive afterward (no undocumented SIGKILL escalation snuck in), and that the PID file still
names the original process (no replacement was started). The test sleeps 150ms after `Up` before
signalling: without it, SIGTERM can race the shell's own startup and arrive before its `trap`
statement is registered, in which case the default disposition applies and the process exits
normally — a false pass for the wrong reason, not exercising the timeout path at all. This was not
a hypothetical concern; it is what the test did before the sleep was added (see below).

Falsifiability check, done twice — once while the test lived in this form, and once more after
moving it from a separate `!windows`-tagged file into `process_test.go` to match this card's
`verify:` command, since that move changed the failure text's file/line: reverted the fix's `return
fmt.Errorf(...)` line only (kept the stderr warning), ran
`go test ./internal/lifecycle -run TestProcessPlugin_Restart_HungProcess -v`, and confirmed it
failed with `Restart: expected a non-nil error for a process that ignores SIGTERM past
haltExitTimeout, got nil` — the pre-fix log shows `[+] started api` printed a second time (`Up`
silently starting nothing new, per the `already running` info log naming the same original pid) as
the observable consequence. Restored the fix and confirmed the same run passes, and that
`TestProcessPlugin_Restart_LeavesProcessRunning` (`internal/lifecycle`, unmodified) and
`TestRunPlanRestartLeavesNativeProcessRunning` (`internal/cli`, unmodified) both still pass —
the sub-`haltExitTimeout` graceful-exit path is untouched by this change.

Files touched: `internal/lifecycle/process.go` (the fix), `internal/lifecycle/process_test.go`
(new test + `os/exec` import for SIGKILL-based test cleanup, since the process under test ignores
SIGTERM), this task card.

## Independent review (2026-09-04)

APPROVED WITH FINDINGS (all MINOR, none blocking) by a separate agent, done in an isolated
detached-HEAD review worktree: end-to-end error propagation traced through `Orchestrator.Stop` ->
`Orchestrator.Restart` -> CLI `RunE` -> `os.Exit(1)`, not just asserted from the diff; the
falsifiability check was independently reproduced by hand; both named non-regression tests
confirmed unmodified and passing; full gate suite re-run clean; scope confirmed as exactly the
three files this card lists. One MINOR finding fixed in this same change: the four Completion
Criteria checkboxes above were left unchecked despite `status: done` (now checked). A second
MINOR finding — a "smart-quote artifact" in `TestProcessPlugin_Restart_HungProcess`'s doc
comment (`` `trap ” TERM` `` where `` `trap '' TERM` `` was expected) — was investigated and is
**not a defect**: this repo's Go 1.26.5 toolchain's `gofmt -s` doc-comment formatter converts an
adjacent `''` pair into `”` inside any comment directly attached to a declaration (verified by
reproducing it in an isolated scratch file, independent of this repo); reverting it to straight
quotes makes the file gofmt-noncompliant and fails `make fmt-check`. Left as originally written.
The remaining MINOR (sleep-based test synchronization is safe-direction but not fully
deterministic; a log-marker poll would remove the residual flakiness risk) was left as a
documented follow-up candidate, not
acted on, since the reviewer confirmed it cannot mask a regression via false-pass.
