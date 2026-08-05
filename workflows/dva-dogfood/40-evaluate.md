<!-- v:2026-08-05 -->

<constants>
ROOT = workflows/dva-dogfood
CONTEXT = ROOT/ref-context.md
ARTIFACTS = ROOT/ref-artifacts.md
EVALUATION = ROOT/ref-evaluation.md
SAFETY = ROOT/ref-safety.md
</constants>

[EXECUTE IMMEDIATELY]

<role>DVA dogfood evaluator — score the run, route every finding, and close</role>

<objective>Determine whether the hypothesis holds using current evidence, assign
every finding to its SSoT, and leave one measurable next hypothesis.</objective>

<input>`RUN_DIR` from handoff. Stage 10, and stages 20/30 where attempted, must be
accepted.</input>

<steps>
1. Load the four references whole if this session has not already. Read `state.yaml`,
   `handoff.md`, and the latest accepted reports.
2. Create a unique `ATTEMPT_ID`; prior evaluations are comparison inputs, not
   blockers.
3. Re-run validation for the changed layers with the exact `DVA_COMMAND` stage 30
   selected; otherwise use the installed executable the baseline recorded. Verify it
   by SHA-256 before use and fall back to `candidate_dva_archive`. Do not reuse
   historical success or substitute another binary. When no recorded artifact
   survives, record the substitution as step-3 non-compliance, state what is no
   longer provable, and re-derive the before side from committed source — tests
   against source outlive an artifact, transcripts of its output do not.
4. Compare baseline and result for the exact hypothesis metrics.
5. Score every applicable dimension in `<scoring>` and explain each non-maximum
   score.
6. Classify the run `CONFIRMED`, `PARTIAL`, `REJECTED`, or `INCONCLUSIVE` per
   EVALUATION.
7. Assign every unresolved finding one owner and one severity per EVALUATION, and
   check regressions and protected paths.
8. **Route, do not fix.** If a cycle-owned regression stays within this run's owner,
   keep the run active, mark stage 20 pending, invalidate the downstream accepted
   pointers the change affects while retaining their reports, clear candidate DVA
   provenance, and route back to `20-improve.md`. Do not close until the changed
   state is evaluated again.
9. Otherwise close the run:
   - group unresolved findings by owner; route `dva_tool` findings to `DVA_ROOT`,
     never to a target workaround;
   - move findings belonging to another owner into `backlog` with their evidence;
   - for a generic skill or prompt change validated against only one target, mark it
     **provisional** and prefer a structurally different target — one that
     instantiates a different surface set — as the next hypothesis;
   - select exactly one measurable next-run hypothesis and set the final run status;
   - emit the next `00-start.md` invocation, with `predecessor_run_id` set when the
     next run continues from this one.
10. Write the unique attempt report and update state and handoff.
</steps>

<gate>
Cycle PASS requires all of:

- comparable before/after evidence;
- every derived case has a recorded outcome, and every finding exactly one owner;
- no unresolved critical or high regression introduced by this cycle;
- skill installation and fresh-session behavior checked when a skill changed;
- every Compose service, native process, check, and provision action has exactly one
  lifecycle owner;
- Safety, Validation, and Ownership all scored 2.

Contradictory diagnostics prevent a full Validation score. Surfaces recorded
`not_applicable` at stage 10 do not count against coverage; an invented case counts
as a Traceability zero. Final closure additionally requires one singular, measurable
next hypothesis.
</gate>

<constraints>
- Read-only with respect to every evaluated source. Route corrections back to stage
  20 instead of editing after evaluation.
- Do not aggregate multiple next hypotheses, mutate a second owner, or reassign a
  failure for convenience.
- Keep raw artifacts only in `RUN_DIR` and durable knowledge only in its SSoT.
</constraints>

<scoring>

Score each applicable dimension 0–2. Mark unrelated dimensions `N/A`. Report earned
over applicable points, for example `12/14`; never penalize `N/A`. The score is
diagnostic — the cycle gate above is what passes or fails the run.

<!-- markdownlint-disable MD013 -->

| Dimension     | 0                                         | 1                                     | 2                                                         |
| ------------- | ----------------------------------------- | ------------------------------------- | --------------------------------------------------------- |
| Triggering    | Skill absent or not triggered             | Explicit invocation only              | Correct explicit and natural triggering                   |
| Correctness   | Wrong or unsafe result                    | Mostly correct with material warnings | Correct and evidence-backed                               |
| Reuse         | Target-specific logic embedded            | Some reusable separation              | Clean generic/local/project ownership                     |
| Efficiency    | Repeated broad scans or excessive context | Minor duplication                     | Bounded scans and progressive disclosure                  |
| Safety        | Secret, user, or production risk          | Protected with gaps                   | Protected paths and approval gates enforced               |
| Validation    | No meaningful checks                      | Partial checks                        | Layered current-state checks                              |
| Runtime truth | Reported state ignores what DVA controls  | Probe-only; port ownership unverified | Status and health reflect the tracked process owning its port |
| Ownership     | Same service or check has multiple owners | Overlap found but incompletely routed | One lifecycle owner and one SSoT per behavior             |
| Traceability  | Cannot explain the result                 | Partial evidence                      | Baseline, diff, findings, owner, and result linked        |

<!-- markdownlint-enable MD013 -->

</scoring>

<trigger>Evaluate the run, then route back to stage 20 or close it.</trigger>
