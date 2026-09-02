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
status: todo
---

# Task 253: align help groups and descriptions

## Summary

Make the existing command surface easier to scan without changing argv, aliases, reserved names, or
manifest schema. Move `status` to the lifecycle help group, present `manifest` as a core machine-discovery
command, and distinguish declared configuration (`show`) from current workspace/runtime state (`status`).

## Completion Criteria

- [ ] Root help places `status` with lifecycle commands and `manifest` with core discovery commands while every command name, alias, argument rule, flag, and reserved-name behavior remains unchanged | verify: `/usr/bin/grep -Eq '^func TestCommandHelpGroupsAndDiscoveryDescriptions\(' internal/cli/root_command_registration_test.go && go test ./internal/cli -count=1`
- [ ] `show` is described as declared workspace configuration and `status` as current workspace and runtime status without claiming that status is runtime-only | verify: `go test ./internal/cli -count=1`
- [ ] Help snapshots or focused assertions pin group membership, ordering, and the two descriptions so future drift is visible | verify: `go test ./internal/cli -count=1`
- [ ] User-facing current-state documentation is updated only where the visible help or descriptions changed, with no new route advertised | verify: `make doc-check`
- [ ] Repository gates pass | verify: `make lint && make test && make test-integration && make commit-check`

## Non-goals

- No command metadata registry refactor.
- No `ktl`, `validate`, project addressing, or vocabulary decision.
- No manifest JSON field or compatibility change.
- No reserved-command count or command-set correction; TASK-244 owns that current-state drift.
