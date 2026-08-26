---
id: TASK-123
title: "The dogfood evaluation treats the reserved command namespace as a non-owner, so a reserved-name interaction collision is never a case"
type: chore
priority: P3
effort: M
created-at: 2026-07-31T00:00:00+09:00
scope: "workflows/dva-dogfood — ref-evaluation.md lifecycle_boundary surface + stage 40 cross-run comparison"
status: done
quality-review: conditional
quality-reviewed-at: 2026-08-19T15:34:28+09:00
verified-at: 2026-08-19T15:37:48+09:00
archived-at: 2026-08-19T15:37:48+09:00
quality-review-evidence: |
  - kind: rework
    command-or-step: quality-review
    result: AC marked [x] for 60-evaluate.md:105-109 but file is gone; deferred reserved-name case AC still open; re-home promotion clause under current stages.
  - kind: unit
    unit: 123-ac-honest
    command-or-step: stage-40 retarget of ACs
    result: ok — promotion AC verifies 40-evaluate; reserved discover still present
  - kind: dogfood
    unit: gorisa-20260807-193617-91531d
    command-or-step: stage 10 freeze + stage 30 forward-test lifecycle_boundary:up|build
    result: ok — reserved names instantiate cases (up, build); FT confirmed dual ownership (built-in + hooks); singleton fixture AC still open
  - kind: fixture
    unit: 123-fixture-one-reserved
    command-or-step: workflows/dva-dogfood/fixtures/absent-plans-one-reserved case derivation
    result: ok — exactly one lifecycle_boundary:up; EVIDENCE.md
  - kind: re-review
    command-or-step: "re-review 2026-08-19 — the fail above is the 2026-08-07 verdict, kept as the record it was; this entry and those below supersede it"
    result: conditional. Its three causes are gone — no criterion cites the deleted 60-evaluate.md any more (the four remaining mentions are historical prose), the singleton reserved-name criterion closed via the fixture on 2026-08-08, and the promotion clause is re-homed in 40-evaluate.md. 5 of 5 criteria [x], 0 open
  - kind: automated
    command-or-step: "AC2 binding verbatim — /usr/bin/grep -n 'reserved' workflows/dva-dogfood/ref-evaluation.md"
    result: exit 0, one hit at :26 — the discover clause naming "the reserved built-in command namespace" as an owner alongside stack, plans and interaction. This is the widening the criterion claims
  - kind: automated
    command-or-step: "AC5 premise re-derived against internal/config/reserved.go rather than against the fixture's own comment"
    result: |
      reservedCommands holds 23 names; `up` is one of them and `fixture-ping` is not.
      The fixture therefore declares exactly one reserved-name interaction, and
      case_ids.txt carries exactly one lifecycle_boundary id (`:up`) out of four ids,
      with no `:build`. Premise and result both hold, checked independently of the
      "must not add a second reserved key" comment that asserts it in the fixture
  - kind: automated
    command-or-step: "AC3 binding verbatim — grep -n 'Cross-run promotion|case_manifest_hash' workflows/dva-dogfood/40-evaluate.md"
    result: exit 0, prints line 74. FINDING — the criterion's suffix says "lines 74-79"; the clause runs 74-80. Identical sentence to 082's third criterion; the command half is correct in both
  - kind: manual
    command-or-step: "AC4 — cites dogfood run 20260807-193617-91531d (stage 10 freeze, stage 30 forward-test)"
    result: |
      FINDING - not re-verifiable. The run directory is gone and .gitignore:34 excludes
      tmp/, so neither the frozen lifecycle_boundary:up and :build cases nor the
      forward-test ownership output can be re-derived. The tick rests on the kind:
      dogfood entry recorded above at the time. Recorded so a later reader does not
      mistake it for re-checkable. Same limitation as 082 AC4/AC5, same run
  - kind: automated
    command-or-step: "normalization directed by TASK-194, verified after the fact"
    result: type chore, priority P3, both `## Summary` and `## Completion Criteria` present; ce task validate --all exits 0 at 4 valid, 0 invalid
rework-remarks: |
  Singleton reserved-name AC closed via fixture absent-plans-one-reserved (2026-08-08). All ACs [x].
  Dogfood run 20260807-193617-91531d (gorisa): interaction.up (before) and
  interaction.build (replace) produced lifecycle_boundary:up and :build cases;
  forward-test children explained ownership correctly. Residual: exactly-one-case
  AC still wants a fixture with a single reserved-name interaction (gorisa has two).
verification-summary: |
  Archived at quality-review: conditional. The singleton criterion — one reserved-name
  interaction yields exactly one case — was re-derived against internal/config/reserved.go
  rather than against the fixture comment that asserts it: the map holds 23 names, `up` is
  among them and `fixture-ping` is not, and case_ids.txt carries exactly one
  lifecycle_boundary id with no :build. The discover clause naming the reserved built-in
  namespace as an owner prints at ref-evaluation.md:26.

  Conditional for two carried findings: the promotion criterion says lines 74-79 where the
  clause runs 74-80 (identical sentence in 082), and AC4 cites dogfood run
  20260807-193617-91531d, unrecoverable under a gitignored tmp/.
---

# Task 123: Decide whether a reserved-name interaction collision is a case

## Summary

`workflows/dva-dogfood/ref-evaluation.md` declares the `lifecycle_boundary`
surface with `instances: per_overlap`, discovering:

> a service or process owned by more than one of **stack, plans, applications, interaction**

All four owners are config sections. The collision [TASK-076](../_archive/076-manifest-advertises-the-one-invocation-that-cannot-reach-the-interaction.md)
fixed — an `interaction:` key that shadows a reserved builtin command — is between
`interaction` and the **reserved command namespace**, which is not a config section and is not
on that list. So the overlap never instantiated a case, and the quality of dva's answer
(advertising an invocation that could not reach the interaction) was never scored. This is the
mirror of [TASK-082](082-the-dogfood-loop-cannot-score-an-absent-section.md): 082's blind spot
was an absent section, this one's is a second owner that is not a section at all. Both kept the
loop from seeing the same family of discovery defect.

## The decision (proposed)

**Widen `lifecycle_boundary`'s discover clause to count the reserved command set as an owner.**

Today the surface instantiates one case per overlap among four section owners. A reserved-name
interaction is owned by both `interaction` (the section) and the builtin command of the same
name, so widening the owner set makes every reserved-name interaction an instance — exactly the
shape whose manifest/ls/usage_example answer 076 had to fix by hand.

This prefers widening an existing surface over adding a new one, for the reason 082 already
recorded: the loop's value is fewer surfaces that each cover more, not a surface per defect. The
alternative — a standalone `reserved_collision` surface — is rejected for the same reason 082
rejected a standalone route surface.

## The cost that needs deciding with it

Identical to 082's, and shared with it: changing the manifest bytes changes `case_manifest_hash`,
so stage 40 must treat the first run after this lands as a **cross-run promotion, not a
regression**. If 082 and 123 are both going to land, they should land together — two manifest
edits mean two hash bumps, and staging them as one avoids a spurious regression between them.
(The former stage-60 evaluate file is gone; the live clause is in `40-evaluate.md`, re-homed
from deleted `60-evaluate.md`.)

This task does not block on 082 and 082 does not block on it; the coupling is only that the
hash-bump handling is the same mechanic, so whoever runs the first cycle after either lands
should know the other may follow.

## Non-goals

- Do not change the product behavior 076 shipped. The manifest `shadowed_by_builtin` mark,
  the `dva run <name>` usage example, and the rewritten warning are correct; this task is about
  whether the loop would *catch* a regression in them.
- Do not add reserved-name collisions as a `config_schema` instance. They are a routing/ownership
  question, not a schema question; `lifecycle_boundary` is the right surface.

## Resolution

**Decision: taken — widen `lifecycle_boundary`'s owner set.** A new standalone surface was
rejected for the same reason [TASK-082](082-the-dogfood-loop-cannot-score-an-absent-section.md)
rejected a standalone route surface: the loop values fewer surfaces that each cover more. The
existing `per_overlap` instantiation already yields one case per overlap, so once the reserved
command namespace is an owner, a reserved-name interaction (owned by both `interaction` and the
builtin of the same name) instantiates exactly one case with no new instance type.

Shipped as one half of a coupled edit with 082, so the two surface changes share a single
`case_manifest_hash` bump rather than two:

- `workflows/dva-dogfood/ref-evaluation.md` — `lifecycle_boundary.discover` widened to
  `…, or the reserved built-in command namespace`.
- `workflows/dva-dogfood/40-evaluate.md` — the shared cross-run-promotion clause (re-homed from
  deleted `60-evaluate.md`): a `case_manifest_hash` delta is itself a promotion, not a
  regression. `ref-evaluation.md` records hash derivation and tuple compatibility;
  `ref-artifacts.md` carries `evaluation.case_manifest_hash`.

Criterion 4 is a runtime verification: it fires when stage 20 next runs against a fixture with a
reserved-name interaction, not at this edit. The manifest + stage-40 promotion clause is the
actionable scope.

## Completion Criteria

- [x] The decision is recorded as taken or deferred in this file's Resolution | verify: `human — the Resolution above records the decision as taken`
- [x] If taken, `lifecycle_boundary`'s discover clause names the reserved command set as an owner | verify: `/usr/bin/grep -n 'reserved' workflows/dva-dogfood/ref-evaluation.md` — prints the widened discover clause
- [x] The cross-run-promotion note reaches stage 40 (not deleted `60-evaluate.md`) | verify: `rg -n 'Cross-run promotion|case_manifest_hash' workflows/dva-dogfood/40-evaluate.md` — prints the hash-delta-is-a-promotion clause at lines 74–80
- [x] A reserved-name interaction instantiates at least one lifecycle_boundary case | verify: dogfood stage 10 on gorisa froze `lifecycle_boundary:up` and `lifecycle_boundary:build`; stage 30 FT confirmed dual ownership for both
- [x] A fixture with **exactly one** reserved-name interaction instantiates **exactly one** case | verify: fixture `workflows/dva-dogfood/fixtures/absent-plans-one-reserved` has only `interaction.up` reserved → case_ids contain exactly `lifecycle_boundary:up` — see `EVIDENCE.md` / `case_ids.txt`

## Related

- [TASK-082](082-the-dogfood-loop-cannot-score-an-absent-section.md) — the sibling blind spot
  (absent section). Same hash-bump cost; land together if both go.
- [TASK-076](../_archive/076-manifest-advertises-the-one-invocation-that-cannot-reach-the-interaction.md)
  — the product fix whose regression this surface would catch. Its "Related — the loop that
  should have caught this" section is the origin of this task.
- [TASK-074](../_archive/074-app-subcommands-answer-an-absent-section-three-ways.md) — the other
  discovery defect 082/123 exist to keep watch over.

## Normalization (TASK-194, 2026-08-19)

Frontmatter and section headings were brought onto the schema that `ce task validate` and the
task_management engine both enforce. The filename is unchanged.

| field | was | now |
|---|---|---|
| `type` | `decision` | `chore` |
| heading | `## The blind spot` | `## Summary` |
| heading | `## Acceptance criteria` | `## Completion Criteria` |

`decision` names a workflow *state* a card passes through (`decision/` → `todo` → `done`), not
a kind of work, and `type:` has to survive the card's whole lifecycle. This card left the
decision state long ago and sat in `done/` still declaring `type: decision`, which recorded
where it had been rather than what it is. CE separately models a decision as its own document
kind with an ADR schema under `decisions/`, so the value collided with a live concept instead
of filling a gap. Full reasoning: `tasks/done/194-*.md`, `## Decision (2026-08-19)`.
