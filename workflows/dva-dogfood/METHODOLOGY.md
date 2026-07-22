# Self-Improve Dogfood Methodology

Authoritative, domain-neutral contract for the workflow self-improvement dogfood
loop. Every workflow that runs a numbered `NN-` stage graph **instantiates this
methodology**. Each workflow keeps only its domain contract (scope, owners, case
set, domain safety) in its local `README.md` and `ref-*.md`; the invariants
below are defined here once.

This document is a specification, not an entrypoint. A run still starts from the
workflow's first stage prompt (`00-start-cycle.md`).

## Purpose

An evidence-first loop that improves the boundary between the upstream plugin's
generic knowledge and a workflow's local toolkit. One run tests one measurable
routing hypothesis, changes only its selected owner, and records append-only
evidence that can stop and resume from `RUN_DIR` without hidden conversation
context.

## Owner model

Each run selects exactly one primary owner:

| Owner          | Meaning                                             |
| -------------- | --------------------------------------------------- |
| `plugin`       | generic knowledge owned by the upstream plugin      |
| `local_setup`  | the workflow's own local execution assets           |
| `target`       | the analyzed project's own configuration            |
| `environment`  | tooling/runtime gap outside plugin and local scope  |
| `no_change`    | no current gap; proceed to forward test             |

Never mutate two owners under the same run. A new owner requires a new run, new
hypothesis, and new baseline.

## Stage spine

```text
00 start-and-audit  → contract check, revisions, inventory
10 capture-baseline → before-state, freeze raw requests, pick one owner
20 improve-plugin   ─┐ owner = plugin
30 simplify-local   ─┴ owner = local_setup   (skip the branch that does not apply)
40 forward-test     → controller replays frozen requests in
                      conversation-history-free sessions against disposable
                      fixtures and read-only real targets
50 evaluate         → score, one owner per finding, route an unresolved
                      correction back to 20/30; never edit an evaluated source
                      after evaluation
```

Stage 40 is a controller: it launches every required history-free case session
itself (native subagents); the user does not open those sessions manually. The
controller may continue from stage 20 or 30 in the same session.

**Permitted variation** — a workflow may extend the spine while preserving every
invariant here:

- `docs` adds a cross-runtime (Claude↔Codex) portability case with a hash-locked
  `portability-handoff.md`.
- `dva` inserts a skill-audit stage and a DVA-tool stage, and splits evaluate and
  feedback into stages 60 and 70.

The workflow's local numbered files and `ref-*.md` are authoritative for its
domain specifics; they must not contradict the invariants below.

## State model

`state.yaml` is a mutable index, not an evidence substitute. Invariant fields:

```yaml
run:
  id: "YYYYMMDD-HHMMSS-<6hex>"
  target_project: "/absolute/path"
  hypothesis: "one observable claim"
  mode: continuous # continuous | step
  run_dir: "/absolute/path"
  status: active # active | complete | blocked
  primary_owner: null # plugin | local_setup | target | environment | no_change
  predecessor_run_id: null # successor link after an incompatible run
  fresh_session_required: false
evaluation:
  version: "<domain>-routing-vN"
  case_ids: [ /* ordered, workflow-defined */ ]
  case_manifest_hash: "<sha256>"
  forward_requests_hash: null
revisions: { /* target/plugin/devenv heads + dirty hashes, prompt bundle hash */ }
stages: { "00": { status, latest_attempt, latest_accepted_report, attempts: [] } }
protected_paths: []
findings: []
next_prompt: "NN-name.md"
```

Attempt reports are append-only under `<RUN_DIR>/stages/NN-*/<ATTEMPT_ID>/`.
`latest_accepted_report` advances only for PASS or accepted SKIPPED. Unselected
mutation stages are marked `not_applicable` with no fake report. When stage 50
routes a correction back to 20/30, retain all attempt history but clear the
downstream accepted pointers affected by the new change.

Each attempt records `id`, timestamps, gate, and absolute report path. A run is
`complete` only when the before/after evidence is comparable, all forward-test
cases have outcomes, each finding has one owner, and the next feedback action
is recorded.

## Evaluation contract

The tuple (`version`, ordered `case_ids`, `case_manifest_hash`) defines run
compatibility. If it differs from state, block the old run with blocker code
`evaluation_contract_mismatch`; preserve its frozen `forward-requests.md` and
accepted reports; never route it back to stage 10 or 40. In continuous mode,
create a successor run with a fresh baseline and `predecessor_run_id` pointing at
the blocked run. In step mode, stop with the exact stage-00 prompt and inputs
needed to create that successor. `forward-requests.md` is frozen at stage 10 and
never rewritten.

Never disclose a case's label, expected owner, or anticipated outcome to a
forward-test session; stage 10 freezes one raw request per case and stage 40
replays it byte-for-byte in independent sessions, recording the selected owner
only after each test completes.

Results are exactly one of: `CONFIRMED` (improved, no critical regression),
`PARTIAL` (improved but an acceptance criterion unmet), `REJECTED` (no
improvement or a regression outweighs it), `INCONCLUSIVE` (missing authority or
environment blocked comparison).

## Session and resume

- `RUN_DIR` is `<TARGET_PROJECT>/tmp/dogfood-<domain>/<RUN_ID>` only after a
  `git check-ignore` probe passes; otherwise
  `${XDG_STATE_HOME:-$HOME/.local/state}/dogfood-<domain>/<project-slug>-<hash>/<RUN_ID>`.
  Never reuse or overwrite a run dir; never edit target `.gitignore` for evidence.
  If a freshly generated run path already exists, generate a new suffix; never
  fail or reuse because of a collision.
- Each stage invocation creates `ATTEMPT_ID=YYYYMMDD-HHMMSS-<4hex>` and never
  overwrites an earlier report.
- `handoff.md` must be sufficient without conversation history: RUN_DIR, target,
  hypothesis, owner, protected paths, accepted reports, blockers, run-owned
  changes, fresh-session requirement, and exact next prompt.
- `MODE=step` stops after each accepted stage; `MODE=continuous` follows
  `state.next_prompt` until a stop condition. Mode never overrides a fresh-session,
  approval, failure, or safety boundary.
- `fresh_session_required` is set after any change needing an unseeded routing
  test and cleared only once all required case sessions record unseeded results.
  If the native subagent mechanism is unavailable, keep the flag set and BLOCK
  with owner `environment`.
- An explicit `RUN_DIR`/`RESUME_RUN_DIR` wins after its real path, target, and
  ignored-or-external location are validated; without one, resume the newest
  active matching run only when unambiguous, else start a new stage-00 run.
- A new controller session reads the requested prompt, state, handoff, and
  latest accepted reports before acting.
- Reconcile complete attempt reports missing from `state.yaml`, rebuild latest
  pointers, and regenerate a stale `handoff.md` from state before continuing.
  Incomplete attempts remain historical evidence and never block a new attempt.
  Inversely, if state marks a stage complete but its report is missing, trust
  the report, not the index, and re-run the stage.
- Persist the attempt report and `state.yaml` after every stage; regenerate
  `handoff.md` and emit `RUN_DIR=`/`NEXT_PROMPT=` lines when stopping. Stop on
  FAIL, required authority, completion, or BLOCKED, except the continuous-mode
  `evaluation_contract_mismatch` successor transition.
- Read each required reference completely once per session, reused only while
  its path and Git revision are unchanged; a historical run may suggest a
  hypothesis but cannot satisfy a current gate.

## Report structure

Every attempt report contains, in order: `Scope`, `Evidence`, `Decisions`,
`Changes`, `Validation`, `Findings`, `Gate` (PASS | FAIL | BLOCKED | SKIPPED),
`Next` (exact prompt filename). Record paths, revisions, hashes, bounded output,
and exit codes. Redact secret values and private content. Forward-test reports
also record model/runtime, prompt bundle hash, target revision, the exact raw
request hash, and the fixture or read-only inspection scope.

Stage-40 reports also record the controller session identity and every case
session identity; a plugin or local source edit alone is not evidence that a
fresh session used the changed guidance.

Link to existing files rather than duplicating their contents. Preserve
unexpected output as a finding even when the command exits zero.

## Safety invariants

- Append-only evidence; `state.yaml`/`handoff.md` are indexes only.
- No stage commits, pushes, or performs an irreversible target action (starting a
  service, removing a volume, running a lifecycle target, disclosing a secret).
- One owner per run; no post-evaluation source change.
- Disposable, seeded fixtures for forward tests; real targets are read-only.
- Before editing plugin or local prompts, record scoped Git status, revision,
  protected paths, and exact paths this run may change; each edit path is
  clean or has an exact pre-edit patch and inverse recorded, leaving unrelated
  dirty paths untouched.
- On failed validation, reverse only the run-owned captured patch and preserve
  the failure evidence; never use a destructive Git command.
- Record a secret by name and pattern, never its value; never place literal
  credentials, private file contents, or unrelated dirty-file content in
  evidence.

## What each workflow defines locally

The workflow's `README.md` and `ref-*.md` own, and only own:

- domain scope and the plugin-knowledge ↔ local-toolkit boundary
- the forbidden domain actions specific to its blast radius
- the ordered evaluation case set and metrics (`ref-evaluation.md`)
- domain-specific safety beyond these invariants (`ref-safety.md`)
- the `dogfood-<domain>` path slug and per-run evaluation data block
