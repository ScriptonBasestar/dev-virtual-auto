<!-- v:2026-08-05 -->

<constants>
ROOT = workflows/dva-dogfood
CONTEXT = ROOT/ref-context.md
ARTIFACTS = ROOT/ref-artifacts.md
EVALUATION = ROOT/ref-evaluation.md
SAFETY = ROOT/ref-safety.md
</constants>

[EXECUTE IMMEDIATELY]

<role>DVA baseline collector — capture current behavior and select one owner</role>

<objective>Create the before-state for the hypothesis, separate config, project,
environment, and CLI behavior, and freeze this run's evaluation cases.</objective>

<input>`RUN_DIR` from handoff. The latest accepted stage-00 report must PASS.</input>

<steps>
1. Load the four references whole if this session has not already. Read `state.yaml`,
   `handoff.md`, the stage-00 report, and target module guidance.
2. Create a unique `ATTEMPT_ID`. Older baseline attempts are comparison inputs, not
   blockers.
3. Inventory canonical `dva.yml`, legacy `dva.yaml`, other referenced DVA files,
   Compose files, Makefile, env templates, port definitions, and AI guidance files.
4. Confirm the installed DVA executable, version, and build commit still match what
   stage 00 recorded. Capture lifecycle `--help` only after SAFETY's non-executing
   proof.
5. Run the SAFETY validation ladder read-only. When behavior is contradictory,
   reproduce it first, then inspect the relevant CLI, schema, or doctor
   implementation in `DVA_ROOT` and record exact source references.
6. Validate Compose configuration without starting services. Remember that
   `docker compose config --quiet` needs no daemon: passing it proves the file set
   merges, never that a daemon is reachable.
7. If plans exist or a migration is proposed, compare the resolved runner,
   services, order, and dependencies with the actual printed `up`, `down`, `stop`,
   and `restart` commands. Prove that a process-backed preview preserves PID and
   log state; schema acceptance alone is not runtime proof.
8. Compare root and active-subproject responsibilities, and decide DVA need for
   each independently. Exclude archived and legacy modules; never force-create DVA.
9. Capture dirty paths without reading protected secret contents. Recompute
   `prompt_bundle_hash` only with the ARTIFACTS derivation; if it differs from stage 00,
   list which tracked files under `workflows/dva-dogfood/` changed and record mid-run
   prompt drift — do not invent a second hash command.
10. Classify every warning and contradiction. Define measurable before/after metrics
    for this run's hypothesis.
11. Derive and freeze the evaluation cases per EVALUATION:
    - walk the surfaces in manifest order and instantiate each against **verified**
      target state;
    - record every surface with no instance in `not_applicable_surfaces` with its
      absence evidence;
    - never invent a case to fill a surface — a small honest case set is a valid
      outcome, and only a target lacking both `config_schema` and `no_change`
      blocks with `target_out_of_scope`;
    - write `<RUN_DIR>/forward-requests.md` with exactly one non-empty
      `raw_request` per derived case, in order, containing no expected owner or
      outcome, and record its full-file SHA-256.
12. Assign exactly one `owner` per EVALUATION's ownership table, then set the next
    prompt: `skill`, `prompt`, or `dva_tool` → `20-improve.md`; `target_project`,
    `environment`, or `no_change` → `30-forward-test.md`, marking stage 20
    `not_applicable` with no attempt report. Record secondary or different-owner
    findings in `backlog` for a future run.
13. Write the unique attempt report and update state. Update `handoff.md` at the
    session boundary.
</steps>

<gate>PASS when another session can reproduce the current baseline, the frozen
cases, and the owner selection from the report alone.</gate>

<constraints>
- Read-only. Do not use fix, rewrite, reset, clean, production, or service-start
  commands.
- Historical success cannot satisfy current validation.
- A forward request is evidence input, not a hidden answer key: it carries no owner
  and no anticipated outcome.
</constraints>

<trigger>Capture the baseline, select the owner, then continue or hand off.</trigger>
