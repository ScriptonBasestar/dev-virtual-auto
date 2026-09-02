---
id: TASK-253
title: "Align help groups and discovery descriptions"
type: feature
priority: P1
effort: S
exec-tier: standard
created-at: 2026-09-02T10:05:00+09:00
source: "PLAN-003 incremental discovery improvement"
scope: "Cobra help grouping, command descriptions, focused tests, and current documentation"
status: done
completed-at: 2026-09-02T13:27:05+09:00
completion-summary: "Moved manifest into Core and status into Lifecycle help, clarified show/status discovery descriptions, and aligned focused tests and USAGE without changing routes or schema."
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "go test ./internal/cli -count=1"
    result: "passed, including focused help grouping, order, description, and compatibility assertions"
  - kind: automated
    command-or-step: "dva lint && dva test && dva test integration"
    result: "passed; static analysis, race/coverage suite, and integration suite exited zero"
  - kind: automated
    command-or-step: "make doc-check && make commit-check"
    result: "passed; links, size, CI labels, flow decisions, and commit subjects verified"
quality-review: pass
quality-reviewed-at: 2026-09-02T13:27:05+09:00
quality-review-evidence:
  - "independent reviewer verified help placement, exact descriptions, invariant tests, and manifest schema stability"
  - "review major about the stale positional show claim was fixed; focused re-review passed"
archived-at: 2026-09-02T13:27:05+09:00
verified-at: 2026-09-02T13:27:05+09:00
verification-summary: "Help taxonomy and discovery descriptions are aligned and regression-tested without changing any public route or manifest schema."
---

# Task 253: align help groups and descriptions

## Summary

Make the existing command surface easier to scan without changing argv, aliases, reserved names, or
manifest schema. Move `status` to the lifecycle help group, present `manifest` as a core machine-discovery
command, and distinguish declared configuration (`show`) from current workspace/runtime state (`status`).

## Completion Criteria

- [x] Root help places `status` with lifecycle commands and `manifest` with core discovery commands while every command name, alias, argument rule, flag, and reserved-name behavior remains unchanged | verify: `/usr/bin/grep -Eq '^func TestCommandHelpGroupsAndDiscoveryDescriptions\(' internal/cli/root_command_registration_test.go && go test ./internal/cli -count=1`
- [x] `show` is described as declared workspace configuration and `status` as current workspace and runtime status without claiming that status is runtime-only | verify: `go test ./internal/cli -count=1`
- [x] Help snapshots or focused assertions pin group membership, ordering, and the two descriptions so future drift is visible | verify: `go test ./internal/cli -count=1`
- [x] User-facing current-state documentation is updated only where the visible help or descriptions changed, with no new route advertised | verify: `make doc-check`
- [x] Repository gates pass | verify: `dva lint && dva test && dva test integration && make commit-check`

## Implementation record

- `manifest` is now a Core command and `status` is rendered under Lifecycle's Other Commands section.
- `show` names declared workspace configuration; `status` names current workspace and runtime state.
- Focused assertions pin group order, help block placement, Use, aliases, Args, flags, reservation, manifest descriptions and schema version.
- USAGE presents manifest with core discovery, status with lifecycle, and no longer advertises the unsupported positional `show <NAME>` form.

## Non-goals

- No command metadata registry refactor.
- No `ktl`, `validate`, project addressing, or vocabulary decision.
- No manifest JSON field or compatibility change.
- No reserved-command count or command-set correction; TASK-244 owns that current-state drift.
