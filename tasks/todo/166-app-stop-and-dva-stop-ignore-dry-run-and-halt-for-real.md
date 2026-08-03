---
id: TASK-166
title: "app stop and dva stop accept --dry-run and halt the app for real (the sibling of TASK-153)"
type: bug
priority: P2
status: todo
effort: S
created-at: 2026-08-03T14:05:00+09:00
source: "TASK-153 review (M1) — the same defect class in scope-out siblings"
depends-on: [TASK-153]
scope: "dva repo — internal/cli/app.go:76, internal/cli/compose.go:399"
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

- [ ] `app stop` and `dva stop` branch on `dryRun` the way `app restart` does: `HaltAppsDryRun`
      under `--dry-run`, `HaltApps` otherwise. `dva stop` keeps its existing stack-half behaviour
      (`orch.Stop` already receives `dryRun`).
- [ ] A test for each path asserts no SIGTERM under `--dry-run` (a stand-in process left alive), in
      the shape of TASK-153's `TestHaltAppsDryRunDoesNotSignal`. Prefer driving the CLI wiring
      (`RunE`), not just the lifecycle layer, so dropping the branch is caught (the gap TASK-153's
      review M2 noted).
- [ ] `make test` exits 0.

## Notes

`DownApps` (`dva down`) is distinct: it removes pid/log files and reclaims ports, so its dry-run
story is larger than a signal withholding — out of scope here unless the same flag is found to
reach it. Check whether `dva down --dry-run` reaches `DownApps` and record the answer.
