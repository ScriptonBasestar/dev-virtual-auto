---
id: TASK-164
title: "Two resolved decisions sit in tasks/decision/ with status done, where the archival pass never looks"
type: chore
priority: P4
status: todo
effort: XS
created-at: 2026-08-03T15:40:00+09:00
source: "TASK-133 finalize verification — found while checking where an open decision belongs"
scope: "dva repo — tasks/decision/082-*.md, tasks/decision/123-*.md"
---

# Task 164: Move the two finished decisions out of the decision queue

## Problem

`tasks/decision/` is the queue of decisions still to be made. Two of its three files are already
made:

| file | `status:` | has `## Resolution` |
|---|---|---|
| `082-the-dogfood-loop-cannot-score-an-absent-section.md` | `done` | yes, `:65` |
| `123-dogfood-loop-cannot-score-a-reserved-name-collision.md` | `done` | yes, `:63` |
| `163-decide-whether-detectedproject-survives-as-a-name-or-collapses-to-a-flag.md` | `todo` | no — genuinely open |

The state directory and the `status:` field disagree, and the directory is the one the tooling
walks. The done-finalize pass reads `tasks/done/`, so these two are never verified against their
deliverables and never archived — they are finished work that the process cannot see. Compare
[TASK-130](../_archive/130-the-lint-gate-is-a-strict-subset-of-what-an-editor-sees.md), a
decision that was resolved and moved to `done/`, which is the path these two did not take.

Two records is small. The reason to fix it is that a queue whose contents do not mean what the
queue name says stops being readable as a queue — the same shape as a gate whose silence reads
as a pass.

## Acceptance criteria

- [ ] Both files are verified against their deliverables the way any `done` item is — the
      resolution is checked against what actually shipped, not accepted from the `status:` field
      — and then archived, or reopened if the resolution did not land.
- [ ] After the move, every file in `tasks/decision/` has `status:` matching an undecided state.
      Print the count checked, not just "clean".
- [ ] Sweep the other state directories for the same disagreement and report the count per
      directory, including the zero ones. `tasks/todo`, `tasks/blocked` and `tasks/plan` were
      not checked when this was filed.

## Notes

Whether the fix is "move the files" or "make the pass read `status:` instead of the directory"
is open — but the two must not stay in disagreement. The directory-as-authority reading is the
one the rest of the process already assumes.
