---
id: TASK-236
title: "Retained takeover backups have no deterministic read-only inventory"
type: feature
priority: P2
effort: S
created-at: 2026-08-27T00:00:00+09:00
source: "post-TASK-235 operational follow-up"
scope: "takeover backup list API, CLI, manifest, tests, and user documentation"
status: done
completed-at: 2026-08-27T00:00:00+09:00
verification-status: verified
---

# Task 236: list retained takeover backups safely

## Summary

Takeover creates a durable, receipt-verified backup and ordinary uninstall deliberately retains it.
`dva skill status` only surfaces the first backup path per installation, so an operator cannot obtain
a stable backup ID inventory without inspecting DVA state directly.

## Completion Criteria

- [x] `dva skill backup list` reports each receipt-backed backup by deterministic ID, destination,
  selected runtimes, skill names, and available/corrupt verification state | verify:
  `go test ./internal/skillinstall -run ListTakeoverBackups`
- [x] Scope/runtime filtering reuses installer destination resolution and lists a shared project
  destination once | verify: `go test ./internal/skillinstall -run ListTakeoverBackups`
- [x] Listing performs no state mutation and fails closed on an invalid receipt | verify:
  `go test ./internal/skillinstall -run ListTakeoverBackups`
- [x] Text output and `--json` surface are deterministic, and the command manifest marks list as a
  query | verify: `go test ./internal/cli -run Skill`
- [x] Canonical usage documentation describes discovery without implying delete or automatic GC | verify:
  `make doc-check`

## Decision

Keep backup deletion and retention policy out of this task. The list command validates the same
receipt and backup tree that restore uses, returns only the calculated backup ID plus its destination,
and never trusts a receipt-provided arbitrary state path. Runtime filters select destinations; when
multiple runtimes share a project destination, resolver grouping deliberately produces one row.
