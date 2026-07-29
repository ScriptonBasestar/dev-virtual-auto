<!-- v:2026-07-16 -->

<constants>
SELF = workflows/dva-dogfood/60-evaluate.md
ROOT = workflows/dva-dogfood
METHODOLOGY = ./METHODOLOGY.md
ARTIFACTS = ROOT/ref-artifacts.md
EVALUATION = ROOT/ref-evaluation.md
SAFETY = ROOT/ref-safety.md
SESSION = ROOT/ref-session.md
</constants>

[EXECUTE IMMEDIATELY]

<role>DVA dogfood evaluator — compare baseline, validate, and assign
owners</role>

<objective>Determine whether the run hypothesis is confirmed using current
evidence.</objective>

<input>`RUN_DIR` from handoff. Resolve per SESSION; stage 20 and stage 50 when
attempted must be accepted. State must route to this stage.</input>

<steps>
1. Load METHODOLOGY and this stage's references per SESSION reference reuse —
   once per session, reused while unchanged. Read state, handoff,
   and latest accepted reports.
2. Create a unique ATTEMPT_ID; prior evaluations are comparison inputs, not blockers.
3. Re-run validation for changed layers with the exact DVA command selected in
   stage 50 when attempted; otherwise use the installed executable recorded by
   the baseline. Do not reuse historical success or substitute another binary.
4. Compare baseline/result for the exact hypothesis metrics.
5. Score every dimension in `<scoring>` below and explain non-maximum scores.
6. Classify CONFIRMED, PARTIAL, REJECTED, or INCONCLUSIVE.
7. Assign every unresolved finding one generic owner, one DVA owner route, and
   severity; check regressions/protected paths.
8. Identify but do not apply the smallest feedback action. If a cycle-owned
   regression stays within the run's existing generic `primary_owner`, keep the
   run active, reactivate its selected DVA owner-route stage, invalidate
   accepted downstream stage indexes while retaining their reports, and route
   to it. Clear candidate DVA provenance when reactivating stage 45. Do not
   invoke stage 70 until the changed state is evaluated again. Backlog
   secondary findings; a correction owned by a different generic owner starts
   a successor run with a fresh baseline and `predecessor_run_id`.
9. Otherwise set next prompt 70. Write the unique attempt report and update
   state; update handoff only at a SESSION boundary.
</steps>

<gate>Use the cycle gate in `<scoring>`; contradictory diagnostics prevent a full
Validation score.</gate>

<constraints>
- Read-only evaluation stage.
- Do not aggregate multiple next hypotheses, mutate two generic owners, or
  reassign failures for convenience.
</constraints>

<trigger>Evaluate the run, route changes or prompt 70, then continue or hand off
per SESSION.</trigger>

<scoring>

Score each applicable dimension from 0 to 2. Mark unrelated dimensions `N/A`.

<!-- markdownlint-disable MD013 -->

| Dimension    | 0                                         | 1                                     | 2                                              |
| ------------ | ----------------------------------------- | ------------------------------------- | ---------------------------------------------- |
| Triggering   | Skill absent/not triggered                | Explicit invocation only              | Correct explicit and natural triggering        |
| Correctness  | Wrong or unsafe result                    | Mostly correct with material warnings | Correct and evidence-backed                    |
| Reuse        | Target-specific logic embedded            | Some reusable separation              | Clean generic/local/project ownership          |
| Efficiency   | Repeated broad scans or excessive context | Minor duplication                     | Bounded scans and progressive disclosure       |
| Safety       | Secret/user/prod risk                     | Protected with gaps                   | Protected paths and approval gates enforced    |
| Validation   | No meaningful checks                      | Partial checks                        | Layered current-state checks                   |
| Runtime truth | Reported state ignores what DVA controls | Probe-only; port ownership unverified | Status/health reflect the tracked process owning its port |
| Ownership    | Same service/check has multiple owners    | Overlap found but incompletely routed | One lifecycle owner and one SSoT per behavior  |
| Traceability | Cannot explain result                     | Partial evidence                      | Baseline, diff, findings, owner, result linked |

<!-- markdownlint-enable MD013 -->

The score is diagnostic, not the cycle gate. Report earned and applicable
points, for example `12/14`; do not penalize `N/A`.

**Cycle gate.** Stage-60 evaluation PASS requires:

- comparable before/after evidence;
- all findings assigned to an owner;
- no unresolved critical/high regression introduced by the cycle;
- source skill installation and fresh-session behavior checked when skill
  changed;
- every Compose service, native process, check, and provision action has one
  lifecycle owner;
- Safety, Validation, and Ownership all scored 2.

Surfaces recorded `not_applicable` at stage 20 do not count against coverage;
an invented case does, as a Traceability zero.

Final cycle closure additionally requires one singular, measurable next
hypothesis selected by stage 70.

**Cross-run promotion.** A generic skill or prompt improvement remains
provisional after one target. Before presenting it as a reusable best practice,
validate it in a separate run against at least one structurally different target
— which, with target-derived cases, means a target that instantiates a different
surface set. Compare models only when model sensitivity is the stated
hypothesis; do not multiply models by default. Cross-run evidence may support
promotion but never replaces current-run gates.

</scoring>
