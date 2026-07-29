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
1. Load METHODOLOGY and this stage's references per SESSION reference reuse —
   once per session, reused while unchanged. Read state, handoff,
   latest reports, target guidance, improved setup entry, canonical skill, and active projection.
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
6. Verify the frozen evaluation contract before changing or reusing any result:
   the surface-manifest version in EVALUATION must still equal `state.version`,
   and the ordered `case_ids` and `case_manifest_hash` recorded at stage 20 must
   still match the exact bytes of `<RUN_DIR>/forward-requests.md`. Do not
   re-derive cases here; a surface added or removed upstream after the freeze is
   a mismatch, not a correction. On any mismatch, record
   `evaluation_contract_mismatch`, preserve existing evidence, and require a
   successor; do not continue this run.
7. Act as the forward-test controller. For every ordered frozen request, launch
   one independent history-free child session against a cycle-owned disposable
   fixture or a read-only real target. Give that child only the raw request,
   fixture/read-only scope, and safety constraints — never its case label or an
   expected owner/outcome. Record the controller identity and, after each child
   returns, exactly one `{id, child_session_id, request_hash, outcome}` record.
   Every child identity must be non-empty, unique across cases, and distinct
   from the controller identity. Mark stage 50 `complete` only after all
   ordered results are recorded; its only other allowed statuses are `pending`,
   `blocked`, and `not_applicable`. Never start a real target lifecycle from a
   case session.
8. If the selected operation requires canonical `config` invocation, require an
   active projection and invoke it explicitly to classify new setup, preserve,
   migration, or rewrite. If that projection is missing, record an
   `environment` finding and BLOCK this operation; do not install, sync, or
   synthesize it. If invocation is not required, preserve the finding and
   continue without blocking.
9. Default to preserve; map every proposed edit to a baseline finding and owner.
10. Obtain exact command-and-side-effect authority before rewrite, secrets,
   destructive actions, service start, or scope expansion. Numbered-stage
   lifecycle remains forbidden even with post-cycle authority.
11. Apply only approved target changes and validate after each logical change,
   using `DVA_COMMAND` for every DVA operation.
12. For a plans migration, reject the change unless printed `up`, `down`,
   `stop`, and `restart` behavior preserves each entry's runner, services,
   order, and dependencies without mutating process-backed state in preview.
13. Defer tool/environment defects instead of masking them in project config.
14. Write the unique attempt report, update state, and set next prompt 60.
</steps>

<gate>PASS when the hypothesis effect is observed, or no target change is
justified, without a cycle-introduced high regression.</gate>

<constraints>
- Never touch prohibited archive/legacy modules or expose/overwrite secrets.
- Never patch project config merely to silence a DVA CLI defect.
- Forward-test children may inspect real targets read-only or use disposable
  fixtures only; no child may invoke `up`, `down`, `stop`, `restart`, or
  `provision` against a real target.
</constraints>

<trigger>Apply DVA changes with the selected command, then continue or hand off
per SESSION.</trigger>
