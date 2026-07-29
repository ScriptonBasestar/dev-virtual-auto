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

<!-- contract:stage id=60 mode_step=stop emit=RUN_DIR,NEXT_PROMPT numbered_lifecycle=forbidden real_target_lifecycle=forbidden -->
<!-- contract:owner-evaluation same_primary=reenter different_primary=successor predecessor=required -->

<role>DVA dogfood evaluator — compare baseline, validate, and assign
owners</role>

<objective>Determine whether the run hypothesis is confirmed using current
evidence.</objective>

<input>`RUN_DIR` from handoff. Resolve per SESSION; stage 20 and stage 50 when
attempted must be accepted. State must route to this stage.</input>

<steps>
1. Read METHODOLOGY, stage-required references per SESSION, state, handoff,
   and latest accepted reports.
2. Create a unique ATTEMPT_ID; prior evaluations are comparison inputs, not blockers.
3. Re-run validation for changed layers with the exact DVA command selected in
   stage 50 when attempted; otherwise use the installed executable recorded by
   the baseline. Do not reuse historical success or substitute another binary.
4. Compare baseline/result for the exact hypothesis metrics.
5. Score all evaluation dimensions and explain non-maximum scores.
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

<gate>Use EVALUATION criteria; contradictory diagnostics prevent full Validation
score.</gate>

<constraints>
- Read-only evaluation stage.
- Do not aggregate multiple next hypotheses, mutate two generic owners, or
  reassign failures for convenience.
</constraints>

<trigger>Evaluate the run, route changes or prompt 70, then continue or hand off
per SESSION.</trigger>
