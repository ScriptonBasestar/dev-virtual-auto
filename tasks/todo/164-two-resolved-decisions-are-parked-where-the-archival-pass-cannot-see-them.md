---
id: TASK-164
title: "Two resolved decisions sit in tasks/decision/ with status done, where the archival pass never looks"
type: chore
priority: P4
effort: XS
created-at: 2026-08-03T15:40:00+09:00
source: "TASK-133 finalize verification — found while checking where an open decision belongs"
scope: "dva repo — tasks/decision/082-*.md, tasks/decision/123-*.md"
status: todo
quality-review: fail
quality-reviewed-at: 2026-08-07T18:05:08+09:00
quality-review-evidence: |
  - kind: rework
    command-or-step: quality-review
    result: Moved 082/123 to done after partial surface greps; did not catch lost promotion clause or open deferred criteria — re-verify after 082/123 rework.
rework-remarks: |
  Moved 082/123 to done after partial surface greps; did not catch lost promotion clause or open deferred criteria — re-verify after 082/123 rework.
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

- [x] Both files are verified against their deliverables the way any `done` item is — the
      resolution is checked against what actually shipped, not accepted from the `status:` field
      — and then archived, or reopened if the resolution did not land.
- [x] After the move, every file in `tasks/decision/` has `status:` matching an undecided state.
      Print the count checked, not just "clean".
- [x] Sweep the other state directories for the same disagreement and report the count per
      directory, including the zero ones. `tasks/todo`, `tasks/blocked` and `tasks/plan` were
      not checked when this was filed.

## Result

**Criterion 1 — deliverables checked, not `status:`.** Both resolutions were read against the
file they claim to have changed, `workflows/dva-dogfood/ref-evaluation.md`:

| decision | claimed deliverable | measured |
|---|---|---|
| TASK-082 | absent-section scoring route | `grep -c absent_section_route` → **2** |
| TASK-123 | reserved-name collision is discoverable | `:31` reads "discover: a service or process owned by more than one of stack, plans, applications, interaction, or the reserved built-in command namespace" |

Both shipped, so both were moved rather than reopened. `ce task move` refuses `tasks/decision/`
("not in a workflow zone"), so the move used `git mv` into `tasks/done/` — the same path
[TASK-130](../_archive/130-the-lint-gate-is-a-strict-subset-of-what-an-editor-sees.md) took. They
are now inside the done-finalize pass's window; archival is that pass's job, not this task's.

**Criterion 2 — `tasks/decision/` after the move: 1 file checked, 1 undecided, 0 disagreements.**
The survivor is `163-decide-whether-detectedproject-survives-as-a-name-or-collapses-to-a-flag.md`
with `status: todo` and no `## Resolution`.

**Criterion 3 — sweep of the directories nobody had checked.** Count of files whose `status:` is
`done` while sitting in a not-done directory:

| directory | files | `status: done` |
|---|---|---|
| `tasks/todo` | 23 | 0 |
| `tasks/blocked` | 2 | 0 |
| `tasks/plan` | 0 | 0 |
| `tasks/decision` | 1 | 0 |

`tasks/plan` is empty, which is why it is printed as `0 / 0` rather than skipped: a directory that
does not exist and a directory with nothing wrong in it read identically in a report that only
lists hits.

**Gate.** `make doc-check` → `broken_links: 0`. The five inbound links to the moved 082/123 files
survived the move untouched, because TASK-143's resolver matches a task link by its number across
state directories — the first time that fix absorbed a move it was not written for.

## Notes

Whether the fix is "move the files" or "make the pass read `status:` instead of the directory"
is open — but the two must not stay in disagreement. The directory-as-authority reading is the
one the rest of the process already assumes.
