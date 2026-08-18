---
id: TASK-185
title: "The backup is mentioned only inside the two flow files, in no document a user reads"
type: docs
priority: P2
effort: S
created-at: 2026-08-18T15:24:47+09:00
completed-at: 2026-08-18T19:05:00+09:00
source: "4ec336b — grep for the backup path matches flow YAML only"
scope: "dva repo — docs/, USAGE.md"
status: done
---

# Task 185: Tell users their config gets snapshotted

## Summary

`grep -rl 'backups/dva'` matches exactly two files, both flow definitions. No entry
in `docs/` or any root document says that running `dva-improve` writes a copy of the
config, where it lands, or that it is deliberately hidden from git.

That makes the feature undiscoverable in both directions. A user who wants the snapshot
cannot find it, and a user who finds the directory has no way to learn what wrote it or
whether deleting it is safe.

Three things carry the weight and are easy to get wrong if left implicit: the snapshot is
of the **working tree** rather than `HEAD`, the directory hides itself from git rather than
editing the project's `.gitignore`, and the flow never restores on its own.

## Completion Criteria

- [x] The backup path appears in user-facing docs | verify: `grep -rq 'backups/dva' docs/`
- [x] The doc says the snapshot captures uncommitted working-tree state, not HEAD | verify: human — read the section
- [x] The doc says the directory ignores itself and the project's own `.gitignore` is untouched | verify: human — read the section
- [x] The doc is reachable from where improve is described, not filed on its own | verify: human — the improve/guided documentation links to it

## Technical Notes

- Depends on TASK-183 for the restore half and TASK-184 for the retention rule; writing all
  three into one section at the end costs less than three passes, but this card should not
  block on them — documenting what exists today is already worth doing.
- Doc size and naming rules: `skill:docs:doc-standards`.

## Resolution

Closed by the work delivered under TASK-183 rather than by a second pass. The technical
note on this card predicted that ("writing all three into one section at the end costs
less than three passes"), and that is what happened — 183 needed the same section to
explain where a snapshot lives before it could explain how to restore one.

No new writing was required. Each criterion was checked against what shipped:

| criterion | where it is satisfied |
| --- | --- |
| backup path in user-facing docs | `docs/50-improve-flow-backup-and-restore.md`, `## 어디에 남는가` |
| working-tree state, not HEAD | `## 왜 스냅샷이 필요한가` — "git이 덮지 못하는 창 하나를 덮는다 — **플로우 실행 시점의 미커밋 로컬 수정**", with a table whose other row sends restoring-to-last-commit to `git checkout --` |
| directory ignores itself, project `.gitignore` untouched | `## 어디에 남는가` — the `.gitignore` containing `*`, and the stated reason: not adding an unrequested line to someone else's repository |
| reachable from where improve is described | `USAGE.md:983`, at the end of the `### 설정 백업과 복원` section, which sits directly under the improve flow documentation |

The remaining half of this card's concern — a user who *finds* the directory and wants to
know what wrote it — is answered by the doc's opening line and its `## 구현 위치` section,
which name the flows and the steps that write there.

Retention is deliberately still open as TASK-184. `docs/50` documents the current
unbounded behaviour and a measured prune one-liner rather than claiming a rule that does
not exist yet.
