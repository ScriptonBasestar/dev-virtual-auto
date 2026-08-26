---
id: TASK-232
title: "make install leaves an earlier destination replaced when a later rename fails"
type: fix
priority: P1
effort: M
created-at: 2026-08-26T17:30:00+09:00
source: "post-TASK-229 installation contract review"
scope: "Makefile install-binary compensation transaction and isolated installer fixture"
status: doing
---

# Task 232: compensate a partially replaced binary installation

## Summary

`install-binary` stages both candidates before the first rename, but a later destination rename
failure leaves an earlier destination on the new binary. The command must restore the earlier
state on a best-effort basis and report whether that restoration worked.

## Completion Criteria

- [x] Before replacement, retain an exact rollback copy of every existing regular destination;
  restore its bytes and mode when a later replacement fails | verify: `go test ./tools/installcheck`
- [x] If a prior destination did not exist, remove the newly created file during rollback | verify: `go test ./tools/installcheck`
- [x] Failure output distinguishes a successful rollback from a failed rollback and preserves
  the completed replacement and rollback ledgers | verify: `go test ./tools/installcheck`
- [x] All staging and backup artifacts are removed after success, failed replacement, and failed
  rollback | verify: `go test ./tools/installcheck`
- [x] The guarantee excludes crashes, power loss, and cross-filesystem all-or-nothing commit | verify: human — review this task and USAGE.md

## Decision

The installer still does not claim a multi-filesystem atomic commit: it stages and atomically
renames each destination independently. Before the first replacement it also copies each existing
regular destination into that destination's filesystem. If a later rename fails, it walks already
replaced destinations in reverse order, renaming the backup back or removing a newly created
destination. A rollback failure remains fail-fast and visible; cleanup removes retained staging
and backup artifacts. A crash or power loss between operations can still leave mixed state.
