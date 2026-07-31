# DVA Dogfood Evaluation Reference

Domain deltas only; invariants live in
[METHODOLOGY.md](./METHODOLOGY.md).

Scoring, the cycle gate, and cross-run promotion are stage-60 concerns and live
in [60-evaluate.md](60-evaluate.md). Stage 20 needs only the surfaces and
ownership below.

## Evaluation surfaces

A surface is a routing behavior to test, not a project. The exact YAML bytes in
this block are the canonical ordered surface manifest. Stage 20 computes its
SHA-256 from the bytes between the fenced-YAML lines, including the final
newline, and records that value in `state.yaml` as `case_manifest_hash`. No hash
is stored here: a hand-maintained constant beside the bytes it describes is
updated in the same edit that changes them, so it detects nothing. The
run-to-run comparison is what carries meaning.

<!-- evaluation-manifest:start -->
```yaml
version: dva-routing-v2
surfaces:
  - id: config_schema
    discover: sections declared by the target's own dva.yml against the installed schema version
    instances: single
  - id: provision
    discover: provision entries or profiles declared by the target
    instances: single
  - id: lifecycle_boundary
    discover: a service or process owned by more than one of stack, plans, applications, interaction, or the reserved built-in command namespace
    instances: per_overlap
  - id: subproject
    discover: subprojects declared in the root dva.yml, and directories holding their own dva.yml
    instances: per_subproject
  - id: compose_profiles
    discover: profiles declared in the Compose files the target references
    instances: single
  - id: runtime_truth
    discover: long-running services the target declares with a tracked PID or bound port
    instances: single
  - id: absent_section_route
    discover: a section a command family reads (stack, applications, plans) that is absent in this target's dva.yml
    instances: per_absent_section
  - id: no_change
    discover: always instantiable
    instances: single
```
<!-- evaluation-manifest:end -->

The manifest names surfaces, never targets. It carries no expected owner and no
expected outcome.

## Deriving the run's cases

Stage 20 walks the surfaces in manifest order and instantiates each one against
the declared target:

- `instances: single` yields at most one case, with `id` equal to the surface id.
- `instances: per_subproject` / `per_overlap` yields one case per discovered
  instance, `id` = `<surface>:<instance>`, instances sorted lexically so the
  order is reproducible.
- `instances: per_absent_section` inverts the rule: the absent section is the
  instance, not a non-instance. It yields one case per section a command family
  reads that the target's `dva.yml` lacks, `id` = `absent_section_route:<section>`,
  absent sections sorted lexically. File this surface not-applicable only when no
  command family reads a section the target lacks — absence here is the thing
  tested, not the reason to skip. Each case asks whether the command's answer (a)
  states a next action, (b) states what the config does declare, and (c) is
  parseable under `--json`.
- A surface with no instance in this target is **not** a case. Record it in
  `evaluation.not_applicable_surfaces` with the evidence that showed its absence
  — the absent file, the empty section, the command output. Never invent a case
  to fill a surface; a request about project state that does not exist is an
  answer key, not evidence.

The derived ordered list is `evaluation.case_ids`. Because it is target-derived,
two runs against different targets legitimately hold different case sets while
sharing one `case_manifest_hash`; compatibility is the tuple (`version`,
`case_manifest_hash`, ordered `case_ids`), so a cross-target comparison is a
cross-run promotion question, not a contract mismatch.

`config_schema` and `no_change` are instantiable for any target that has a
`dva.yml`. If either fails to instantiate, the target is out of scope for this
workflow: block with `target_out_of_scope` rather than proceeding on one case.

## Freezing the contract

Stage 20 records `version`, the manifest SHA-256, the derived ordered `case_ids`,
and the not-applicable surfaces in `state.yaml`, then creates
`<RUN_DIR>/forward-requests.md`. That frozen file is a strict YAML document with
only `version`, `case_manifest_hash`, and ordered `requests`; every request has
only `id` and non-empty `raw_request`, one per derived case in the same order.
Its SHA-256 is stored as `evaluation.forward_requests_hash`.

Once frozen, a changed manifest, changed derived order, or changed frozen byte is
an `evaluation_contract_mismatch`: do not reuse the run's evidence; block it and
require a successor under the methodology.

## Forward test

Stage 50 first verifies this contract, then launches one independent,
history-free child session per request. A child receives only its raw request,
the disposable fixture or read-only target scope, and the safety constraints. It
receives neither surface metadata nor any expected owner/outcome. The controller
records one result for each ordered ID only after the child returns; each result
contains `id`, `child_session_id`, `request_hash`, and `outcome`.

For a completed forward test, every `child_session_id` is non-empty, unique, and
different from `controller_session_id`; identity reuse is not an independent
history-free session. The completed state also records `controller_session_id`.
Case sessions never start a real target lifecycle.

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

## Regression severity

<!-- markdownlint-disable MD013 -->

| Severity | Meaning                                                  | Response                                  |
| -------- | -------------------------------------------------------- | ----------------------------------------- |
| Critical | Secret exposure, destructive action, production mutation | Stop and report immediately               |
| High     | Invalid DVA config, broken standard workflow             | Roll back cycle-owned change and diagnose |
| Medium   | Warning, docs drift, redundant command                   | Record and route before next cycle        |
| Low      | Clarity, token, naming, minor UX issue                   | Backlog unless it is the cycle hypothesis |

<!-- markdownlint-enable MD013 -->
