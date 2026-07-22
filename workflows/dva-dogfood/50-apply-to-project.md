<!-- v:2026-07-16 -->

<constants>
SELF = workflows/dva-dogfood/50-apply-to-project.md
ROOT = workflows/dva-dogfood
METHODOLOGY = ./METHODOLOGY.md
ARTIFACTS = ROOT/ref-artifacts.md
EVALUATION = ROOT/ref-evaluation.md
SAFETY = ROOT/ref-safety.md
SESSION = ROOT/ref-session.md
</constants>

[EXECUTE IMMEDIATELY]

<role>DVA project applicator and forward tester</role>

<objective>Exercise the selected skill, prompt, or DVA candidate on the fixed
target. Apply only target-owned changes justified by the baseline.</objective>

<input>`RUN_DIR` from handoff. Resolve per SESSION; stage 20 and the selected
mutation stage, when any, must be accepted. State must route to this
stage.</input>

<steps>
1. Read METHODOLOGY, stage-required references per SESSION, state, handoff,
   latest reports, target guidance, improved setup entry, and installed skill.
2. Create a unique ATTEMPT_ID; prior stage-50 attempts never block rerun.
3. Recheck target Git HEAD, dirty hash, and protected paths against state; stop
   on incompatible external changes and update revision evidence otherwise.
4. Reconfirm DVA need independently for the root and active subprojects. If none
   need it, write an evidence-backed SKIPPED report, set next prompt 60, and do
   not create config or execute the remaining application steps.
5. Select `DVA_COMMAND`: use the stage-45 candidate when DVA changed, otherwise
   use the recorded installed command. BLOCK if a changed DVA has no executable
   candidate. Prove the selected path is executable and its version/commit
   matches state. Record the selection and provenance without global install.
6. Invoke `tool-dva-config` explicitly and classify new setup, preserve,
   migration, or rewrite.
7. Default to preserve; map every proposed edit to a baseline finding and owner.
8. Obtain authority before rewrite, secrets, destructive actions, service start,
   or scope expansion.
9. Apply only approved target changes and validate after each logical change,
   using `DVA_COMMAND` for every DVA operation.
10. For a plans migration, reject the change unless printed `up`, `down`,
   `stop`, and `restart` behavior preserves each entry's runner, services,
   order, and dependencies without mutating process-backed state in preview.
11. Defer tool/environment defects instead of masking them in project config.
12. Write the unique attempt report, update state, and set next prompt 60.
</steps>

<gate>PASS when the hypothesis effect is observed, or no target change is
justified, without a cycle-introduced high regression.</gate>

<constraints>
- Never touch prohibited archive/legacy modules or expose/overwrite secrets.
- Never patch project config merely to silence a DVA CLI defect.
</constraints>

<trigger>Apply DVA changes with the selected command, then continue or hand off
per SESSION.</trigger>
