<!-- v:2026-07-16 -->

<constants>
SELF = workflows/dva-dogfood/20-capture-baseline.md
ROOT = workflows/dva-dogfood
METHODOLOGY = ./METHODOLOGY.md
CONTEXT = ROOT/ref-context.md
ARTIFACTS = ROOT/ref-artifacts.md
EVALUATION = ROOT/ref-evaluation.md
SAFETY = ROOT/ref-safety.md
SESSION = ROOT/ref-session.md
</constants>

[EXECUTE IMMEDIATELY]

<role>DVA baseline collector — capture current behavior without fixes</role>

<objective>Create the before-state for the hypothesis and separate config,
project, environment, and CLI behavior.</objective>

<input>`RUN_DIR` from handoff. Resolve per SESSION; latest accepted stage-10
report must PASS.</input>

<steps>
1. Read METHODOLOGY, stage-required references per SESSION, state, handoff,
   latest stage-10 report, selected canonical skill, and target/module guidance.
2. Create a unique ATTEMPT_ID; older baseline attempts are comparison inputs, not blockers.
3. Inventory canonical `dva.yml`, legacy `dva.yaml`, other referenced DVA
   files, Compose, Makefile, env template, port, and AI guidance files.
4. Capture the installed DVA executable path, version, and build commit; compare
   them with `DVA_ROOT` HEAD and dirty hash without assuming they match. Capture
   lifecycle help only after SAFETY's non-executing help proof.
5. Run the read-only validation ladder. When behavior is contradictory, first
   reproduce it, then inspect the relevant CLI/schema/doctor implementation in
   `DVA_ROOT` and record exact source references.
6. Validate Compose configuration without starting services.
7. If plans exist or migration is proposed, compare resolved runner, services,
   order, and dependencies with the actual printed `up`, `down`, `stop`, and
   `restart` commands. Prove process-backed preview preserves PID/log state;
   schema acceptance alone is not runtime proof.
8. Compare root/active subproject responsibilities and decide DVA need for each
   independently; exclude archived/legacy modules and never force-create DVA.
9. Capture dirty paths without reading protected secret contents.
10. Classify warnings and contradictions; define measurable before/after metrics.
11. Before selecting an owner, freeze the evaluation contract from
    EVALUATION: copy its exact ordered IDs, version, and deterministic manifest
    SHA-256 into state. Create `<RUN_DIR>/forward-requests.md` as its strict
    YAML request document with exactly one non-empty `raw_request` per case in
    the same order, then record its full-file SHA-256. Requests must contain no
    expected-owner or expected-outcome field. Do not reuse or rewrite a frozen
    file; any byte/order/manifest mismatch blocks this run with
    `evaluation_contract_mismatch` and requires a successor.
12. Assign exactly one generic `primary_owner` and one DVA `owner_route`.
    Map `skill` and `dva_tool` to `plugin`, `prompt` (including workflow-contract
    changes) to `local_setup`, `target_project` to `target`, and preserve
    `environment` and `no_change` as both route and generic owner. Select prompt
    30, 40, 45, 50, 60, or 50 for `skill`, `prompt`, `dva_tool`,
    `target_project`, `environment`, or `no_change` respectively. Mark
    unselected owner stages 30, 40, and 45 `not_applicable` without attempt
    reports. For skill ownership, keep stage 40 conditional until stage 30
    determines whether a fresh-session gate is required. This run may mutate
    only its selected generic primary owner; secondary or different-owner
    findings are backlog for a successor run.
13. Write the unique attempt report and update state. Update handoff only at a
    SESSION boundary.
</steps>

<gate>PASS when another session can reproduce the current baseline and owner
route from the report.</gate>

<constraints>
- Do not use fix, rewrite, reset, clean, production, or service-start commands.
- Historical success cannot satisfy current validation.
- A forward request is evidence input, not a hidden answer key: do not put an
  owner or anticipated outcome in it.
</constraints>

<trigger>Capture baseline, select the owner stage, then continue or hand off per
SESSION.</trigger>
