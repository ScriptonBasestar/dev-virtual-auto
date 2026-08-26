---
id: TASK-185
title: "The backup is mentioned only inside the two flow files, in no document a user reads"
type: docs
priority: P2
effort: S
created-at: 2026-08-18T15:24:47+09:00
completed-at: 2026-08-18T19:05:00+09:00
quality-review: pass
quality-reviewed-at: 2026-08-19T14:08:41+09:00
verified-at: 2026-08-19T14:21:45+09:00
archived-at: 2026-08-19T14:21:45+09:00
quality-review-evidence: |
  - kind: automated
    command-or-step: "grep -rq 'backups/dva' docs/ (AC1)"
    result: exit 0 — the binding is scoped to docs/ and matches docs/50-improve-flow-backup-and-restore.md alone. USAGE.md names the directory too (:973, :977, :978) but sits at the repository root, outside this grep. The card's original finding was that only the two flow YAMLs matched. Corrected after archival: this line first read "docs/50 ... and USAGE.md both name the directory", crediting the binding with a match it cannot make
  - kind: manual
    command-or-step: "AC2 — read docs/50 for working-tree vs HEAD"
    result: pass — "## 왜 스냅샷이 필요한가" names 미커밋 로컬 수정 in bold and tables the split, sending the last-commit case to `git checkout --` and reserving 스냅샷만 for the uncommitted one
  - kind: manual
    command-or-step: "AC3 — read docs/50 for the self-ignoring directory"
    result: pass — "## 어디에 남는가" shows the tree with `.gitignore` containing `*`, states the project root .gitignore is untouched, and gives the reason (남의 저장소에 요청받지 않은 줄을 추가하지 않는다)
  - kind: manual
    command-or-step: "AC4 — placement of the inbound link"
    result: pass — USAGE.md:983 links docs/50 from the end of `### 설정 백업과 복원` (USAGE.md:971), which sits inside `## LLM Integration` where the improve flows are described; make doc-check resolves the link
  - kind: manual
    command-or-step: "residual half — a user who finds the directory and asks what wrote it"
    result: pass — docs/50's opening line names the `am run dva-improve` family and `## 구현 위치` tables both flows with their four steps
  - kind: manual
    command-or-step: "drift note (not a defect in this card)"
    result: the Resolution's closing paragraph says retention is still open and docs/50 documents unbounded behaviour with a manual one-liner. True at completion; TASK-184 has since landed and docs/50 "## 보존" now states the 10-file bound. Superseded by a later card, not wrong when written
source: "4ec336b — grep for the backup path matches flow YAML only"
scope: "dva repo — docs/, USAGE.md"
status: done
verification-summary: |
  quality-review pass, re-observed at disposition. `grep -rq 'backups/dva' docs/` exit 0 and all
  four criteria still land where the review found them: docs/50 `## 왜 스냅샷이 필요한가` names
  미커밋 로컬 수정 and routes the last-commit case to `git checkout --`; `## 어디에 남는가` shows
  the `.gitignore` holding `*` and states the project root file is untouched; USAGE.md:983 links
  docs/50 from the end of `### 설정 백업과 복원` (:971) and `make doc-check` exit 0 resolves it.
  Archived with the recorded drift note standing: the Resolution's closing paragraph describes
  retention as still open, which TASK-184 has since closed — superseded, not wrong when written.
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

- [x] The backup path appears in user-facing docs | verify: `/usr/bin/grep -rq 'backups/dva' docs/`
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
