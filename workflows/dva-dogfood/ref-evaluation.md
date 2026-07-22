# DVA Dogfood Evaluation Reference

Domain deltas only; invariants live in
[METHODOLOGY.md](./METHODOLOGY.md).

## Finding ownership

Assign every finding to exactly one primary owner.

<!-- markdownlint-disable MD013 -->

| Owner          | Signals                                                          | Example                                                         |
| -------------- | ---------------------------------------------------------------- | --------------------------------------------------------------- |
| Skill          | Agent lacked reusable procedure or made inconsistent decisions   | No preserve/rewrite decision rule                               |
| Prompt         | Local routing, paths, or SSoT boundary is wrong                  | packages/devbox/operate routes DVA before Compose normalization |
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
