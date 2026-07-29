<!-- v:2026-07-16 -->

<constants>
SELF = workflows/dva-dogfood/40-improve-prompts.md
ROOT = workflows/dva-dogfood
METHODOLOGY = ./METHODOLOGY.md
PACKAGES_ROOT = the prmpt framework (external)
CONTEXT = ROOT/ref-context.md
ARTIFACTS = ROOT/ref-artifacts.md
EVALUATION = ROOT/ref-evaluation.md
SAFETY = ROOT/ref-safety.md
SESSION = ROOT/ref-session.md
</constants>

[EXECUTE IMMEDIATELY]

<!-- contract:stage id=40 mode_step=stop emit=RUN_DIR,NEXT_PROMPT numbered_lifecycle=forbidden real_target_lifecycle=forbidden -->

<role>DVA fresh-session gate and setup prompt improver</role>

<objective>Validate changed skill triggering when required, then improve local
routing only for prompt-owned findings while retaining devenv-specific
SSoT.</objective>

<input>`RUN_DIR` from handoff. Resolve per SESSION; stage 20 and any selected
stage-30 attempt must be accepted. State must route to this stage.</input>

<steps>
1. Read METHODOLOGY, stage-required references per SESSION, state, handoff,
   latest reports, prmpt guidance, relevant setup files, canonical skill, and active projection.
2. Create a unique ATTEMPT_ID; prior stage-40 attempts never block rerun.
3. If `fresh_session_required` is true, verify this is a new session and record
   the natural-trigger result. On success clear the flag; on failure keep it set,
   write BLOCKED evidence, and stop without prompt changes.
4. Recheck devenv Git HEAD, dirty hash, and protected paths against state. Stop
   on incompatible external changes; otherwise update revision evidence.
5. Map findings to prompt ownership; SKIP with evidence when none match.
6. Verify routing: classification → Makefile/Compose/ports/env → DVA → docs.
7. Invoke the canonical `config` skill where generic DVA configuration reasoning is required.
8. Keep workstation paths, templates, registry, and DVA/Compose boundary in devenv.
9. Remove duplicated generic procedure, validate, write the report, update
   state, and set next prompt 50. Update handoff only at a SESSION boundary.
</steps>

<gate>PASS when any required fresh-session result is successful and routing uses
the installed skill without generic duplication, or prompt changes are validly
SKIPPED.</gate>

<constraints>
- Do not move devenv-specific SSoT into the canonical DVA skills.
- Do not add target-specific commands or reformat unrelated prompts.
</constraints>

<trigger>Validate triggering or improve prompts, then continue or hand off per
SESSION.</trigger>
