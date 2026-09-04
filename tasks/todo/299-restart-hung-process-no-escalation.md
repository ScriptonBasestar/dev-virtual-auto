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
status: todo
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

- [ ] A regression test reproduces the current gap against pre-fix code (i.e. is genuinely
      falsifiable): a process-plugin entry backed by a helper process that ignores `SIGTERM`
      past `haltExitTimeout`, restarted via the orchestrator's `Restart`, currently exits/reports
      success while the original process is still running and no new process was started
      | verify: `grep -Eq '^func TestProcessPlugin_Restart_HungProcess' internal/lifecycle/process_test.go && go test ./internal/lifecycle -count=1`
- [ ] The chosen direction (from the three options above, or a documented combination) is
      implemented and the regression test above reflects the new, intended behavior for a hung
      process — either it is actually terminated and replaced, or `dva restart`/`dva stop`
      reports a non-zero exit for this case, or both, per the decision made | verify: `human — confirm the implemented behavior matches the direction documented in this card's "Recommended direction" or an explicitly recorded amendment to it`
- [ ] No change to behavior for the common case where the process exits within
      `haltExitTimeout` — `TestProcessPlugin_Restart_LeavesProcessRunning` and
      `TestRunPlanRestartLeavesNativeProcessRunning` (both from TASK-294) continue to pass
      unmodified | verify: `go test ./internal/lifecycle -run TestProcessPlugin_Restart_LeavesProcessRunning -count=1 -v && go test ./internal/cli -run TestRunPlanRestartLeavesNativeProcessRunning -count=1 -v`
- [ ] Repository gates pass | verify: `make lint && make test && make test-integration && make commit-check`

## Non-goals

- Not re-litigating TASK-294's fix for the sub-5s race — that fix is correct and closed; this
  card is only about what happens past the existing `haltExitTimeout` boundary.
- Not picking the escalation strategy in advance — this card documents the tradeoffs; the
  implementer or a human decides which of options (1)/(2)/(3) (or combination) to build.
- Not extending this to the `compose`/`kubectl`/`helm` plugins — their stop/down semantics don't
  go through `haltProcess`; this is process-plugin (native runner) specific, matching TASK-294's
  scope.
