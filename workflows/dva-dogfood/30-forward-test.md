<!-- v:2026-08-05 -->

<constants>
ROOT = workflows/dva-dogfood
CONTEXT = ROOT/ref-context.md
ARTIFACTS = ROOT/ref-artifacts.md
EVALUATION = ROOT/ref-evaluation.md
SAFETY = ROOT/ref-safety.md
</constants>

[EXECUTE IMMEDIATELY]

<role>DVA forward-test controller and target applicator</role>

<objective>Replay the frozen requests in independent history-free sessions, and
apply target-owned changes only when the target is this run's owner.</objective>

<input>`RUN_DIR` from handoff. Stage 10, and stage 20 when it was selected, must be
accepted.</input>

<steps>
1. Load the four references whole if this session has not already. Read `state.yaml`,
   `handoff.md`, the latest accepted reports, target guidance, and the active skill
   projection.
2. Create a unique `ATTEMPT_ID`; prior stage-30 attempts never block a rerun.
3. **Fresh-session gate.** If `fresh_session_required` is true, verify this is a new
   session with no prior exposure to the change, record the natural-trigger result,
   and clear the flag on success. On failure keep the flag set, write BLOCKED
   evidence, and stop. A source edit alone is never evidence that a fresh session
   used the changed guidance. If the independent-session mechanism is unavailable,
   keep the flag set and BLOCK with owner `environment`.
4. Recheck target Git HEAD, dirty hash, and protected paths against state. Stop on
   incompatible external changes; otherwise update revision evidence.
5. Select `DVA_COMMAND`: the stage-20 candidate when DVA changed, otherwise the
   installed command recorded at baseline. BLOCK if a changed DVA has no executable
   candidate. Prove the selected path is executable and that its **SHA-256** matches
   state; a matching version and build commit do not identify a binary, because two
   different builds can stamp the same commit. On a mismatch, use
   `candidate_dva_archive` and record why the build path was not used. Record the
   selection without installing globally.
6. Recompute the SHA-256 of `<RUN_DIR>/forward-requests.md` and compare it with
   `evaluation.forward_requests_hash`. On any difference, BLOCK: the requests were
   frozen precisely so they could not be reworded after the baseline was seen. Do
   not re-derive cases here.
7. Act as the forward-test controller. For every ordered frozen request, launch one
   independent history-free child session against a cycle-owned disposable fixture
   or a read-only real target. Give the child only its raw request, the
   fixture/read-only scope, and the safety constraints — never its case label,
   surface, or an expected owner or outcome. Record the controller identity and,
   after each child returns, exactly one `{id, child_session_id, request_hash,
   outcome}`. Every child identity must be non-empty, unique across cases, and
   distinct from the controller's. Mark stage 30 `complete` only after all ordered
   results are recorded.
8. Reconfirm DVA need independently for the root and active subprojects. If none
   needs it, record that with evidence and do not create a config.
9. **Target application** — only when `state.run.owner` is `target_project`:
   - default to preserve, and map every proposed edit to a baseline finding;
   - if the operation requires canonical `config` invocation, require an active
     projection and invoke it explicitly to classify new setup, preserve, migration,
     or rewrite. If that projection is missing, record an `environment` finding and
     BLOCK this operation; never install, sync, or synthesize it;
   - obtain exact command-and-side-effect authority before a rewrite, secret access,
     destructive action, service start, or scope expansion;
   - apply only approved changes and validate after each logical change using
     `DVA_COMMAND`;
   - for a plans migration, reject the change unless the printed `up`, `down`,
     `stop`, and `restart` behavior preserves each entry's runner, services, order,
     and dependencies without mutating process-backed state in preview;
   - defer tool and environment defects instead of masking them in project config.
10. Write the unique attempt report, update state, and set the next prompt to
    `40-evaluate.md`.
</steps>

<gate>PASS when every ordered case has a recorded result from a distinct
history-free session, any required fresh-session check succeeded, and the
hypothesis effect is observed — or no target change was justified — without a
cycle-introduced high regression.</gate>

<constraints>
- Numbered-stage lifecycle execution stays forbidden even with post-cycle authority.
- No case session may invoke `up`, `down`, `stop`, `restart`, or `provision` against
  a real target; children inspect real targets read-only or use disposable fixtures.
- Never touch prohibited archive or legacy modules, expose or overwrite secrets, or
  patch project config merely to silence a DVA CLI defect.
</constraints>

<trigger>Run the forward test, apply approved target changes, then continue or hand
off.</trigger>
