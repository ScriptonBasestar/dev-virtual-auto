---
id: TASK-082
title: "The dogfood evaluation files an absent surface as not-applicable, so no cycle can score how dva answers it"
type: decision
priority: P3
effort: M
created-at: 2026-07-30T00:00:00+09:00
scope: "workflows/dva-dogfood — ref-evaluation.md case manifest + stage 40 cross-run comparison"
status: todo
quality-review: fail
quality-reviewed-at: 2026-08-07T18:05:08+09:00
quality-review-evidence: |
  - kind: rework
    command-or-step: quality-review
    result: Promotion clause claimed in 60-evaluate.md (deleted in stage collapse); deferred ACs still [ ]; case_manifest_hash promotion not re-homed into 40-evaluate.
  - kind: unit
    unit: 082-surface-present
    command-or-step: rg -n 'absent_section_route|per_absent_section|next action' workflows/dva-dogfood/ref-evaluation.md
    result: ok — surface id + instances + next-action rubric all present (lines 37–39, 57–63)
  - kind: unit
    unit: 082-ac-honest
    command-or-step: rg -n '60-evaluate|case_manifest_hash|\[ \].*absent section|\[ \].*regression|\[ \].*not_applicable' tasks/todo/082-the-dogfood-loop-cannot-score-an-absent-section.md
    result: ok — scope/cost/ACs retargeted to stage 40; promotion clause AC verifies 40-evaluate; runtime ACs left [ ] open
rework-remarks: |
  Promotion clause re-homed (unit 082-rehome-promotion): stage 40 gate + ref-evaluation
  hash derivation/compatibility wording + evaluation.case_manifest_hash in ref-artifacts.
  Unit 082-surface-present: confirmed absent_section_route + per_absent_section
  (a next action / b what-config-declares / c --json) still shipped in ref-evaluation.md.
  Unit 082-ac-honest: no AC claims live 60-evaluate.md; promotion note verify binds
  to 40-evaluate; scope/cost narrative say stage 40. Residual: deferred runtime ACs
  (absent scored case, post-hash regression check, not_applicable retention) still open
  until a dogfood cycle runs against a stack-only fixture.
---

# Task 082: Decide whether an absent section is a case

## The blind spot

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

## Acceptance criteria

- [x] A decision is recorded here with its rationale | verify: `human — this file names the chosen option (A, in the Resolution above)`
- [x] The cross-run-promotion note reaches stage 40 (not deleted `60-evaluate.md`) | verify: `rg -n 'Cross-run promotion|case_manifest_hash' workflows/dva-dogfood/40-evaluate.md` — prints the hash-delta-is-a-promotion clause at lines 74–79
- [ ] If A or C: an absent section produces a scored case | verify: `human — run one cycle against a stack-only fixture and read evaluation.cases for the absent applications section` — **deferred to the next dogfood cycle; the surface is in the manifest**
- [ ] If A: the first post-change run is not reported as a regression | verify: `human — stage 40 output on the run after the hash change` — **promotion clause file binding is the [x] AC above; this AC is runtime-only and stays open until a cycle runs**
- [ ] `not_applicable_surfaces` still records genuinely unevaluable surfaces | verify: `human — a compose-less target still files the compose surface as not applicable` — **deferred to the next cycle; the `per_absent_section` bullet scopes not-applicable explicitly**

