---
id: TASK-245
title: "Freeze the public and filesystem contract for the config env bridge"
type: chore
priority: P0
effort: M
exec-tier: strong
created-at: 2026-09-01T19:21:00+09:00
source: "PLAN-002 env bridge decision gate"
scope: "env_file source-target model, exact CLI grammar, Git/path safety, output contract, cross-platform replace spike"
status: todo
needs-human: true
decision-status: pending
---

# Task 245: freeze the env bridge contract

## Summary

Decide the public CLI, configuration shape, filesystem safeguards, and output contract before any
security-sensitive bridge implementation begins.

## Decision required

D9 approved a bridge under the existing `config` group, not the exact `edit`/`unseal` argv or schema.
The current `env_file` accepts string, list, and object shapes and normalizes through `any`. A schema-only
addition can validate and then disappear at runtime. The command also needs a deterministic selector
when more than one env target exists.

No production implementation begins until this card is decided.

## Completion Criteria

- [ ] Choose one source↔target representation and show its behavior in every existing top-level `env_file` shape, including load, merge, show, and validation round-trip compatibility; interaction/subcommand `env_file` must reject encrypted-source metadata unless a separate runtime use case is approved | verify: human — the decision must include accepted and rejected YAML examples for both schema locations
- [ ] Freeze the exact command grammar and the zero/one/many encrypted-entry selection rule; ambiguous selection and implicit multi-target writes must fail closed | verify: human — the decision must include an argv table with text and JSON outcomes
- [ ] Define `edit` ownership and the full unseal state matrix across source/target existence, required/optional, and force | verify: human — the matrix must cover every Cartesian branch
- [ ] Define the resolution anchor for root/module/override/subproject declarations plus path containment, absolute paths, Git-outside behavior, tracked/not-ignored targets, symlink/non-regular files, source=target, and permission failures | verify: human — every location and unsafe state must name its exact resolution and whether it is non-overridable
- [ ] Limit `--force` to existing regular-target overwrite unless a separately justified security decision says otherwise; it must not silently bypass tracked, ignore, symlink, type, or path guards | verify: human — rejected alternatives and migration advice must be recorded
- [ ] Run a Linux/macOS/Windows replacement and concurrency spike; specify handle-relative or equivalent TOCTOU defense, file and parent-directory sync, atomicity, durability, cancellation cleanup, SIGKILL/power-loss limits, owned stale-temp recovery, and fail-closed behavior on an unverified platform | verify: human — evidence must include commands, OS/version, results, unresolved guarantees, and the exact supported-OS CI matrix that will keep those guarantees live
- [ ] Freeze success/error text, JSON envelope, exit codes, secret redaction, and stable machine-code policy without inventing a second root error envelope | verify: human — fixture-ready expected documents must contain no decrypted value or raw child output
- [ ] Record the selected option and why alternatives were rejected in this card before changing its status | verify: `make doc-check`

## Non-negotiable baseline

Sops is invoked without a shell, dotenv input/output is explicit, secret material never reaches DVA
output, and an ambiguous selector fails before any write. DVA does not adopt age key/provider ownership.
