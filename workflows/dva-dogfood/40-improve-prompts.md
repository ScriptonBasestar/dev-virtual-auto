<!-- v:2026-07-16 -->

<constants>
SELF = workflows/dva-dogfood/40-improve-prompts.md
ROOT = workflows/dva-dogfood
METHODOLOGY = ./METHODOLOGY.md
CONTEXT = ROOT/ref-context.md
ARTIFACTS = ROOT/ref-artifacts.md
EVALUATION = ROOT/ref-evaluation.md
SAFETY = ROOT/ref-safety.md
SESSION = ROOT/ref-session.md
</constants>

[EXECUTE IMMEDIATELY]

<role>DVA fresh-session gate and workflow prompt improver</role>

<objective>Validate changed skill triggering when required, then improve this
workflow's own stage prompts and references only for prompt-owned
findings.</objective>

<input>`RUN_DIR` from handoff. Resolve per SESSION; stage 20 and any selected
stage-30 attempt must be accepted. State must route to this stage.</input>

<steps>
1. Load METHODOLOGY and this stage's references per SESSION reference reuse —
   once per session, reused while unchanged. Read state, handoff,
   latest reports, the stage prompts and references this run may change,
   canonical skill, and active projection.
2. Create a unique ATTEMPT_ID; prior stage-40 attempts never block rerun.
3. If `fresh_session_required` is true, verify this is a new session and record
   the natural-trigger result. On success clear the flag; on failure keep it set,
   write BLOCKED evidence, and stop without prompt changes.
4. Recheck DVA_ROOT Git HEAD, dirty hash, and protected paths against state. Stop
   on incompatible external changes; otherwise update revision evidence.
5. Map findings to prompt ownership; SKIP with evidence when none match.
6. Verify stage routing: owner selection → mutation stage → forward test →
   evaluation, and that each stage names the references it actually needs.
7. Invoke the canonical `config` skill where generic DVA configuration reasoning is required.
8. Keep reusable DVA procedure in the canonical skills and keep only run
   orchestration here.
9. Remove duplicated generic procedure, validate against `<prompt-validation>`,
   write the report, update state, and set next prompt 50. Update handoff only at
   a SESSION boundary.
</steps>

<prompt-validation>
- Keep executable prompts in English and concise.
- Keep shared rules in `ref-*`, not duplicated in every prompt.
- Keep a rule in the reference the stage that uses it actually loads; a section
  every stage reads and one stage consumes belongs in that stage.
- Verify all relative links and referenced filenames.
- Run the repository's Markdown formatter/linter scoped as narrowly as supported.
</prompt-validation>

<gate>PASS when any required fresh-session result is successful and routing uses
the installed skill without generic duplication, or prompt changes are validly
SKIPPED.</gate>

<constraints>
- Do not move run-orchestration-only routing into the canonical DVA skills.
- Do not add target-specific commands or reformat unrelated prompts.
</constraints>

<trigger>Validate triggering or improve prompts, then continue or hand off per
SESSION.</trigger>
