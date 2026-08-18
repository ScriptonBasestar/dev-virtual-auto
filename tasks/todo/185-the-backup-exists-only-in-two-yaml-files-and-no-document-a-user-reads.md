---
id: TASK-185
title: "The backup is mentioned only inside the two flow files, in no document a user reads"
type: docs
priority: P2
effort: S
created-at: 2026-08-18T15:24:47+09:00
source: "4ec336b — grep for dva-improve-backups matches flow YAML only"
scope: "dva repo — docs/, USAGE.md"
status: todo
---

# Task 185: Tell users their config gets snapshotted

## Summary

`grep -rl dva-improve-backups` matches exactly two files, both flow definitions. No entry
in `docs/` or any root document says that running `dva-improve` writes a copy of the
config, where it lands, or that it is deliberately hidden from git.

That makes the feature undiscoverable in both directions. A user who wants the snapshot
cannot find it, and a user who finds the directory has no way to learn what wrote it or
whether deleting it is safe.

Three things carry the weight and are easy to get wrong if left implicit: the snapshot is
of the **working tree** rather than `HEAD`, the directory hides itself from git rather than
editing the project's `.gitignore`, and the flow never restores on its own.

## Completion Criteria

- [ ] The backup path appears in user-facing docs | verify: `grep -rq 'dva-improve-backups' docs/`
- [ ] The doc says the snapshot captures uncommitted working-tree state, not HEAD | verify: human — read the section
- [ ] The doc says the directory ignores itself and the project's own `.gitignore` is untouched | verify: human — read the section
- [ ] The doc is reachable from where improve is described, not filed on its own | verify: human — the improve/guided documentation links to it

## Technical Notes

- Depends on TASK-183 for the restore half and TASK-184 for the retention rule; writing all
  three into one section at the end costs less than three passes, but this card should not
  block on them — documenting what exists today is already worth doing.
- Doc size and naming rules: `skill:docs:doc-standards`.
