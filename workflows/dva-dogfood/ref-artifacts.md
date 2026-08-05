# DVA Dogfood Artifact and Session Reference

Evidence layout, run state, report structure, and the resume protocol.

## Layout

**A stage's evidence directory is named exactly after its prompt file, without the
`.md` suffix.** Any other name is a defect: two names for one stage split the
evidence across directories and leave empty ghosts behind.

```text
<RUN_DIR>/
├── state.yaml
├── handoff.md
├── forward-requests.md
└── stages/
    ├── 00-start/<ATTEMPT_ID>/report.md
    ├── 10-baseline/<ATTEMPT_ID>/report.md
    ├── 20-improve/<ATTEMPT_ID>/report.md
    ├── 30-forward-test/<ATTEMPT_ID>/report.md
    └── 40-evaluate/<ATTEMPT_ID>/report.md
```

Attempt reports are append-only. `ATTEMPT_ID` is `YYYYMMDD-HHMMSS-<4hex>` and a
stage invocation never overwrites an earlier report. `state.yaml` and `handoff.md`
are mutable indexes, never evidence substitutes.

## State schema

```yaml
run:
  id: "YYYYMMDD-HHMMSS-<6hex>"
  target_project: "/absolute/path"
  hypothesis: "one observable claim"
  mode: continuous # continuous | step
  run_dir: "/absolute/path"
  created_at: "RFC3339 timestamp"
  status: active # active | complete | blocked
  blocked_reason: null
  predecessor_run_id: null
  fresh_session_required: false
  owner: null # skill | prompt | dva_tool | target_project | environment | no_change

evaluation:
  manifest_version: "dva-routing-v2"
  case_ids: [] # ordered, derived from the surfaces for THIS target
  not_applicable_surfaces: [] # {surface, evidence} per surface with no instance
  forward_requests_hash: null # sha256 of the frozen forward-requests.md
  forward_test:
    controller_session_id: null
    cases: [] # {id, child_session_id, request_hash, outcome} per ordered case

revisions:
  target_head: null
  target_dirty_hash: null # sha1 of `git status --porcelain` run from the repo root
  dva_head: null
  dva_dirty_hash: null # same derivation, or the values cannot be compared
  skill_source_hash: null
  installed_skill_hash: null
  prompt_bundle_hash: null

sources:
  dva_executable: null
  dva_version: null
  dva_build_commit: null
  candidate_dva_executable: null # stage 20 build; never overwrites installed provenance
  candidate_dva_build_commit: null
  skill_source: null
  skill_installed: null
  config_projection: null # active | missing

authority:
  scope: numbered_stage # numbered_stage | post_cycle_qa
  approved: false
  commands: [] # exact entries: {command: "...", side_effect: "declared_effect"}

stages:
  "00": { status: complete, latest_attempt: null, latest_accepted_report: null, attempts: [] }
  # Repeat the same structure for 10, 20, 30, and 40.
  # status: pending | complete | blocked | not_applicable

protected_paths: []
findings: []   # {id, owner, severity, evidence, summary}
backlog: []    # findings whose owner is not this run's owner
next_prompt: "10-baseline.md"
```

`latest_attempt` tracks the newest terminal report; `latest_accepted_report` tracks
the newest PASS or accepted SKIPPED report. A stage excluded by the run's owner is
`not_applicable` with **no** attempt report — never a fabricated one.
`next_prompt` always names the next invoked stage, not the next numeric one.

`fresh_session_required` is set by any change needing an unseeded routing test and
cleared only once the required case sessions record unseeded results. A failed
check leaves it set and blocks further mutation.

When stage 40 routes a correction back to stage 20 within the run's existing owner,
mark stage 20 pending, clear the downstream `latest_accepted_report` pointers the
new change affects, and retain all attempt history. Reactivating stage 20 also
clears candidate DVA provenance until a new executable passes its gate.

`sources.config_projection: missing` is an `environment` finding. It never permits
installing or synthesizing a projection. `authority` accepts runtime work only at
`scope: post_cycle_qa` with approved exact `{command, side_effect}` entries
matching every invoked lifecycle command; a generic approval is not authority, and
valid post-cycle authority still does not unlock a numbered stage.

## Report structure

Every attempt report contains, in order: `Scope`, `Evidence`, `Decisions`,
`Changes`, `Validation`, `Findings`, `Gate` (PASS | FAIL | BLOCKED | SKIPPED),
`Next` (exact prompt filename).

- `Scope` lists the repositories and files inspected or edited; `Changes` lists the
  files changed, or `None` for a read-only stage.
- `Findings` use stable IDs such as `SKILL-001`, `PROMPT-001`, `DVA-001`,
  `PROJECT-001`.
- Record paths, revisions, hashes, bounded output, and exit codes. Redact secret
  values and private content.
- Link to existing files rather than duplicating their contents.
- Preserve unexpected output as a finding even when the command exits zero.
- Forward-test reports additionally record model/runtime, prompt bundle hash,
  target revision, each raw request hash, the fixture or read-only inspection
  scope, the controller session identity, and every case session identity. A source
  edit alone is not evidence that a fresh session used the changed guidance.

## Evidence rules

- Record installed DVA version/commit separately from `DVA_ROOT` HEAD and dirty
  hash; equality must be proven, never inferred.
- Record the canonical skill hash separately from each projection, and prove the
  relation the projection's declared shape supports. A copy or symlink target must
  be proven *identical* (same hash, or same inode for a symlink). A conversion
  target — anything `skills/_targets.yaml` marks `generated: true` with a different
  shape — can never match the source hash; prove it is *current* instead (clean
  `git status` plus a projection mtime at or after every canonical source it
  derives from). Claiming hash equality for a conversion is a false gate.
- Redact secrets instead of copying environment files (`.env`) into evidence.
  Record a secret by name and pattern, never by value.

## New run and resume

- `RUN_DIR` is `<TARGET_PROJECT>/tmp/dogfood-dva/<RUN_ID>` only after a
  `git check-ignore` probe passes; otherwise
  `${XDG_STATE_HOME:-$HOME/.local/state}/dogfood-dva/<project-slug>-<hash>/<RUN_ID>`.
- Never write evidence into a Git-trackable path and never edit the target
  `.gitignore` to make one ignorable.
- Never reuse or overwrite a run dir. If a freshly generated path already exists,
  generate a new suffix; a collision never fails a run.
- An explicit `RUN_DIR`/`RESUME_RUN_DIR` wins after its real path, target, and
  ignored-or-external location are validated. Without one, resume the newest active
  matching run only when unambiguous; otherwise start a new stage-00 run. Existing
  unrelated or completed runs never block a new run.
- A new controller session reads the requested prompt, state, handoff, and latest
  accepted reports before acting.
- Reconcile complete attempt reports missing from `state.yaml`, rebuild latest
  pointers, and regenerate a stale `handoff.md` before continuing. Incomplete
  attempts remain historical evidence and never block a new attempt. Inversely, if
  state marks a stage complete but its report is missing, trust the report and
  re-run the stage.
- Initially inspect at most the three newest sibling runs. A historical run may
  suggest a hypothesis but can never satisfy a current gate.

## Session boundaries

A session boundary occurs after every accepted PASS or accepted SKIPPED stage in
`MODE=step`, on BLOCKED or FAIL, when a stage requires user authority, whenever
`fresh_session_required` becomes true, and when the run ends.

Every boundary regenerates `handoff.md` and emits literal `RUN_DIR=` and
`NEXT_PROMPT=` lines. A completion boundary names the separately authorized
post-cycle QA prompt, or `none`; it never starts runtime work from a numbered
stage.

`handoff.md` must be sufficient without conversation history: RUN_DIR, target,
hypothesis, owner, protected paths, the latest attempt for every attempted stage,
accepted reports, blockers, run-owned changes, installed/candidate/selected DVA
provenance, the fresh-session requirement, whether the next session resumes or
creates a new attempt, and the next prompt with its exact invocation.
