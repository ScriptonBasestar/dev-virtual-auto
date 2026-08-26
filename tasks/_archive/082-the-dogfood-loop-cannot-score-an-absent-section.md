---
id: TASK-082
title: "The dogfood evaluation files an absent surface as not-applicable, so no cycle can score how dva answers it"
type: chore
priority: P3
effort: M
created-at: 2026-07-30T00:00:00+09:00
scope: "workflows/dva-dogfood — ref-evaluation.md case manifest + stage 40 cross-run comparison"
status: done
quality-review: conditional
quality-reviewed-at: 2026-08-19T15:33:33+09:00
verified-at: 2026-08-19T15:37:48+09:00
archived-at: 2026-08-19T15:37:48+09:00
quality-review-evidence: |
  - kind: rework
    command-or-step: quality-review
    result: Promotion clause claimed in 60-evaluate.md (deleted in stage collapse); deferred ACs still [ ]; case_manifest_hash promotion not re-homed into 40-evaluate.
  - kind: unit
    unit: 082-surface-present
    command-or-step: rg -n 'absent_section_route|per_absent_section|next action' workflows/dva-dogfood/ref-evaluation.md
    result: ok — surface id + instances + next-action rubric all present
  - kind: unit
    unit: 082-ac-honest
    command-or-step: stage-40 retarget of ACs
    result: ok — promotion clause verifies 40-evaluate; runtime ACs left open
  - kind: dogfood
    unit: gorisa-20260807-193617-91531d
    command-or-step: workflows/dva-dogfood full cycle MODE=step on gorisa-devbox
    result: PARTIAL — absent_section_route not_applicable (stack+plans present; applications not a live command family); surface amended (commit 0a58cf5) to derive sections from installed DVA; promotion hash for next baseline recorded
  - kind: fixture
    unit: 082-fixture-absent-plans
    command-or-step: workflows/dva-dogfood/fixtures/absent-plans-one-reserved case derivation
    result: ok — case_ids include absent_section_route:plans; validate rc 0; EVIDENCE.md
  - kind: re-review
    command-or-step: "re-review 2026-08-19 — the fail above is the 2026-08-07 verdict, kept as the record it was; this entry and those below supersede it"
    result: conditional. All three rework causes are gone — the promotion clause lives in 40-evaluate.md, the card's `60-evaluate.md` mentions are now historical prose only ("re-homed from deleted ..."), and 0 criteria are open (5 of 5 [x])
  - kind: automated
    command-or-step: "AC2 binding verbatim — grep -n 'Cross-run promotion|case_manifest_hash' workflows/dva-dogfood/40-evaluate.md"
    result: exit 0, prints line 74. FINDING — the criterion's suffix says the clause sits at "lines 74-79"; it runs 74-80, ending "never replaces current-run gates." with </gate> at 81. Off by one, in the prose half; the command half is correct. 123 carries the identical sentence
  - kind: automated
    command-or-step: "AC3 premise re-derived from the fixture rather than from EVIDENCE.md"
    result: fixtures/absent-plans-one-reserved/dva.yml has version, stack and interaction and no plans key; case_ids.txt holds 4 ids including absent_section_route:plans. The premise the criterion asserts is what the fixture actually is
  - kind: manual
    command-or-step: "AC4 and AC5 — both cite dogfood run 20260807-193617-91531d"
    result: |
      FINDING - not re-verifiable. The run directory is gone and .gitignore:34 excludes
      tmp/, so neither the frozen/post-edit hashes nor state.yaml's
      not_applicable_surfaces can be re-derived by anyone reading this card. The ticks
      rest on the kind: dogfood entry recorded above at the time, not on an artifact.
      The durable half of AC4 - that the clause naming a hash delta as promotion exists
      in 40-evaluate.md - does re-derive. Recorded so a later reader does not mistake
      these two for re-checkable.
  - kind: automated
    command-or-step: "normalization directed by TASK-194, verified after the fact"
    result: type chore, priority P3, both `## Summary` and `## Completion Criteria` present; ce task validate --all exits 0 at 4 valid, 0 invalid
rework-remarks: |
  Residual AC closed via fixture absent-plans-one-reserved (2026-08-08). All ACs [x].
  Dogfood run 20260807-193617-91531d (gorisa): applications is not a command family
  on DVA 0.1.44 (`dva app` unknown). Stage 10 correctly did not invent an
  applications case. Stage 20 fixed ref-evaluation discover/derivation (committed
  0a58cf5). Residual runtime ACs need a target/fixture missing a *live* section
  (stack or plans), not applications.
verification-summary: |
  Archived at quality-review: conditional. Every criterion re-derived from artifacts
  rather than from the card: the cross-run-promotion clause is in 40-evaluate.md at
  line 74, the fixture's dva.yml carries stack and interaction with no plans key, and
  case_ids.txt holds absent_section_route:plans among four ids.

  Two findings ride along and are why this is conditional, not pass. The third
  criterion's suffix places the clause at lines 74-79 where it runs 74-80 — the rg
  command it binds to is correct, only the sentence describing the output is off, and
  123 carries the same sentence. AC4 and AC5 cite dogfood run 20260807-193617-91531d,
  whose directory is gone under a tmp/ excluded at .gitignore:34, so the frozen and
  post-edit hashes and state.yaml's not_applicable_surfaces are recorded in this card's
  evidence block and nowhere re-checkable.
---

# Task 082: Decide whether an absent section is a case

## Summary

`workflows/dva-dogfood/ref-evaluation.md` routes cases by declared surface, and states that a
surface with **no** instance in the target is not a case: the absence is recorded under
`evaluation.not_applicable_surfaces` as evidence and generates no work.

That is defensible for coverage — you cannot evaluate a compose runner in a project with no
compose files. It is wrong for *messages*, because the absence is exactly when dva has to say
something, and what it says is what the user reads.

[TASK-074](../_archive/074-app-subcommands-answer-an-absent-section-three-ways.md) is the proof.
Seven `dva app` subcommands answered an absent `applications:` section three incompatible ways —
two capitalizations, both exit codes, and one subcommand asserting the named app was "not found"
in a set that did not exist. Nineteen cycles ran over it. None could have scored it, because
every one of those cycles filed `applications` under `not_applicable_surfaces` and moved on.

The same blind spot covers every other section. `dva stack`, `dva app`, and the plan commands all
have an empty-section path, and none of them has ever been evaluated.

## The decision

Adding a case type is cheap; the cost is in the comparison.

**A. Add an `absent_section_route` surface** with `instances: per_absent_section`, scored on
whether the command (a) states a next action, (b) states what the config *does* declare, and
(c) is parseable under `--json`. Turns every absent section into one scored case, which is the
only way this class gets found by the loop instead of by a user.

**B. Leave the manifest alone** and rely on tasks like 074 being filed by hand. Honest about what
the loop does, but it means the loop's score is silent about the messages a compliant config
actually hits.

**C. Score it without a manifest change** — extend the existing per-surface prompts to ask about
the absent path too. Cheapest, but it hides a new criterion inside an old case, so a score
movement cannot be attributed.

Recommendation: **A**. It is the only option where the finding class shows up in the score.

Note that (c) currently fails for every command on this path — see
[TASK-079](../_archive/079-json-flag-does-not-cover-failures.md). Adding the criterion before that is
fixed means every instance scores low on it at once, which is accurate but will look like a
regression.

## The cost that needs deciding with it

Changing the manifest bytes changes `case_manifest_hash`. Stage 40 compares against the previous
run by that hash, so the first run after the change is not comparable to the one before it and
must be treated as a **cross-run promotion**, not a regression. Whoever takes this has to say in
the task how stage 40 learns that — a recorded baseline reset, or a hash-change branch in the
comparison itself. (The former stage-60 evaluate file is gone; the live clause is in
`40-evaluate.md`, re-homed from deleted `60-evaluate.md`.)

## Resolution

**Decision: A.** An `absent_section_route` surface is the only option where the finding class
shows up in the score, and the loop's whole purpose is to find what 074 found by hand.

Shipped, coupled with [TASK-123](123-dogfood-loop-cannot-score-a-reserved-name-collision.md) as
one manifest edit so the two surface changes share a single `case_manifest_hash` bump:

- `workflows/dva-dogfood/ref-evaluation.md` — added the `absent_section_route` surface
  (`instances: per_absent_section`) before `no_change`; added the `per_absent_section` dispatch
  bullet, which states the inversion (the absent section is the instance, not a non-instance) and
  carries the (a) next-action / (b) what-config-declares / (c) `--json`-parseable rubric in-place.
- `workflows/dva-dogfood/40-evaluate.md` — one cross-run-promotion clause (re-homed from
  deleted `60-evaluate.md`): a run whose `case_manifest_hash` differs from its
  predecessor's is itself a promotion, so the manifest-induced case-set delta is not
  reported as a regression. `ref-evaluation.md` records hash derivation and tuple
  compatibility; `ref-artifacts.md` carries `evaluation.case_manifest_hash`.

The (c) caveat at line 52-55 is now partly moot: [TASK-079](../_archive/079-json-flag-does-not-cover-failures.md)
shipped the `--json` failure envelope, so the route commands are `--json`-parseable on the
absent-section path (measured: `app up myapp --json` emits `{"error":{…}}`). Criterion (c) is no
longer universally failing on day one.

Criteria 2-4 are runtime verifications: they fire when the dogfood loop next runs against a
stack-only target, not at this edit. The manifest + stage-40 promotion clause is the
actionable scope.

## Completion Criteria

- [x] A decision is recorded here with its rationale | verify: `human — this file names the chosen option (A, in the Resolution above)`
- [x] The cross-run-promotion note reaches stage 40 (not deleted `60-evaluate.md`) | verify: `rg -n 'Cross-run promotion|case_manifest_hash' workflows/dva-dogfood/40-evaluate.md` — prints the hash-delta-is-a-promotion clause at lines 74–80
- [x] If A or C: an absent **live** command-family section produces a scored case | verify: fixture `workflows/dva-dogfood/fixtures/absent-plans-one-reserved` (plans absent, stack present) derives `absent_section_route:plans` — see `EVIDENCE.md` / `case_ids.txt`; applications not invented
- [x] If A: a case_manifest_hash delta is treated as promotion, not regression | verify: dogfood stage 40 run `20260807-193617-91531d` records frozen hash `2b72f5f5…` → post-edit `33561703…` as cross-run promotion for the next baseline (clause in `40-evaluate.md`)
- [x] `not_applicable_surfaces` still records genuinely unevaluable surfaces | verify: gorisa dogfood stage 10 filed `absent_section_route` not_applicable with evidence (stack+plans present; applications not live on installed binary) — see `$RUN_DIR/state.yaml`

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
