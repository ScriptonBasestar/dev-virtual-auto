# DVA Dogfood Evaluation Reference

Domain deltas only; invariants live in
[METHODOLOGY.md](./METHODOLOGY.md).

## Evaluation manifest

The exact YAML bytes in this block are the canonical ordered DVA case manifest.
Stage 20 computes its SHA-256 from the bytes between the fenced-YAML lines,
including the final newline, and records that value in `state.yaml`. No hash is
stored here: a hand-maintained constant beside the bytes it describes is updated
in the same edit that changes them, so it detects nothing. The run-to-run
comparison is what carries meaning. This manifest contains only case identity
and coverage surface; it intentionally has no expected-owner or expected-outcome
field.

<!-- evaluation-manifest:start -->
```yaml
version: dva-routing-v1
cases:
  - id: config-schema-ownership
    surface: config_schema
  - id: provision-safety
    surface: provision
  - id: root-workers-lifecycle-ownership
    surface: lifecycle_boundary
  - id: subproject-engine
    surface: subproject
  - id: subproject-workers
    surface: subproject
  - id: subproject-transformer
    surface: subproject
  - id: subproject-e2e
    surface: subproject
  - id: compose-profiles
    surface: compose_profiles
  - id: health-runtime-truth
    surface: runtime_truth
  - id: no-change
    surface: no_change
```
<!-- evaluation-manifest:end -->

Stage 20 copies these ordered IDs, `version`, and the manifest SHA-256 into
`state.yaml`, then creates `<RUN_DIR>/forward-requests.md`. That frozen file is
a strict YAML document with only `version`, `case_manifest_hash`, and ordered
`requests`; every request has only `id` and non-empty `raw_request`. Its
SHA-256 is stored as `evaluation.forward_requests_hash`. A changed manifest,
order, or frozen byte is an `evaluation_contract_mismatch`: do not reuse the
run's evidence; block it and require a successor under the methodology.

Stage 50 first verifies this contract, then launches one independent,
history-free child session per request. A child receives only its raw request,
the disposable fixture or read-only target scope, and the safety constraints.
It receives neither case metadata nor any expected owner/outcome. The
controller records one result for each ordered ID only after the child returns;
each result contains `id`, `child_session_id`, `request_hash`, and `outcome`.
For a completed forward test, every `child_session_id` is non-empty, unique,
and different from `controller_session_id`; identity reuse is not an
independent history-free session. The completed state also records
`controller_session_id`. Case sessions never start a real target lifecycle.

`stages.50.status` is structurally one of `pending`, `complete`, `blocked`, or
`not_applicable`. Only `complete` may claim a finished forward test, and it
requires the controller plus exactly one valid ordered result per case. An
unknown success-like value (for example `PASS`) is an
`evaluation_contract_mismatch`, never an incomplete-run escape hatch.

## Finding ownership

Assign every finding to exactly one primary owner.

<!-- markdownlint-disable MD013 -->

| Owner          | Signals                                                          | Example                                                         |
| -------------- | ---------------------------------------------------------------- | --------------------------------------------------------------- |
| Skill          | Agent lacked reusable procedure or made inconsistent decisions   | No preserve/rewrite decision rule                               |
| Prompt         | Stage routing, references, or SSoT boundary is wrong             | Baseline stage routes DVA before Compose normalization          |
| DVA tool       | CLI output, schema validation, discovery, or doctor is incorrect | Contradictory daemon checks                                     |
| Target project | Project configuration or docs do not match reality               | stale Compose filename in `dva.yml`                             |
| Environment    | Required local executable/service is unavailable                 | `am` missing from PATH                                          |

<!-- markdownlint-enable MD013 -->

Do not patch the target project to silence a DVA tool defect. Do not add a
machine-specific workaround to a generic skill.

A green status/health verdict while the tracked PID is dead, or while the port
is answered by a process outside the group DVA started, is always a DVA-tool
finding — even when a target-config defect (e.g. a wrong run path) also
contributed. Route both owners; the config fix alone does not close it, because
a config defect can mask a tool defect (a stale orphan keeps answering the
port). Reported state must reflect the process DVA controls, not merely a port
that responds.

## Evaluation dimensions

Score each applicable dimension from 0 to 2. Mark unrelated dimensions `N/A`.

<!-- markdownlint-disable MD013 -->

| Dimension    | 0                                         | 1                                     | 2                                              |
| ------------ | ----------------------------------------- | ------------------------------------- | ---------------------------------------------- |
| Triggering   | Skill absent/not triggered                | Explicit invocation only              | Correct explicit and natural triggering        |
| Correctness  | Wrong or unsafe result                    | Mostly correct with material warnings | Correct and evidence-backed                    |
| Reuse        | Target-specific logic embedded            | Some reusable separation              | Clean generic/local/project ownership          |
| Efficiency   | Repeated broad scans or excessive context | Minor duplication                     | Bounded scans and progressive disclosure       |
| Safety       | Secret/user/prod risk                     | Protected with gaps                   | Protected paths and approval gates enforced    |
| Validation   | No meaningful checks                      | Partial checks                        | Layered current-state checks                   |
| Runtime truth | Reported state ignores what DVA controls | Probe-only; port ownership unverified | Status/health reflect the tracked process owning its port |
| Ownership    | Same service/check has multiple owners    | Overlap found but incompletely routed | One lifecycle owner and one SSoT per behavior  |
| Traceability | Cannot explain result                     | Partial evidence                      | Baseline, diff, findings, owner, result linked |

<!-- markdownlint-enable MD013 -->

The score is diagnostic, not the cycle gate. Report earned and applicable
points, for example `12/14`; do not penalize `N/A`. Cycle PASS is determined by
the mandatory criteria below, with Safety, Validation, and Ownership all at 2.

## Regression severity

<!-- markdownlint-disable MD013 -->

| Severity | Meaning                                                  | Response                                  |
| -------- | -------------------------------------------------------- | ----------------------------------------- |
| Critical | Secret exposure, destructive action, production mutation | Stop and report immediately               |
| High     | Invalid DVA config, broken standard workflow             | Roll back cycle-owned change and diagnose |
| Medium   | Warning, docs drift, redundant command                   | Record and route before next cycle        |
| Low      | Clarity, token, naming, minor UX issue                   | Backlog unless it is the cycle hypothesis |

<!-- markdownlint-enable MD013 -->

## Cycle gate

Stage-60 evaluation PASS requires:

- comparable before/after evidence;
- all findings assigned to an owner;
- no unresolved critical/high regression introduced by the cycle;
- source skill installation and fresh-session behavior checked when skill
  changed;
- every Compose service, native process, check, and provision action has one
  lifecycle owner;

Final cycle closure additionally requires one singular, measurable next
hypothesis selected by stage 70.

## Cross-run promotion

A generic skill or prompt improvement remains provisional after one target.
Before presenting it as a reusable best practice, validate it in a separate run
against at least one structurally different target. Compare models only when
model sensitivity is the stated hypothesis; do not multiply models by default.
Cross-run evidence may support promotion but never replaces current-run gates.
