<!-- v:2026-08-05 -->

<constants>
ROOT = workflows/dva-dogfood
CONTEXT = ROOT/ref-context.md
ARTIFACTS = ROOT/ref-artifacts.md
EVALUATION = ROOT/ref-evaluation.md
SAFETY = ROOT/ref-safety.md
</constants>

[EXECUTE IMMEDIATELY]

<role>DVA improver — make one reusable, evidence-backed change to the selected owner</role>

<objective>Apply the smallest change capable of moving this cycle's metric, in the
one owner stage 10 selected, or record a justified SKIP.</objective>

<input>`RUN_DIR` from handoff. Stage 10 must be accepted and `state.run.owner` must
be `skill`, `prompt`, or `dva_tool`.</input>

<steps>
1. Load the four references whole if this session has not already. Read `state.yaml`,
   `handoff.md`, the latest accepted reports, and all applicable `DVA_ROOT` guidance.
2. Create a unique `ATTEMPT_ID`; prior stage-20 attempts never block a rerun.
3. Recheck `DVA_ROOT` HEAD, dirty hash, and protected paths against state. Stop on
   incompatible external changes; never absorb unrelated concurrent work.
4. Map the baseline findings to the selected owner. **SKIP with evidence when no
   finding under this owner is reproduced** — a SKIP is a valid, recordable outcome,
   not a failure.
5. Run the owner block below.
6. Apply the smallest change capable of altering this cycle's metric. Add a focused
   regression test or deterministic fixture before or with the fix.
7. Write the unique attempt report, update revision evidence and state, and set the
   next prompt to `30-forward-test.md`.
</steps>

<owner-skill>
Only when `owner: skill`.

1. Read the selected canonical skill completely, plus the official skill-creator
   instructions. This is the stage that reads skill bodies deeply; earlier stages
   deliberately did not.
2. Edit only the canonical skill under `SKILLS_ROOT`; never hand-edit a generated or
   installed copy. Keep rules concise, move detail to references, and exclude
   target-specific paths.
3. Run official skill validation and test any changed scripts.
4. Run `make generate` from `DVA_ROOT`, then verify every target form declared by
   `skills/_targets.yaml` against the canonical source using the relation its shape
   supports, per ARTIFACTS Evidence rules.
5. If installed metadata or body changed, set `fresh_session_required: true`. Stage
   30 will not launch case sessions until it clears.
6. One primary skill hypothesis per run.
</owner-skill>

<owner-prompt>
Only when `owner: prompt`.

1. Verify stage routing end to end: owner selection → mutation → forward test →
   evaluation, and that each stage names the references it actually loads.
2. Keep reusable DVA procedure in the canonical skills and only run orchestration
   here. Remove duplicated generic procedure rather than restating it.
3. Validate the edit:
   - executable prompts stay in English and concise;
   - shared rules live in `ref-*`, not duplicated in every stage;
   - a stage's evidence directory name still equals its prompt basename;
   - all relative links and referenced filenames resolve;
   - run the repository's Markdown checks scoped as narrowly as supported
     (`make doc-check` enforces size and link limits under `workflows/`).
4. Do not add target-specific commands or reformat unrelated prompts.
</owner-prompt>

<owner-dva-tool>
Only when `owner: dva_tool`.

1. Compare the installed DVA version and build commit with `DVA_ROOT` HEAD and
   dirty hash before changing anything.
2. Map the finding to generator prompt/library, schema/config, validator/doctor,
   lifecycle runtime, or documentation ownership. Do not edit a generated artifact
   when its source generator exists.
3. For a lifecycle defect, test runner, services, order, dependencies,
   up/down/stop/restart symmetry, direct-help safety, and non-mutating preview
   behavior independently, including process-backed PID and log state.
4. Run scoped tests, then the repository build/test gate with bounded output.
5. Build and execute the fixed local artifact, then **copy it into this attempt's
   `artifacts/` directory** — the build path is not durable and a concurrent build
   destroys it silently. Record the build path in `candidate_dva_executable`, the
   copy in `candidate_dva_archive`, the commit in `candidate_dva_build_commit`, and
   the **full** SHA-256 of both the archived copy and the installed executable in
   `candidate_dva_sha256` and `dva_sha256`. Never truncate a digest. Keep installed,
   source, and candidate provenance distinct — never overwrite installed provenance.
6. Install or replace a global DVA binary only with recorded user authority.
7. Do not change target config to compensate for a DVA source defect, and do not
   run lifecycle commands against the target as a preview test.
</owner-dva-tool>

<gate>PASS when one owner-scoped change is validated — with its projection synced
(skill), its links and limits checked (prompt), or its regression test and
executable candidate recorded (dva_tool) — or SKIPPED with evidence assigning the
finding elsewhere.</gate>

<constraints>
- Mutate only `state.run.owner`. A finding under a different owner goes to
  `backlog`, never into this run.
- Record the exact pre-edit patch and its inverse before editing any path that is
  not clean; leave unrelated dirty paths untouched.
</constraints>

<trigger>Improve the selected owner, then continue or hand off.</trigger>
