<!-- v:2026-07-16 -->

<constants>
SELF = workflows/dva-dogfood/30-improve-skill.md
ROOT = workflows/dva-dogfood
METHODOLOGY = ./METHODOLOGY.md
CONTEXT = ROOT/ref-context.md
ARTIFACTS = ROOT/ref-artifacts.md
EVALUATION = ROOT/ref-evaluation.md
SAFETY = ROOT/ref-safety.md
SESSION = ROOT/ref-session.md
</constants>

[EXECUTE IMMEDIATELY]

<role>DVA skill improver — make one reusable evidence-backed change</role>

<objective>Improve one canonical DVA skill only where the baseline proves a
reusable gap.</objective>

<input>`RUN_DIR` from handoff. Resolve per SESSION; latest accepted reports
through stage 20 must PASS.</input>

<steps>
1. Load METHODOLOGY and this stage's references per SESSION reference reuse —
   once per session, reused while unchanged. Read state, handoff,
   latest reports, full skill, and official skill-creator instructions.
2. Create a unique ATTEMPT_ID; prior stage-30 attempts never block rerun.
3. Recheck DVA_ROOT Git HEAD, the selected canonical skill hash, and protected
   paths against state. Stop on incompatible external changes; otherwise update
   revision evidence.
4. Map baseline findings to skill ownership; SKIP with evidence when none match.
5. Apply the smallest change capable of altering this cycle's metric.
6. Keep workflow rules concise; move detail to references and exclude target-specific paths.
7. Run official skill validation and test changed scripts.
8. Run `make generate`; verify every target form declared by
   `skills/_targets.yaml` and compare each projection with the canonical source
   using the relation its shape supports, per ARTIFACTS Evidence rules.
9. If installed metadata/body changed, set `fresh_session_required: true` and
   stage 40 pending, then set next prompt 40. Write the attempt report and update
   state before the required session boundary. Otherwise mark stage 40
   `not_applicable` and set next prompt 50.
</steps>

<gate>PASS when one generic change is validated/synced, or SKIPPED with
evidence.</gate>

<constraints>
- One primary skill hypothesis per run.
- Edit only the canonical skill; never hand-edit generated or installed copies.
</constraints>

<trigger>Improve the skill, then continue or hand off per SESSION.</trigger>
