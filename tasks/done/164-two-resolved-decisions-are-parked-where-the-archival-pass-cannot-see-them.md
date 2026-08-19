---
id: TASK-164
title: "Two resolved decisions sit in tasks/decision/ with status done, where the archival pass never looks"
type: chore
priority: P3
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

## Summary

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

## Completion Criteria

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

### Superseded (2026-08-19)

The paragraph above is kept as written and is no longer true. It was measured on 2026-08-07;
the deferred dogfood-cycle ACs were closed the next day by the fixture
`workflows/dva-dogfood/fixtures/absent-plans-one-reserved`, and both cards record the closure
in their `rework-remarks`. Re-observed at the time of this note, not read from those remarks:

| claim | measured 2026-08-19 |
|---|---|
| fixture exists | `workflows/dva-dogfood/fixtures/absent-plans-one-reserved/` — `case_ids.txt`, `dva.yml`, `compose.yaml`, `EVIDENCE.md` |
| 082's absent live section case | `case_ids.txt` contains `absent_section_route:plans` |
| 123's exactly-one reserved case | `case_ids.txt` contains exactly one `lifecycle_boundary:up` |
| promotion clause re-homed | `workflows/dva-dogfood/40-evaluate.md:74` — `**Cross-run promotion.**` |
| `60-evaluate.md` | absent from `workflows/`, as the review said |
| open checkboxes on 082 / 123 | 0 and 0 |

So nothing here describes work that no card tracks: the work was finished, and both cards are
in `done/` with every criterion ticked. Recorded rather than deleted, because the failure this
card exists to prevent is a claim outliving its measurement.

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

## Normalization (TASK-194, 2026-08-19)

Frontmatter and section headings were brought onto the schema that `ce task validate` and the
task_management engine both enforce. The filename is unchanged.

| field | was | now |
|---|---|---|
| `priority` | `P4` | `P3` |
| heading | `## Problem` | `## Summary` |
| heading | `## Acceptance criteria` | `## Completion Criteria` |

`P4` is defined nowhere in the system: `taskFilenameRe` admits `P[0-3]`, the engine's file
schema stops at P3, and CE issues stop at P2. Since this card's filename may not change,
accepting `P4` in frontmatter alone would have split one convention against itself. The card
is already in `done/`, so its priority is historical. Full reasoning: `tasks/done/194-*.md`,
`## Decision (2026-08-19)`.
