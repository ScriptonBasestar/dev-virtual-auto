<!-- v:2026-07-16 -->

<constants>
SELF = workflows/dva-dogfood/45-improve-dva-tool.md
ROOT = workflows/dva-dogfood
METHODOLOGY = ./METHODOLOGY.md
CONTEXT = ROOT/ref-context.md
ARTIFACTS = ROOT/ref-artifacts.md
EVALUATION = ROOT/ref-evaluation.md
SAFETY = ROOT/ref-safety.md
SESSION = ROOT/ref-session.md
</constants>

[EXECUTE IMMEDIATELY]

<role>DVA source improver — fix a reproduced generator, schema, validator, or
runtime defect at its source</role>

<objective>Improve DVA itself only when the baseline proves a reusable DVA-owned
defect; otherwise record a justified skip.</objective>

<input>`RUN_DIR` from handoff. Resolve per SESSION; stage 20 must be accepted
and state must route to this stage.</input>

<steps>
1. Load METHODOLOGY and this stage's references per SESSION reference reuse —
   once per session, reused while unchanged. Read state, handoff,
   latest reports, and all applicable `DVA_ROOT` guidance.
2. Create a unique ATTEMPT_ID; prior stage-45 attempts never block rerun.
3. Compare installed DVA version/build commit with DVA_ROOT HEAD and dirty hash.
4. Recheck DVA_ROOT dirty paths and protected files. Stop on incompatible
   external changes; never absorb unrelated concurrent work.
5. Map findings to generator prompt/library, schema/config, validator/doctor,
   lifecycle runtime, or documentation ownership. SKIP if no DVA-owned defect is reproduced.
6. Add a focused regression test or deterministic prompt fixture before or with the smallest fix.
7. For lifecycle defects, test runner, services, order, dependencies,
   up/down/stop/restart symmetry, direct-help safety, and non-mutating preview
   behavior independently. Include process-backed PID/log state.
8. Run scoped tests, then the repository build/test gate with bounded output.
9. Build and execute the fixed local artifact. Record its absolute path in
   `candidate_dva_executable` and its commit in `candidate_dva_build_commit`.
   Keep installed/source/candidate provenance distinct.
10. Install or replace a global DVA binary only with recorded user authority.
11. Write the unique attempt report; update revision evidence and state, set
    next prompt 50, and update handoff only at a SESSION boundary.
</steps>

<gate>PASS when a reproduced DVA defect has a focused source fix, regression
test, and executable candidate, or SKIPPED when evidence assigns the finding
elsewhere.</gate>

<constraints>
- Do not change target config to compensate for a DVA source defect.
- Do not edit generated artifacts when their source generator exists.
- Do not run lifecycle commands against the target as a preview test.
</constraints>

<trigger>Improve DVA source, record the candidate, then continue or hand off per
SESSION.</trigger>
