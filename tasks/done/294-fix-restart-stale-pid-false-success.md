---
id: TASK-294
title: "Fix restart reporting success while the process pid race leaves it stopped"
type: bug
priority: P2
effort: S
exec-tier: standard
created-at: 2026-09-04T00:00:00+09:00
source: "TASK-292 independent review round 3 (review-task292), non-blocking item 3"
scope: "process plugin restart path only (native runner), both single-plan `dva restart` and composition restart"
status: done
depends-on: []
---

# Task 294: fix restart reporting success while the process pid race leaves it stopped

## Summary

`dva restart <plan>` (and composition `restart`, which is implemented as Stop then Up — see
`internal/lifecycle/orchestrator.go` `Orchestrator.Restart`) can stop a `process`-plugin entry, then
report the restart as successful while the entry is actually left stopped.

Root cause, confirmed by reading `internal/lifecycle/process.go`:

- `ProcessPlugin.Stop` calls `haltProcess`, which sends `SIGTERM` via `terminateProcessGroup(pid)` and
  **deliberately preserves the PID file** ("Vagrant halt semantics" — process can be restarted quickly
  by `up`). `terminateProcessGroup` does not wait for the process to actually exit; SIGTERM delivery and
  the process's own shutdown are asynchronous.
- Immediately afterward, `Orchestrator.Restart` calls `Up`, which re-enters `ProcessPlugin.Up`'s
  "already running" check: it reads the same (still-present) PID file and calls
  `IsProcessRunning(pid)`. If the process has not finished exiting yet — likely for anything with
  graceful-shutdown handling, or just OS scheduling variance — this check still returns true, so `Up`
  logs `"already running"` and returns a `running` status **without starting a new process**.
- The original process then finishes exiting from the SIGTERM shortly after. Net effect: the entry is
  stopped, but `dva restart` exits 0 and reports it as running.

This is a real, reproducible false-success report — not a cosmetic issue — since scripts and CI that
gate on `dva restart`'s exit code will proceed believing the service is up.

## Recommended direction

`Up`'s "already running" check needs to distinguish "still shutting down from a Stop this same
invocation just issued" from "was already running before this invocation started." Two candidate
approaches, in order of preference:

1. Have `haltProcess` wait (bounded, e.g. poll `IsProcessRunning` with a short timeout) for the process
   to actually exit before returning, matching the synchronous stop semantics `Down`/`removeProcess`
   effectively get for free by removing the PID file. This fixes the race at the source and keeps
   `Up`'s existing "already running" short-circuit correct for its actual intended case (an
   out-of-band-running process, not a same-invocation restart).
2. Alternatively, have `Restart` (orchestrator-level) not rely on `Stop`+`Up`'s PID-file
   already-running check at all for the process plugin — e.g. pass a flag through `UpOptions` that
   tells `ProcessPlugin.Up` to skip the already-running short-circuit when called as the second half of
   a restart, since the caller already knows it just stopped this entry.

Prefer (1): it fixes the underlying race for any other caller that stops and immediately checks/starts
the same entry, not just the `Restart` code path, and needs no new option threaded through
`UpOptions`/`PluginContext`.

## Completion Criteria

- [x] A regression test reproduces the bug against pre-fix code (i.e. is genuinely falsifiable, not
      just descriptive): stop a process-plugin entry via the plan orchestrator's `Restart`, and assert
      the entry is actually running afterward (e.g. via a live PID check or the plugin's own `Status`),
      not just that `Restart` returned a nil error | verify: `/usr/bin/grep -Eq '^func TestProcessPlugin_Restart_LeavesProcessRunning\(' internal/lifecycle/process_test.go && go test ./internal/lifecycle -count=1`
- [x] `dva restart <plan>` and composition `restart` both leave the process-plugin entry genuinely
      running afterward, verified against the real binary (not just unit-level), for both a fast-exiting
      test process and one with a short graceful-shutdown delay | verify: `/usr/bin/grep -Eq '^func TestRunPlanRestartLeavesNativeProcessRunning\(' internal/cli/plan_lifecycle_test.go && go test ./internal/cli -count=1`
- [x] No change to `Down`/`removeProcess`/`Stop`'s existing preserved-PID-file semantics for the
      non-restart stop case (`dva stop <plan>` alone must still leave the PID file in place per the
      existing "Vagrant halt semantics" comment) | verify: `go test ./internal/lifecycle -run TestProcessPlugin_StopProcess_NoPidFile -count=1 -v`
- [x] Repository gates pass | verify: `make lint && make test && make test-integration && make commit-check`

## Non-goals

- Not fixing this for the `compose`/`kubectl`/`helm` plugins — their up/down semantics don't go through
  this same PID-file already-running check; this is process-plugin (native runner) specific.
- Not changing `haltProcess`'s decision to preserve the PID file across a plain `stop` — that behavior
  is intentional and documented, and is not what's broken here.
