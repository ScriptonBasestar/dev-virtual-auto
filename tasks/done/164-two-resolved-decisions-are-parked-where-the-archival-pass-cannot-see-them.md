---
id: TASK-164
title: "Two resolved decisions sit in tasks/decision/ with status done, where the archival pass never looks"
type: chore
priority: P4
effort: XS
created-at: 2026-08-03T15:40:00+09:00
source: "TASK-133 finalize verification — found while checking where an open decision belongs"
scope: "dva repo — tasks zones for 082/123 parking + status agreement"
status: done
quality-review: fail
quality-reviewed-at: 2026-08-07T18:05:08+09:00
quality-review-evidence: |
  - kind: rework
    command-or-step: quality-review
    result: Moved 082/123 to done after partial surface greps; did not catch lost promotion clause or open deferred criteria — re-verify after 082/123 rework.
  - kind: unit
    unit: 164-resweep
    command-or-step: re-verify 082/123 surfaces + zone/status sweep after 082+123 honesty retarget
    result: ok — surfaces present; 082/123 stay in todo with deferred cycle ACs; decision/ empty; zero status:done outside done/; parking bug gone
rework-remarks: |
  Original parking (082/123 in decision/ with status done) no longer matches layout.
  After 082/123 stage-40 honesty rework, deliverables still ship; deferred dogfood
  cycle ACs keep both cards in todo/ — correct, not premature done. This chore's
  parking/sweep intent is closed by the resweep below.
---

# Task 164: Move the two finished decisions out of the decision queue

## Problem

`tasks/decision/` is the queue of decisions still to be made. Two of its three files were already
made when this was filed:

| file | `status:` (at file) | has `## Resolution` |
|---|---|---|
| `082-the-dogfood-loop-cannot-score-an-absent-section.md` | `done` | yes |
| `123-dogfood-loop-cannot-score-a-reserved-name-collision.md` | `done` | yes |
| `163-…` (unrelated; later closed elsewhere) | `todo` | no — was open |

The state directory and the `status:` field disagreed, and the directory is the one the tooling
walks. The done-finalize pass reads `tasks/done/`, so these two were never verified against their
deliverables and never archived — finished work the process could not see. Compare
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

### First pass (historical)

Surfaces were grepped, 082/123 moved into `tasks/done/`, and `tasks/decision/` left only open
work. Quality-review later **failed** that done placement: promotion text still claimed deleted
`60-evaluate.md`, and deferred runtime ACs were still open. Cards were reopened to `todo/` for
honesty rework (082 units + 123-ac-honest).

### Resweep after 082+123 honesty rework (2026-08-07)

**Criterion 1 — deliverables checked, not `status:`.**

| decision | claimed deliverable | measured |
|---|---|---|
| TASK-082 | `absent_section_route` + `per_absent_section` + stage-40 promotion | `ref-evaluation.md` has surface id + instances + next-action rubric; `40-evaluate.md:74` Cross-run promotion; no live verify binds deleted `60-evaluate.md` |
| TASK-123 | reserved namespace in `lifecycle_boundary.discover` + stage-40 promotion | `ref-evaluation.md:26` includes “or the reserved built-in command namespace”; promotion AC verifies `40-evaluate.md` |

In-repo resolution **shipped**. Deferred dogfood-cycle ACs remain open on both cards (runtime
only), so both correctly stay in `tasks/todo/` with `status: todo` — **reopened**, not archived.
Premature `done/` would repeat the QR failure.

**Criterion 2 — `tasks/decision/` after resweep: 0 files checked, 0 disagreements.**

Directory empty. Count checked: **0 / 0**.

**Criterion 3 — zone/status sweep (`status: done` while not in `done/`):**

| directory | files | `status: done` |
|---|---|---|
| `tasks/todo` | 3 | 0 |
| `tasks/blocked` | 0 | 0 |
| `tasks/plan` | 0 | 0 |
| `tasks/decision` | 0 | 0 |
| `tasks/doing` | 0 | 0 |
| `tasks/done` | 0 | 0 |

No parking disagreement remains. The three `todo/` cards are 082, 123, and this chore.

**Gate.** Surfaces and card honesty verified by grep; `make doc-check` not re-run for this
metadata-only resweep (no path moves).

## Notes

Whether the lasting fix is "move the files" or "make the pass read `status:` instead of the
directory" remains open product design — but the two must not stay in disagreement. The
directory-as-authority reading is what the rest of the process already assumes. This card closes
the observed parking bug and the post-rework resweep; it does not wait on dogfood cycle ACs on
082/123.
