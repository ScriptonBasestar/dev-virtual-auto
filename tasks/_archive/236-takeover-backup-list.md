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
  selected runtimes, skill names, and independently verified available/corrupt state | verify:
  `go test ./internal/skillinstall -run ListTakeoverBackups`
- [x] Scope/runtime filtering reuses installer destination resolution, deduplicates the same shared
  destination/backup-ID row, and preserves distinct backup IDs | verify:
  `go test ./internal/skillinstall -run ListTakeoverBackups`
- [x] Listing performs no state mutation across destination, neutral claim, and DVA state roots; every
  found receipt is validated even when it has no takeover metadata | verify:
  `go test ./internal/skillinstall -run ListTakeoverBackups`
- [x] Backup destination and backup-ID ancestors must be regular directories; symlink/non-directory
  ancestors are corrupt and restore cannot trust them | verify:
  `go test ./internal/skillinstall -run VerifyTakeoverBackupsRejectsUnsafeBackupAncestors`
- [x] Text output and `--json` surface are deterministic, and the command manifest marks list as a
  query | verify: `go test ./internal/cli -run Skill`
- [x] Canonical usage documentation describes discovery without implying delete or automatic GC | verify:
  `make doc-check`

## Decision

Keep backup deletion and retention policy out of this task. The list command validates the same
receipt and backup tree that restore uses, returns only the calculated backup ID plus its destination,
and never trusts a receipt-provided arbitrary state path. Each backup ID is verified independently so
one corrupt retained group does not conceal another valid group. Runtime filters select destinations;
when multiple runtimes share a project destination, resolver grouping deduplicates only an identical
destination/backup-ID row and leaves distinct retained IDs visible.
