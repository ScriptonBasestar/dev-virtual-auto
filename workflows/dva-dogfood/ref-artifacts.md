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
  mode: continuous # continuous | step
  run_dir: "/absolute/path/to/tmp/dogfood-dva/run-id"
  created_at: "RFC3339 timestamp"
  status: active # active | complete | blocked
  predecessor_run_id: null
  fresh_session_required: false
  primary_owner: null # plugin | local_setup | target | environment | no_change
  owner_route: null # skill | prompt | dva_tool | target_project | environment | no_change

evaluation:
  version: "dva-routing-v2"
  case_ids: [/* ordered cases derived from the surfaces for THIS target */]
  case_manifest_hash: "<sha256>"
  not_applicable_surfaces: [] # {surface, evidence} per surface with no instance
  forward_requests_hash: null
  forward_test:
    controller_session_id: null
    cases: [] # completed stage 50: one {id, child_session_id, request_hash, outcome} per ordered case

revisions:
  target_head: null
  target_dirty_hash: null # sha1 of `git status --porcelain` run from the repo root
  dva_head: null
  dva_dirty_hash: null # same derivation; every stage must use it or the values cannot be compared
  skill_source_hash: null
  prompt_bundle_hash: null
  installed_skill_hash: null

sources:
  dogfood_root: "workflows/dva-dogfood"
  skills_root: "skills"
  dva_root: "." # this repo root
  skill_source: null
  skill_installed: null
  dva_executable: null
  dva_version: null
  dva_build_commit: null
  candidate_dva_executable: null
  candidate_dva_build_commit: null
  config_projection: null # active | missing

operations:
  config_invocation_required: false

authority:
  scope: numbered_stage # numbered_stage | post_cycle_qa
  approved: false
  commands: [] # exact entries: {command: "...", side_effect: "declared_effect"}

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

`primary_owner` is the domain-neutral mutation boundary from METHODOLOGY.
`owner_route` selects the DVA-specific stage within that boundary:

| DVA owner route       | Generic primary owner |
| --------------------- | --------------------- |
| `skill` / `dva_tool`  | `plugin`              |
| `prompt`              | `local_setup`         |
| `target_project`      | `target`              |
| `environment`         | `environment`         |
| `no_change`           | `no_change`           |

Workflow-contract changes are local setup changes and therefore use
`owner_route: prompt` with `primary_owner: local_setup`. A run may mutate only
its one generic `primary_owner`; a finding that requires a different generic
owner is backlog for a successor run, whose `predecessor_run_id` names this run.

`latest_attempt` tracks the newest terminal report; `latest_accepted_report`
tracks the newest PASS or accepted SKIPPED report. Stages excluded by
stage-20 `owner_route` use `not_applicable` without creating an attempt
report. `next_prompt` always names the next invoked stage, not the next
numeric stage.

When stage 60 routes back within the run's existing generic `primary_owner`,
mark the selected DVA owner-route stage pending and clear downstream
`latest_accepted_report` pointers affected by the new change; retain all
attempt history. A route under a different generic owner is backlog for a
successor run, not a mutation in this run. Reactivating stage 45 also clears
candidate DVA provenance until a new executable passes its gate.

`dva_executable`, `dva_version`, and `dva_build_commit` describe the installed
DVA command. Stage 45 records a tested local build in `candidate_dva_executable`
and `candidate_dva_build_commit`; it never overwrites installed provenance.
`fresh_session_required` is cleared only after a successful fresh-session
trigger check. A failed check leaves it set and blocks further mutation.

`sources.config_projection: missing` is an environment finding; only an
operation with
`operations.config_invocation_required: true` blocks on it, and it never
permits installation or projection synthesis. `authority` accepts runtime work
only at `scope: post_cycle_qa` with approved exact `{command, side_effect}`
entries matching every invoked lifecycle command. The side effect must be the
declared effect for that command; generic approval is invalid.

The `evaluation` block is the state projection of the contract defined in
`ref-evaluation.md`: `case_manifest_hash` holds the surface manifest's hash and
is target-independent, `case_ids` the ordered cases stage 20 derived for this
target, `not_applicable_surfaces` each surface with no instance plus its absence
evidence, and `forward_test` the completed controller and child records. The
rules governing those values — derivation, freezing, mismatch, forward-test
completeness, and the `stages.50.status` enum — live in `ref-evaluation.md` and
are deliberately not restated here.

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
- Record the canonical skill hash separately from each projection, and prove the
  relation the projection's declared shape actually supports. A copy or symlink
  target must be proven *identical* (same hash, or same inode for a symlink). A
  conversion target — anything `skills/_targets.yaml` marks `generated: true`
  with a different shape — can never match the source hash; prove it is *current*
  instead (clean `git status` plus a projection mtime at or after every canonical
  source it derives from). Claiming hash equality for a conversion is a false
  gate, not a strict one.
- When evaluating prompt behavior, record the model/runtime, prompt bundle hash,
  seed revision, and exact reproduction command without copying secret values.
- Redact secrets instead of copying environment files (`.env`) into evidence.
