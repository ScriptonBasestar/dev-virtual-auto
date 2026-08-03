---
id: TASK-166
title: "app stop and dva stop accept --dry-run and halt the app for real (the sibling of TASK-153)"
type: bug
priority: P2
status: done
effort: S
created-at: 2026-08-03T14:05:00+09:00
source: "TASK-153 review (M1) — the same defect class in scope-out siblings"
depends-on: [TASK-153]
scope: "dva repo — internal/cli/{app,compose,provision}.go, internal/lifecycle/{app_manager,orchestrator}.go"
---

# Task 166: Wire --dry-run through the remaining halt paths

## Problem

TASK-153 made `app restart --dry-run` simulate its halt half via `HaltAppsDryRun`. Two sibling
halt paths still accept `--dry-run` and ignore it, sending real SIGTERM:

- **`dva app stop --dry-run`** (`internal/cli/app.go:76`). `app stop` does not set
  `DisableFlagParsing` (only `up`/`restart`/`build` do), so cobra parses the inherited persistent
  `--dry-run` flag and sets the `dryRun` global. `HaltApps(args...)` runs unconditionally — the flag
  is silently accepted and the app is really halted.
- **`dva stop --dry-run`** (`internal/cli/compose.go:399`). `DisableFlagParsing:true`, but
  `parseDvaFlags` (called at `compose.go:395`) consumes `--dry-run` and sets the global, which is
  then passed to `orch.Stop` at `compose.go:404` — yet `am.HaltApps()` at `:399` runs
  unconditionally. So the stack half previews while the app half sends real SIGTERM.

This is the precise asymmetry TASK-153 fixed for `app restart`, surviving in the two paths its
review (M1) traced. An earlier draft of TASK-153's decision wrongly claimed these callers "have no
`--dry-run` half to honour"; that premise does not hold.

## Acceptance criteria

- [x] `app stop` and `dva stop` branch on `dryRun` the way `app restart` does: `HaltAppsDryRun`
      under `--dry-run`, `HaltApps` otherwise. `dva stop` keeps its existing stack-half behaviour
      (`orch.Stop` already receives `dryRun`).
      **Done, and six more sites with it** — see Scope below. The two named paths are
      `internal/cli/app.go:80` and `internal/cli/compose.go:408`.
- [x] A test for each path asserts no SIGTERM under `--dry-run` (a stand-in process left alive), in
      the shape of TASK-153's `TestHaltAppsDryRunDoesNotSignal`. Prefer driving the CLI wiring
      (`RunE`), not just the lifecycle layer, so dropping the branch is caught (the gap TASK-153's
      review M2 noted).
      `TestDryRunHaltPathsDoNotSignal` drives all four `RunE`s; reverting the branches fails
      all four subtests. `TestSignalModeProcessesDryRun` and
      `TestHaltAndStopModeProcessesForwardDryRun` cover the mode health_check path found in
      review, which had no test of any kind.
- [x] `make test` exits 0. 7 packages ok, 0 FAIL.

## Notes

`DownApps` (`dva down`) is distinct: it removes pid/log files and reclaims ports, so its dry-run
story is larger than a signal withholding — out of scope here unless the same flag is found to
reach it. Check whether `dva down --dry-run` reaches `DownApps` and record the answer.

**Answer: yes, at three call sites**, so the condition the Notes set fired and the scope grew
with it — `internal/cli/app.go:96` (`app down`), `internal/cli/compose.go:341` (`dva down`)
and `:586` (`dva clean`). All three passed `dryRun` to the orchestrator on the very next line
while calling `DownApps()` unconditionally above it.

## Scope: eight sites, not two

The task named two. Measuring first found six; review of that fix found two more. None of the
extras are adjacent code — they are the same flag, in the same commands, with strictly more to
lose. Before:

| command | process | pidfile | markers |
|---|---|---|---|
| `app stop --dry-run` | KILLED | kept | — |
| `app down --dry-run` | KILLED | **REMOVED** | — |
| `dva stop --dry-run` | KILLED | kept | — |
| `dva down --dry-run` | KILLED | **REMOVED** | — |
| `dva clean --volumes --force --dry-run` | KILLED | **REMOVED** | **DELETED** |
| `dva stop --mode dev --dry-run` | KILLED | kept | — |
| `dva down --mode dev --dry-run` | KILLED | **REMOVED** | — |
| *control:* `app stop` (no flag) | KILLED | kept | — |

The bottom row is the finding that mattered: under `--dry-run` the output was
byte-comparable to the no-flag control. The flag did nothing at all on the app half.

After, same fixtures:

| command | process | pidfile | markers | preview lines |
|---|---|---|---|---|
| `app stop --dry-run` | ALIVE | kept | — | 1 |
| `app down --dry-run` | ALIVE | kept | — | 3 |
| `dva stop --dry-run` | ALIVE | kept | — | 1 |
| `dva down --dry-run` | ALIVE | kept | — | 3 |
| `dva clean --volumes --force --dry-run` | ALIVE | kept | kept | 1 named marker |
| `dva stop --mode dev --dry-run` | ALIVE | kept | — | 1 |
| `dva down --mode dev --dry-run` | ALIVE | kept | — | 2 |
| *control:* `app down` (no flag) | KILLED | REMOVED | deleted | 0 |
| *control:* `dva down --mode dev` | KILLED | REMOVED | — | 0 |

The sixth site is the one worth arguing about, because it is not a halt:
`clearProvisionMarkers` at `compose.go:582`. It is in scope because of what stopping short
would have produced — `dva clean --volumes --dry-run` printing "would stop", "would delete
app-x.pid" and "would remove volumes" while silently deleting the provision markers. A
preview that is honest about five things and lying about the sixth is harder to catch than
one that is plainly wrong throughout, because the five true lines are what earn it trust.
That is TASK-137's silent-relocation shape: a partial repair that moves the remaining fault
somewhere it is less likely to be looked for.

### The seventh and eighth: `signalModeProcesses`

Review of the six-site fix found two more, outside every file this task had touched:
`internal/lifecycle/orchestrator.go`'s `signalModeProcesses`, reached through
`stopModeProcesses` from `Orchestrator.Down` (`:173`) and `haltModeProcesses` from
`Orchestrator.Stop` (`:228`). Both are the **first statement** of their method, before any
filtering, so the fix threads `dryRun` down rather than guarding at the call sites — a guard
there would have to be written twice and would be free to drift.

These cover mode `health_checks` with a `start:` command, a process class distinct from
`applications:`, so the app-side fix above did not reach them. `dva stop` and `dva down` are
two of the commands this task already claimed to have fixed: the change as reviewed made them
honest about their apps and their stack while still killing the mode processes for real.

The same argument that pulled in `clearProvisionMarkers` applies unchanged, and the reason the
gap survived this long is visible in the file. `startModeProcesses`, the `up` half of the pair,
has honoured `opts.DryRun` since it was written. The pair looked done because half of it was.

## Result

`DownAppsDryRun` is new, alongside TASK-153's `HaltAppsDryRun`. It previews three things the
real path does — the signal, the port reclaim, the file deletions — and departs from
`DownApps` on two points, both deliberate:

- It announces only files that **exist**. `DownApps` removes both unconditionally, which is
  harmless (removing a missing file is a no-op) but would make the preview claim work that
  is not going to happen.
- The port line is gated on the pidfile, exactly as `DownApps` gates it. A docker app's port
  is held by `docker-proxy`; naming it here would advertise a kill the real path never makes.
  It reports through `portOwnerPIDs`, which is `reclaimPort` without the kill.

`clearProvisionMarkers` was split the same way: `provisionMarkers(dir)` lists what
`clearProvisionMarkers(dir)` deletes, and the latter is now written in terms of the former.
A preview that re-derived the `provisioned-` prefix itself would be free to drift from the
deletion it describes.

`signalModeProcesses` prints its two preview lines on different conditions, matching the real
path line for line. The stop line is gated on the process existing, because the real path's is
— it prints only when `Kill` returns nil. The delete line is not, because the real path's
`os.Remove` runs for any `pid > 0` whether the kill succeeded or not; a preview that hid it
would understate what is about to be lost.

### Tests

Five: three in `internal/cli` rather than `internal/lifecycle`, which is the point, and two in
`internal/lifecycle` for a function nothing in the repo had ever called from a test.

`TestDryRunHaltPathsDoNotSignal` drives `appStopCmd`, `appDownCmd`, `stopCmd` and `downCmd`
through `RunE`. TASK-153's review (M2) named this gap: a lifecycle-level test proves
`HaltAppsDryRun` works, not that any command calls it, and deleting the `if dryRun` branch
leaves every such test green. Reverting all four branches:

```
--- FAIL: TestDryRunHaltPathsDoNotSignal/app_stop
--- FAIL: TestDryRunHaltPathsDoNotSignal/app_down
        app down --dry-run deleted app-sleeper.pid
        app down --dry-run deleted app-sleeper.log
--- FAIL: TestDryRunHaltPathsDoNotSignal/dva_stop
--- FAIL: TestDryRunHaltPathsDoNotSignal/dva_down
```

`TestCleanDryRunKeepsProvisionMarkers` drives `cleanCmd.RunE`. Its first draft asserted on
`provisionMarkers`/`clearProvisionMarkers` directly and **stayed green** when the guard in
`compose.go` was reverted — the same registration gap TASK-140 hit, found the same way and
only because the revert was actually run. Rewritten against `RunE`, it fails:

```
--- FAIL: TestCleanDryRunKeepsProvisionMarkers
    clean --volumes --dry-run deleted the provision marker
    clean --dry-run did not name the marker it would delete
```

`TestProvisionMarkersMatchesWhatClearDeletes` pins the probe to the deleter, including a
`not-a-marker` file that neither half may touch. It is deliberately *not* a wiring test and
stays green on the guard revert; the test above covers that.

`TestSignalModeProcessesDryRun` and `TestHaltAndStopModeProcessesForwardDryRun` cover the
seventh and eighth sites. The second exists because `signalModeProcesses` is unexported and
called from nowhere but the two wrappers, so a branch inside it is only reachable if both
forward the flag. Reverting the branch fails all four subtests on the load-bearing line:

```
--- FAIL: TestSignalModeProcessesDryRun/halt:_stop_semantics,_pidfile_kept
        dry-run killed the stand-in (pid 83315); output was:
        [-] stopped worker (pid 83315)
--- FAIL: TestSignalModeProcessesDryRun/stop:_down_semantics,_pidfile_removed
--- FAIL: TestHaltAndStopModeProcessesForwardDryRun/haltModeProcesses
--- FAIL: TestHaltAndStopModeProcessesForwardDryRun/stopModeProcesses
```

That output is the second draft. The first used `IsProcessRunning` alone and did fail on the
revert — but on the *output* assertions, while `"dry-run killed the stand-in"` stayed silent
against a process that had genuinely been SIGTERMed. `IsProcessRunning` is `Signal(0)`, which
succeeds against a zombie, and the stand-in is a direct child of the test binary, so a corpse
reads as alive until something calls `Wait`. Production never sees this: the process signalled
there was started by an earlier `dva up` and re-parented to init, which reaps it. So the
harness models liveness differently from the code — `modeStandIn` returns a channel closed on
reap, and `assertStillAlive` waits 300ms on it to establish that nothing exited. Proving a
non-event needs a window; it costs the suite about a second and is documented rather than
tuned quietly.

Worth noting for the next reader of a red suite: the first draft **was** failing, on four
subtests, for the right defect. Only reading *which* assertion fired showed the strongest one
was inert.

Two things cost more time than the fix and are worth recording:

- The stand-in process must be a **process-group leader** (`SysProcAttr{Setpgid: true}`).
  `HaltApps` signals the group (`syscall.Kill(-pid, ...)`), so a plain `exec.Command`, which
  inherits the test binary's group, would not fail the assertion — it would signal the test
  runner. On macOS there is no `setsid`; the shell equivalent for manual probing is
  `perl -e 'setpgrp; exec "sleep","300"'`.
- `loadConfig`/`loadEnv` memoise into the `cfg`/`env` package globals (`root.go:301`, `:357`)
  for the life of the test binary. Every subtest after the first got the first one's Config,
  whose `FileDir()` pointed at a `TempDir` already removed by cleanup — so the lookups landed
  in a deleted directory and the failure read as "the command said nothing" rather than "the
  command was looking elsewhere". `standInApp` saves and restores both.

The fixture carries no `version:` key, for the reason TASK-137 recorded: it declares the
minimum dva version, and a failed load calls `os.Exit(1)` inside `mustLoadConfig`, taking the
test binary down with no output.

`make build`, `make test` (7 ok / 0 FAIL), `go vet`, `gofmt -l` and `make doc-check` all exit 0.
