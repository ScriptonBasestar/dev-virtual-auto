---
id: TASK-194
title: "Three cards fail validation and the failure has become the baseline"
type: chore
priority: P3
effort: S
created-at: 2026-08-19T13:40:00+09:00
source: "measured 2026-08-19 — ce task validate --all reports 11 valid, 3 invalid on a clean tree"
scope: "dva repo — tasks/done/082, tasks/done/123, tasks/done/164; possibly the ce validator"
status: done
completed-at: 2026-08-19T15:27:01+09:00
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

## Decision (2026-08-19)

**Normalize the three cards. The validator is not widened — `type: decision` and
`priority: P4` are card defects.** Reviewed against `ce-agent-kit` at `c99d1921`; no code
was changed there.

> `type:` names what kind of work a card is and must stay stable for the card's whole
> lifecycle; `decision` names a workflow state the card passes through (`decision/` →
> `todo`/`done`) and is separately already a distinct CE document kind with its own ADR
> schema under `decisions/`, so admitting it as a task type would both make `type:` change
> on every zone transition and collide with an existing schema. `P4` is rejected because the
> filename convention (`P[0-3]`) and every priority scale in CE and the engine stop at P3,
> so accepting it in frontmatter alone would split one convention against itself.

### Why `decision` is a state, not a type

ADR-0003, implemented at `internal/usecase/task/canonical_validator.go:222-234`, holds that
the document — not its directory — is the source of truth for its *kind*, because the
directory encodes workflow state and changes as work progresses. The task_management engine
files `decision` on the state side of exactly that line: `engine/core/decision-decide.yaml`
declares it under `source_state_dirs`, with transitions `decision-to-todo` and
`decision-to-done`.

The concrete failure is that `type:` must survive a card's whole lifecycle — `depends-on`,
`blocks`, `children` and prose links all quote the card's identity — but a card in the
`decision` state moves on to `todo` and then `done`. If `decision` were a type, the type
would have to change at each transition, or go stale. **082 and 123 are that staleness,
observed**: both sit in `tasks/done/`, having long left the decision state, and still carry
`type: decision`. The field records where the card once was, not what kind of work it is.

Independently, CE already models a decision as a first-class *document kind* with its own
schema rather than as a task type. `isDecisionDoc()` (`canonical_validator.go:109`) routes
`decisions/`, `decisions/adr/`, `docs/adr/` and the legacy `tasks/decision/` to
`validateDecisionDoc()` (`:128`), which enforces ADR markdown — `Status`, `## Context`,
`## Decision`, `## Rationale`, `## Consequences` — and bypasses canonical task YAML
entirely. So `type: decision` is not a gap in the enum; it collides with a modeled concept
that has a different schema and a different home.

That resolves this card's "the type exists as a directory but not in the validator": the
directory and the validator do not actually disagree. `tasks/decision/` is a *state* zone,
and CE recognizes it as a location for ADR-shaped documents. Neither reading makes
`decision` a value of `type:`.

### Why `P4` is rejected

`taskFilenameRe` (`internal/usecase/task/validator.go:19`) admits `P[0-3]` only, and
`validator_test.go:362` already asserts that `P4-bad.md` is rejected, commented "priority out
of range". Since this card requires that filenames not change, widening the frontmatter enum
would set the two halves of one convention against each other: `priority: P4` legal in YAML
while `P4-foo.md` stays illegal as a filename.

P4 is undefined everywhere else as well — `schema.todo.md` (P0–P3), the engine's
`20-file-schema.md` (P1–P3), CE issues (P0–P2). Per-kind *narrowing* within a defined scale
is an established pattern there; extending past the scale's range is not, and would have the
validator accept a rank nothing in the system can order.

### Widening would not have cleared the gate anyway

`requireHeading()` (`canonical_validator.go:441`) matches `^##\s+<exact>\s*$` — anchored and
case-sensitive. All three cards use `## The blind spot` and `## Acceptance criteria`, which
fail the required `## Summary` and `## Completion Criteria` regardless of type and priority.
With both enums widened, `ce task validate --all` would still report 11 valid, 3 invalid.
Normalizing the cards is required under either verdict; widening would only have removed two
of three error classes, while permanently weakening the vocabulary.

### Normalization this decision directs

- `082`, `123` — drop `type: decision`. A card that *records* a decision belongs in
  `decisions/` under the ADR schema; one that records *work following from* a decision keeps
  its place in `tasks/done/` and takes a work type (`chore` or `refactor`). Both read as the
  latter.
- `164` — `priority: P4` → `P3`. The card is already in `done/`, so priority is historical.
- all three — `## The blind spot` → `## Summary`, `## Acceptance criteria` →
  `## Completion Criteria`, with at least one `- [ ]`/`[x]`/`[>]` item under the latter.
- filenames stay as they are; none of this touches `taskFilenameRe`.

## Completion Criteria

- [x] `ce task validate --all` exits 0 | verify: `ce task validate --all; echo "exit: $?"`
- [x] `type: decision` is either accepted by the validator or absent from the repo, and `tasks/decision/` agrees with that answer | verify: human — read the decision recorded in this card, then `ls tasks/decision/ && grep -rl 'type: decision' tasks/`
- [x] The deferred dogfood-cycle work named in 164's quality review is either re-registered as its own card or explicitly cancelled with a reason | verify: human — the prose claim in `tasks/done/164-*.md` no longer describes work that nothing tracks
- [x] The three filenames are unchanged | verify: `ls tasks/done/082-* tasks/done/123-* tasks/done/164-*`

## Resolution

Normalized, as the decision above directs. `ce task validate --all` exits 0.

| binding | result |
| --- | --- |
| `ce task validate --all; echo "exit: $?"` | `Summary: 4 valid, 0 invalid (total: 4)`, exit 0 |
| `ls tasks/decision/` | `No such file or directory` — see below |
| `grep -rl '^type: decision' tasks/` | 0 files |
| `ls tasks/done/082-* tasks/done/123-* tasks/done/164-*` | all three listed, exit 0 |

The count moved from 3 invalid to 0 invalid out of **4**, not out of 14: thirteen cards were
archived earlier in the same run, and `validate --all` does not read `tasks/_archive/`.

### `tasks/decision/` is not in the repo

This card's Summary calls it "a real directory in this repo (empty today)", and that is the
half of the premise that does not survive checking. Git does not store empty directories, so
it is tracked nowhere — `git ls-files tasks/decision` returns 0. It exists on disk only in
the primary checkout, left over from when it held files; a fresh worktree of the same commit
has `_archive/`, `done/` and `todo/` and nothing else. Three sibling zone directories,
`blocked/`, `doing/` and `plan/`, are in the identical state.

Forty commits touched `tasks/decision/*`. The last is `2697295` (2026-08-07), which deleted
`163-decide-whether-detectedproject-…md` — one of the eleven archived cards this task went on
to normalize. The state zone and the stale `type:` are one history seen twice.

That strengthens the decision rather than changing it: the directory cannot have been
disagreeing with the validator, because from git's point of view it is not there.

### Beyond the three cards the decision names

AC2 asks for `type: decision` to be absent from *the repo* and greps all of `tasks/`.
Normalizing only the three live cards would have left that grep answering eleven, so the
eleven archived cards carrying it were swept as well (`80376cd`), each keeping its former
value in a `normalized-by:` field rather than only in the commit.

The anchored grep now returns 0. A bare `grep -rl 'type: decision' tasks/` still returns 14,
every one of them prose — this card's own argument plus the `normalized-by:` lines that quote
the string. Anchor the pattern when re-checking.

### Deliberately not done

`tasks/_archive/` diverges from the enum far more widely than this card noticed, and is left
that way:

| value | archived cards | reachable by `validate --all` |
| --- | --- | --- |
| `type: fix` | 57 | 0 |
| `type: plan`, `feat`, `enhancement` | 1 each | 0 |
| `priority: P4` | 18 | 0 |

The eleven were swept because an acceptance criterion on this card greps for that exact
string. Nothing greps for `fix`, and rewriting sixty terminal records to satisfy a gate that
never reads them would be churn. `164`'s `P4` moved for the same rule read the other way — it
sits in `tasks/done/`, which the validator does read.

### AC3 — the deferred dogfood work

Neither re-registered nor cancelled: re-measured, and already closed. The claim at `164:80`
was true when written on 2026-08-07 and stale a day later. It is kept as written and marked
`### Superseded (2026-08-19)` inside 164 rather than deleted, with the fixture evidence
re-observed here — `case_ids.txt` carries `absent_section_route:plans` and exactly one
`lifecycle_boundary:up`, the cross-run promotion clause reads from `40-evaluate.md:74`,
`60-evaluate.md` is gone, and both cards have 0 open checkboxes.

## References

- `tasks/done/082-the-dogfood-loop-cannot-score-an-absent-section.md`
- `tasks/done/123-dogfood-loop-cannot-score-a-reserved-name-collision.md`
- `tasks/done/164-two-resolved-decisions-are-parked-where-the-archival-pass-cannot-see-them.md` — its `quality-review-evidence:` block is where the open work is recorded
- The accepted frontmatter values are whatever `ce`'s `validator.go` enforces: `type:` ∈ {feature, bug, chore, refactor, cleanup, docs, test} and `priority:` ∈ {P0…P3}. Measured 2026-08-19, every card the validator can reach uses `type: chore` and `priority: P3`; `tasks/_archive/` holds nine distinct `type:` values, four of them off that enum — see `## Resolution`

## Open Questions

- ~~Fix direction is a maintainer call: widen the validator, or normalize the three cards.~~
  **Resolved 2026-08-19 — see `## Decision` above: normalize the cards, do not widen the
  validator.** Implementation may proceed without asking again.
- ~~Whether the deferred dogfood ACs are still worth doing at all — the stage collapse
  that orphaned them may have made the question moot.~~ **Resolved 2026-08-19 — neither:
  they were completed on 2026-08-08, one day after the review that called them open.**
  The stage collapse is what closed them, not what orphaned them; `60-evaluate.md` is
  gone and the promotion it carried reads from `40-evaluate.md:74`. See
  `## Resolution § AC3`.

## Technical Notes

- Filenames must not be renamed. The id prefix is fixed at registration time for P0–P3.
- `ls tasks/decision/` is empty today, so nothing depends on the directory's contents — only
  on whether the convention it implies is real.
