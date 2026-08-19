---
id: TASK-194
title: "Three cards fail validation and the failure has become the baseline"
type: chore
priority: P3
effort: S
created-at: 2026-08-19T13:40:00+09:00
source: "measured 2026-08-19 — ce task validate --all reports 11 valid, 3 invalid on a clean tree"
scope: "dva repo — tasks/done/082, tasks/done/123, tasks/done/164; possibly the ce validator"
status: todo
---

# Task 194: Three cards fail validation and the failure has become the baseline

## Summary

`ce task validate --all` has reported **11 valid, 3 invalid** on a clean tree for several
sessions running. The three are always the same:

| card | errors |
|---|---|
| `tasks/done/082-…` | `Invalid type "decision"`, `Missing Summary`, `Missing Completion Criteria`, `No completion criteria defined` |
| `tasks/done/123-…` | the same four |
| `tasks/done/164-…` | `Invalid priority: P4` plus the same structural errors |

A gate that is known to fail is not a gate. Every session now has to carry "the baseline is
082/123/164" as tribal knowledge in order to tell a real regression from the standing
failure — which is exactly the arrangement that lets a fourth failure ride in unnoticed.

Two things are tangled here and both need a decision:

**The type exists as a directory but not in the validator.** `tasks/decision/` is a real
directory in this repo (empty today), and 082/123 carry `type: decision`. The validator
rejects that value. One of the two is wrong; nothing in the repo says which.

**The cards sit in `done/` with open work recorded only as prose.** `164:80` states
"Deferred dogfood-cycle ACs remain open on both cards"; `164:67` records that an earlier
`done/` placement was rejected in quality review because the promotion text still pointed at
a deleted `60-evaluate.md`, and that 082/123 were reopened to `todo/` at the time. They are
in `done/` again now. Whatever is still open is written as sentences, not `- [ ]` lines, so
no tool can see it and nothing will raise it again.

## Completion Criteria

- [ ] `ce task validate --all` exits 0 | verify: `ce task validate --all; echo "exit: $?"`
- [ ] `type: decision` is either accepted by the validator or absent from the repo, and `tasks/decision/` agrees with that answer | verify: human — read the decision recorded in this card, then `ls tasks/decision/ && grep -rl 'type: decision' tasks/`
- [ ] The deferred dogfood-cycle work named in 164's quality review is either re-registered as its own card or explicitly cancelled with a reason | verify: human — the prose claim in `tasks/done/164-*.md` no longer describes work that nothing tracks
- [ ] The three filenames are unchanged | verify: `ls tasks/done/082-* tasks/done/123-* tasks/done/164-*`

## References

- `tasks/done/082-the-dogfood-loop-cannot-score-an-absent-section.md`
- `tasks/done/123-dogfood-loop-cannot-score-a-reserved-name-collision.md`
- `tasks/done/164-two-resolved-decisions-are-parked-where-the-archival-pass-cannot-see-them.md` — its `quality-review-evidence:` block is where the open work is recorded
- The accepted frontmatter values are whatever `ce`'s `validator.go` enforces; the repo's own valid cards use `type:` ∈ {feature, bug, chore, docs} and `priority:` ∈ {P0…P3}

## Open Questions

- Fix direction is a maintainer call, not a cleanup detail: widen the validator to accept
  `decision`/`P4`, or normalize the three cards to the shape the validator already enforces.
  Widening keeps the historical record intact; normalizing keeps one definition of a task.
  **Ask before implementing.**
- Whether the deferred dogfood ACs are still worth doing at all — the stage collapse that
  orphaned them may have made the question moot.

## Technical Notes

- Filenames must not be renamed. The id prefix is fixed at registration time for P0–P3.
- `ls tasks/decision/` is empty today, so nothing depends on the directory's contents — only
  on whether the convention it implies is real.
