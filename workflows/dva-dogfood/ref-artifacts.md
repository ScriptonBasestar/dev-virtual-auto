# DVA Dogfood Artifact Contract

Domain deltas only; invariants live in
[METHODOLOGY.md](./METHODOLOGY.md).

## Layout

```text
<RUN_DIR>/
├── state.yaml
├── handoff.md
└── stages/
    ├── 10-skill-audit/<ATTEMPT_ID>/report.md
    ├── 20-baseline/<ATTEMPT_ID>/report.md
    ├── 30-skill-change/<ATTEMPT_ID>/report.md
    ├── 40-prompt-change/<ATTEMPT_ID>/report.md
    ├── 45-dva-tool-change/<ATTEMPT_ID>/report.md
    ├── 50-application/<ATTEMPT_ID>/report.md
    ├── 60-evaluation/<ATTEMPT_ID>/report.md
    └── 70-feedback/<ATTEMPT_ID>/report.md
```

## State schema

```yaml
run:
  id: "YYYYMMDD-HHMMSS-a1b2c3"
  target_project: "/absolute/path"
  hypothesis: "one observable claim"
  run_dir: "/absolute/path/to/tmp/dogfood-dva/run-id"
  created_at: "RFC3339 timestamp"
  status: active # active | complete | blocked
  fresh_session_required: false
  primary_owner: null # skill | prompt | dva_tool | target_project | environment

revisions:
  target_head: null
  target_dirty_hash: null
  plugin_head: null
  plugin_dirty_hash: null
  devenv_head: null
  devenv_dirty_hash: null
  dva_head: null
  dva_dirty_hash: null
  prompt_bundle_hash: null
  installed_skill_hash: null

sources:
  dogfood_root: "workflows/dva-dogfood"
  packages_root: "the prmpt framework (external)"
  plugin_root: "the claude-ce-plugin repo" # generic DVA skill; DVA config skill is canonical here at skills/config/
  dva_root: "." # this repo root
  skill_source: null
  skill_installed: null
  dva_executable: null
  dva_version: null
  dva_build_commit: null
  candidate_dva_executable: null
  candidate_dva_build_commit: null

stages:
  "00":
    status: complete # pending | complete | blocked | not_applicable
    latest_attempt: null
    latest_accepted_report: null
    attempts: []
  "10":
    status: pending
    latest_attempt: null
    latest_accepted_report: null
    attempts: []
  # Repeat the same structure for 20, 30, 40, 45, 50, 60, and 70.

protected_paths: []
findings: []
next_prompt: "10-audit-skill.md"
```

`latest_attempt` tracks the newest terminal report; `latest_accepted_report`
tracks the newest PASS or accepted SKIPPED report. Stages excluded by
stage-20 owner routing use `not_applicable` without creating an attempt
report. `next_prompt` always names the next invoked stage, not the next
numeric stage.

When stage 60 routes back to an owner, mark that stage pending and clear
downstream `latest_accepted_report` pointers affected by the new change; retain
all attempt history. Reactivating stage 45 also clears candidate DVA provenance
until a new executable passes its gate.

`dva_executable`, `dva_version`, and `dva_build_commit` describe the installed
DVA command. Stage 45 records a tested local build in `candidate_dva_executable`
and `candidate_dva_build_commit`; it never overwrites installed provenance.
`fresh_session_required` is cleared only after a successful fresh-session
trigger check. A failed check leaves it set and blocks further mutation.

## Session handoff

Keep `handoff.md` concise per METHODOLOGY, plus these dva additions:

- surface the latest attempt for every attempted stage, not only accepted
  reports;
- record the next prompt with its exact invocation, not only the filename;
- installed, candidate, and selected DVA executable provenance;
- whether the next session should resume or create a new attempt.

## Stage report fields

Report fields follow METHODOLOGY's order, with these dva conventions:
`Scope` lists the repositories and files inspected or edited; `Changes`
lists the files changed, or `None` for read-only stages. `Findings` use
stable IDs such as `SKILL-001`, `PROMPT-001`, `DVA-001`, `PROJECT-001`.

## Evidence rules

- Record installed DVA version/commit separately from `dva_root` HEAD and dirty
  hash; equality must be proven rather than inferred.
- When evaluating prompt behavior, record the model/runtime, prompt bundle hash,
  seed revision, and exact reproduction command without copying secret values.
- Redact secrets instead of copying environment files (`.env`) into evidence.
