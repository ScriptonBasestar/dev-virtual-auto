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
status: todo
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

- [ ] `--no-wait` and `--no-rollback` propagate unchanged to a composition plan's execution on every
  lifecycle verb (TASK-260 §4.4) | verify: `/usr/bin/grep -Eq '^func TestCompositionPropagateAllFlags\(' internal/cli/composition_flags_test.go && go test ./internal/cli -count=1`
- [ ] `--var`, tag selectors, `--mode`, and `--env` are rejected before any child starts when a composition
  plan is named, with an error naming the plan (and, for `--var`, pointing at `CompositionEntry.Vars`)
  (TASK-260 §4.4) | verify: `/usr/bin/grep -Eq '^func TestCompositionRejectsWholeStackAndVarFlags\(' internal/cli/composition_flags_test.go && go test ./internal/cli -count=1`
- [ ] `--force`, `--volumes`, and `--purge` require `--project <child>` scoping to one of the composition's
  actual children; missing scope or an unsupported child rejects before any child starts — never after a
  partial run (TASK-260 §4.4) | verify: `/usr/bin/grep -Eq '^func TestCompositionDestructiveFlagsRequireProjectScope\(' internal/cli/composition_flags_test.go && go test ./internal/cli -count=1`
- [ ] `logs` labels each child's stream by project; `build` follows `up`'s wave order without triggering
  rollback; `status` returns TASK-291's partial-state JSON shape in both text and `--json` form (TASK-260
  §4.3, §5.3, §5.5) | verify: `/usr/bin/grep -Eq '^func TestCompositionAggregateLogsStatusBuild\(' internal/cli/composition_flags_test.go && go test ./internal/cli -count=1`
- [ ] Exit codes for a composition plan remain flat 0 (success) / 1 (any failure, including rejected
  cycles, partial failure, and rollback failure) — no new exit-code value is introduced (TASK-260 §5.6) |
  verify: `/usr/bin/grep -Eq '^func TestCompositionExitCodesStayFlat\(' internal/cli/composition_flags_test.go && go test ./internal/cli -count=1`
- [ ] Repository gates pass | verify: `make lint && make test && make commit-check`

## Non-goals

- No new exit-code taxonomy or new confirmation-prompt mechanism beyond the existing
  `destructionWarning`/`confirmDestruction` pattern already used for `--volumes`/`--purge`.
- No orchestrator-level rollback logic — that is TASK-291's; this task only enforces flag scope and
  formats output.
- No completion/help-text projection changes beyond what documenting the new flags requires (no route or
  vocabulary rename).
