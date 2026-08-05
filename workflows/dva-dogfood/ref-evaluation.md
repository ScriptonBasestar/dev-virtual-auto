# DVA Dogfood Evaluation Reference

How a run derives its test cases, freezes them, and assigns every finding one
owner. Scoring and the cycle gate live in [40-evaluate.md](40-evaluate.md).

## Evaluation surfaces

A surface is a routing behavior to test, not a project. Stage 10 walks this
manifest in order and instantiates each surface against the declared target.

```yaml
manifest_version: dva-routing-v2
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

The manifest names surfaces, never targets. It carries no expected owner and no
expected outcome.

## Deriving the run's cases

**Verify the instance exists before writing the case.** A case is a question about
project state; a question about state the target does not have cannot be answered
honestly, and freezing one poisons the whole run.

- `instances: single` yields at most one case, `id` = the surface id.
- `instances: per_subproject` / `per_overlap` yields one case per discovered
  instance, `id` = `<surface>:<instance>`, instances sorted lexically.
- `instances: per_absent_section` inverts the rule: the absent section *is* the
  instance. It yields one case per section a command family reads that this
  target's `dva.yml` lacks, `id` = `absent_section_route:<section>`, sorted
  lexically. File this surface not-applicable only when no command family reads a
  section the target lacks. Each case asks whether the command's answer (a) states
  a next action, (b) states what the config does declare, and (c) is parseable
  under `--json`.
- A surface with no instance is **not** a case. Record it in
  `evaluation.not_applicable_surfaces` with the evidence that showed its absence —
  the absent file, the empty section, the command output.

A small case set is a valid outcome. Three honest cases beat ten invented ones, and
a target that instantiates few surfaces is a finding about that target, not a
blocked run. The only out-of-scope condition is a target where `config_schema` and
`no_change` both fail to instantiate — that means no `dva.yml` exists, so the
workflow does not apply: block with `target_out_of_scope`.

Because case sets are target-derived, two runs against different targets
legitimately hold different case sets. Comparing them is a cross-run promotion
question, not an error.

## Freezing the requests

Stage 10 records `manifest_version`, the derived ordered `case_ids`, and the
not-applicable surfaces in `state.yaml`, then creates `<RUN_DIR>/forward-requests.md`:
a strict YAML document with `manifest_version` and ordered `requests`, where every
request has only `id` and a non-empty `raw_request`, one per derived case in the
same order. Store its full-file SHA-256 as `evaluation.forward_requests_hash`.

The freeze exists for one reason: **the controller must not be able to reword a
request after seeing baseline results.** Stage 30 recomputes the hash before
launching any case session and blocks if it differs.

Requests carry no expected-owner and no expected-outcome field. Never disclose a
case's label, surface, or anticipated result to a forward-test session.

## Forward test

Stage 30 launches one independent, history-free child session per request. A child
receives only its raw request, the disposable fixture or read-only target scope,
and the safety constraints — never surface metadata or an expected outcome.

The controller records one result per ordered ID only after the child returns:
`{id, child_session_id, request_hash, outcome}`. Every `child_session_id` must be
non-empty, unique, and different from `controller_session_id`; reusing an identity
is not an independent history-free session.

`stages.30.status` is one of `pending`, `complete`, `blocked`, `not_applicable`.
Only `complete` may claim a finished forward test, and it requires the controller
plus exactly one valid ordered result per case. A case session never starts a real
target lifecycle.

## Results

Every run classifies as exactly one of:

| Result         | Meaning                                                  |
| -------------- | -------------------------------------------------------- |
| `CONFIRMED`    | improved, no critical regression                         |
| `PARTIAL`      | improved but an acceptance criterion is unmet            |
| `REJECTED`     | no improvement, or a regression outweighs it             |
| `INCONCLUSIVE` | missing authority or environment blocked the comparison  |

## Finding ownership

Assign every finding to exactly one owner.

<!-- markdownlint-disable MD013 -->

| Owner            | Signals                                                          | Example                                                |
| ---------------- | ---------------------------------------------------------------- | ------------------------------------------------------ |
| `skill`          | Agent lacked a reusable procedure or made inconsistent decisions | No preserve/rewrite decision rule                      |
| `prompt`         | Stage routing, references, or SSoT boundary is wrong             | Baseline stage routes DVA before Compose normalization |
| `dva_tool`       | CLI output, schema validation, discovery, or doctor is incorrect | Contradictory daemon checks                            |
| `target_project` | Project configuration or docs do not match reality               | Stale Compose filename in `dva.yml`                    |
| `environment`    | A required local executable or service is unavailable            | `am` missing from PATH                                 |

<!-- markdownlint-enable MD013 -->

Do not patch the target project to silence a DVA tool defect. Do not add a
machine-specific workaround to a generic skill. Do not teach the skill to work
around a defect the tool should report itself.

A green status or health verdict while the tracked PID is dead, or while the port
is answered by a process outside the group DVA started, is always a `dva_tool`
finding — even when a target-config defect also contributed. Route both owners; the
config fix alone does not close it, because a config defect can mask a tool defect
when a stale orphan keeps answering the port. Reported state must reflect the
process DVA controls, not merely a port that responds.

## Regression severity

<!-- markdownlint-disable MD013 -->

| Severity | Meaning                                                  | Response                                  |
| -------- | -------------------------------------------------------- | ----------------------------------------- |
| Critical | Secret exposure, destructive action, production mutation | Stop and report immediately               |
| High     | Invalid DVA config, broken standard workflow             | Roll back the cycle-owned change, diagnose |
| Medium   | Warning, docs drift, redundant command                   | Record and route before the next cycle    |
| Low      | Clarity, token, naming, minor UX issue                   | Backlog unless it is the cycle hypothesis |

<!-- markdownlint-enable MD013 -->
