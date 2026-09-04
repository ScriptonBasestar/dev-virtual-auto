---
id: TASK-292
title: "Implement composition CLI flag scope and aggregate output"
type: feature
priority: P1
effort: M
exec-tier: standard
created-at: 2026-09-04T10:00:00+09:00
source: "PLAN-005 implementation of TASK-260's frozen composition contract"
scope: "per-flag propagate/scope/reject enforcement for composition plans, --project scoping for destructive flags, and aggregate status/logs/build/JSON output"
status: done
depends-on: [TASK-290]
---

# Task 292: implement composition CLI flag scope and aggregate output

## Summary

Wire the exact flag-scope table TASK-260 §4.4 froze into command dispatch for composition plans, and
implement the aggregate `status`/`logs`/`build` behavior §4.3 and §5.5 specify. No flag receives new
semantics beyond what that table already decided.

## Recommended direction

At the point a composition plan is detected (mirroring how `internal/cli/compose.go` and its siblings
already detect a named plan today), apply TASK-260 §4.4 before calling into TASK-291's runtime:

- `--no-wait`, `--no-rollback`: pass through unchanged (propagate-to-all).
- `--var`, tag selectors (`--tag`/`--exclude-tag`), `--mode`, `--env`: reject before any child starts,
  with an error that names the composition plan and (for `--var`) points at `CompositionEntry.Vars` in
  `dva.yml` as the supported override mechanism instead.
- `--force`, `--volumes`/`-v`, `--purge`: require `--project <child>` naming one of the composition's
  actual children; reject before any child starts if the flag is present without that scope, or if the
  named child does not support the flag (e.g., no purge target).

For `logs`, prefix each child's stream with its project label; for `build`, run children in the same wave
order as `up` (no rollback — build is non-destructive and does not create running state, §4.3); for
`status`, children may be queried concurrently (read-only, no ordering constraint, §4.3) and the aggregate
report reuses TASK-291's partial-state JSON shape (§5.3, §5.5) for both text and `--json` output. Exit
codes stay flat 0/1 (TASK-260 §5.6) — do not add a new exit-code taxonomy.

## Completion Criteria

- [x] `--no-wait` and `--no-rollback` propagate unchanged to a composition plan's execution on every lifecycle verb (TASK-260 §4.4) | verify: `/usr/bin/grep -Eq '^func TestCompositionPropagateAllFlags\(' internal/cli/composition_flags_test.go && go test ./internal/cli -count=1`
- [x] `--var`, tag selectors, `--mode`, and `--env` are rejected before any child starts when a composition plan is named, with an error naming the plan (and, for `--var`, pointing at `CompositionEntry.Vars`) (TASK-260 §4.4) | verify: `/usr/bin/grep -Eq '^func TestCompositionRejectsWholeStackAndVarFlags\(' internal/cli/composition_flags_test.go && go test ./internal/cli -count=1`
- [x] `--force`, `--volumes`, and `--purge` require `--project <child>` scoping to one of the composition's actual children; missing scope or an unsupported child rejects before any child starts — never after a partial run (TASK-260 §4.4) | verify: `/usr/bin/grep -Eq '^func TestCompositionDestructiveFlagsRequireProjectScope\(' internal/cli/composition_flags_test.go && go test ./internal/cli -count=1`
- [x] `logs` labels each child's stream by project; `build` follows `up`'s wave order without triggering rollback; `status` returns TASK-291's partial-state JSON shape in both text and `--json` form (TASK-260 §4.3, §5.3, §5.5) | verify: `/usr/bin/grep -Eq '^func TestCompositionAggregateLogsStatusBuild\(' internal/cli/composition_flags_test.go && go test ./internal/cli -count=1`
- [x] Exit codes for a composition plan remain flat 0 (success) / 1 (any failure, including rejected cycles, partial failure, and rollback failure) — no new exit-code value is introduced (TASK-260 §5.6) | verify: `/usr/bin/grep -Eq '^func TestCompositionExitCodesStayFlat\(' internal/cli/composition_flags_test.go && go test ./internal/cli -count=1`
- [x] Repository gates pass | verify: `make lint && make test && make commit-check`

## Non-goals

- No new exit-code taxonomy or new confirmation-prompt mechanism beyond the existing
  `destructionWarning`/`confirmDestruction` pattern already used for `--volumes`/`--purge`.
- No orchestrator-level rollback logic — that is TASK-291's; this task only enforces flag scope and
  formats output.
- No completion/help-text projection changes beyond what documenting the new flags requires (no route or
  vocabulary rename).

## Deviations from literal card text

- **Post-review fixes (independent review round 1)**: an independent reviewer found two real bugs
  after the initial implementation, fixed in a follow-up commit on this same branch:
  1. `runCompositionStatus`/`queryCompositionChildStatus` originally hardcoded every child's
     `state` to `"up"` and only downgraded to `"failed"` on a query error — so a composition
     whose children were never started still reported `outcome: "up"` / every child `"up"`.
     Fixed by having `queryCompositionChildStatus` return the child's real `*lifecycle.
     AggregatedStatus` instead of discarding it, and classifying each child's state via a new
     exported `lifecycle.AnyServiceRunning` helper (reusing the orchestrator's existing
     `serviceLooksRunning` classification): `"up"` when a service looks running, `"not_started"`
     when the query succeeded but nothing is running, `"failed"` when the child could not be
     queried at all. `outcome` follows the same logic (`up` only when every child is up).
     Verified against the real `bin/dva status release --json` binary, not just unit tests.
  2. The composition flag validator had no equivalent of the single-plan path's
     `rejectDownOnlyFlags` guard, so `--purge`/`--volumes` passed validation on `up`/`stop`/
     `restart`/`build`/`logs` and would silently no-op once TASK-291 lands (a destructive flag
     accepted-and-ignored, which `plan_lifecycle.go`'s existing comment says must never happen).
     Fixed by adding the same down-only rejection to `validateCompositionFlagScope`, checked
     before the `--project` scope requirement. Verified against the real binary
     (`dva up release --purge --project api` now rejects with "only supported by down").
  Both fixes have dedicated test coverage in `composition_flags_test.go`; the previous version of
  that file had a test that locked in bug #1 (asserted `state == "up"` against a fixture where
  nothing was ever started) and a vacuous "nothing starts on rejection" check (asserted
  `build-order` absent after a rejected `down`, which `down` never writes regardless of whether
  the rejection is even correct) — both rewritten to assert the true, falsifiable behavior.
- **TASK-291 dependency gap closed (commit 2cffbd8)**: `up`/`down`/`stop`/`restart` on a
  composition plan now build a real `*lifecycle.CompositionOrchestrator` (TASK-291, frozen) and
  execute it instead of returning the `errCompositionRuntimeNotImplemented` stub. Notable design
  decisions made while wiring, none of which required touching the frozen orchestrator:
  - **`--force` per-child scoping**: `lifecycle.PlanChildExecutor.Force` is a flat bool, but
    TASK-260 §4.4 requires `--force` to apply only to the `--project`-scoped child. Solved with a
    small `compositionExecutor` wrapper in `composition_flags.go` holding a `forced` and an
    `unforced` `*PlanChildExecutor`, dispatching per child by name match — the orchestrator's
    `CompositionChildExecutor` interface is satisfied without any change to its contract.
  - **`restart` has no orchestrator-level verb**: `CompositionOrchestrator` exposes `Up`/`Down`/
    `Stop`/`Status` but no `Restart`. `runCompositionRestart` composes `Stop` then `Up` on the same
    orchestrator instance (mirroring `lifecycle.Orchestrator.Restart`'s single-plan pattern),
    short-circuiting on a failed `Stop`.
  - **`--project` scope on `down` narrows destructive options, not which children come down**:
    the frozen `teardown()` walks all composed children in reverse-wave order and tears down every
    child the executor's `IsUp` reports as running, regardless of `--project`; the flag only
    selects which child's `ChildDownOptions` carries `Volumes`/`RemoveImages` in
    `CompositionDownOptions.Destructive`. Confirmed by reading `composition_orchestrator.go`
    directly (not guessed) and covered by `TestCompositionUpDownExecuteRealOrchestrator` (proves
    real teardown of every up child) plus `TestCompositionDestructiveOptionsScopePerChild` (proves
    per-child options scoping in isolation).
  `build`, `logs`, and `status` were already fully implemented before this commit and are
  unchanged.
- **`--purge` "unsupported child" sub-case unimplemented**: TASK-260 §4.4 and the card's own
  Completion Criteria call for rejecting `--project <child>` scoping onto a child that "does not
  support" a destructive flag (e.g., no purge target). No capability distinction of this kind
  (e.g., "supports purge") exists anywhere else in the codebase to reuse or model this on, so
  `validateCompositionFlagScope` currently only validates that the named `--project` value
  resolves to an actual composed child, not whether that child's plugin supports the specific
  destructive flag. This is a known gap, not a silent scope narrowing — flagged here for
  follow-up once TASK-291 (or a future task) defines a per-plugin destructive-flag capability
  surface to check against.
- **`internal/cli/compose.go` file-size condition**: the repo's `ce-validate-filesize.sh`
  PostToolUse hook reported `compose.go` exceeds its 500-code-line soft limit on every edit made
  in this task (file was already ~845 code lines / 1318 total lines before this task's changes;
  this task added ~18 lines across 6 call sites). This is a pre-existing condition, not introduced
  by TASK-292, and splitting `compose.go` is out of scope per the card's non-goals. Confirmed this
  does not block any of the three required gates: `make lint`, `make test`, and `make commit-check`
  all passed clean in this worktree without any file-size-related failure — the hook only fires as
  an editor-time warning, not a `make`-driven gate.

## Review findings (independent review, 4 rounds)

- **B1 (fixed, `566c711`)**: `--dry-run` was silently swallowed on every composition verb —
  `parseDvaFlags` consumes it into the package-level `dryRun` global and strips it from
  `extraArgs` before `validateCompositionFlagScope` ever runs, so `newCompositionExecutor` never
  threaded it into either `PlanChildExecutor` instance and composition `up`/`down`/`stop`/
  `restart` executed for real regardless of the flag — including the destructive case
  `down --project X --purge --force --dry-run`, which really removed volumes/images pre-fix.
  Fixed by setting `DryRun: dryRun` on both executor instances and gating `runCompositionDown`'s
  confirmation prompt on `!dryRun` (mirroring `runPlanDown`'s `effectiveDryRun` precedent).
  Verified against the real binary on all four wired verbs and by the reviewer independently
  re-running the pre-fix code path to confirm the new tests are genuinely falsifiable (both
  failed against `566c711^`, one reproducing B1 by actually killing the test's fixture
  processes).
- **Failed-state status coverage gap (closed, `a736686`)**: `queryCompositionChildStatus`'s
  `"failed"` classification had zero test coverage — every fixture that made a child unrunnable
  made the whole root `config.Load` fail first, so the classifier was never reached. Closed by
  putting the unrunnable child in a separate subproject with a required-but-missing `env_file`,
  which fails only that child's `resolvePlanRuntime` while the root config and sibling children
  still load and query cleanly.
- **Non-blocking, deferred to backlog (not this card)**:
  1. `CompositionOrchestrator.Status` (TASK-291) has zero production callers — the CLI `status`
     verb reimplements the same classification independently via `queryCompositionChildStatus` +
     `lifecycle.AnyServiceRunning`. Both are correct today (same underlying
     `serviceLooksRunning` predicate) but it's two implementations of "is this child up" plus two
     independent definitions of §5.3's report shape (`lifecycle.CompositionReport` vs this file's
     `compositionStatusReport`). Worth a TASK-291 follow-up note, not a TASK-292 fix.
  2. TASK-260 §4.4's worked example for `--project` on `down` reads as "the other child is not
     touched," which is ambiguous against the frozen LIFO-teardown model (`--project` scopes
     which child gets destructive flags, not which children get torn down — confirmed correct by
     reading the frozen orchestrator and by binary reproduction). Docs-wording tightening on
     TASK-260's card, not a code change.
  3. Pre-existing `restart` bug, not introduced or worsened by this task: on both the single-plan
     and composition paths, `restart` stops the process, then its "up" half sees the same stale
     PID as "already running," starts nothing, and reports success (`rc=0`) while the
     process is actually down. Severe enough to warrant its own card — noted for a follow-up
     task, out of scope here.
