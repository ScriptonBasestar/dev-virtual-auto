---
id: TASK-249
title: "Redesign init around verified capabilities instead of a fixed three-plan template"
type: chore
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-01T19:25:00+09:00
source: "PLAN-002 tracked D8-compatible scaffold ruling"
scope: "init discovery contract, capability preset integration, plan naming/default rules, human-agent parity, census ownership"
status: todo
needs-human: true
decision-status: pending
---

# Task 249: redesign capability-driven init

## Summary

Replace the rejected fixed three-plan template with an evidence-driven generation contract aligned
with D8 and the repository's capability preset policy.

## Decision required

The proposed unconditional `local-infra`, `local-dev`, and `full-stack` scaffold conflicts with D8,
contains invalid top-level fields, and asks the current Compose-only detector to invent a native runner.
It can also generate empty or duplicate plans that immediately trigger D6.

## Completion Criteria

- [ ] Define expected discovery evidence and generated output for compose-only, native-only, hybrid, and no-discovery fixtures | verify: human — each fixture must list detected facts, unverified facts, generated entries/plans, and explicit omissions
- [ ] Reuse the repository's capability-driven preset policy: generate only self-contained closure from verified providers and omit plans that lack evidence | verify: human — no output may depend on a role label inferred only by a person
- [ ] Keep `local-infra`, `local-dev`, and `full-stack` out of generated defaults unless a future tracked decision explicitly reopens D8; preserve an existing user-declared name, otherwise derive a name mechanically from verified entry/provider identity or require explicit user choice | verify: human — the decision must contain no generator-authored exception for the three rejected labels
- [ ] Decide single-plan implicit default versus explicit `default_plan`, and prove generated multi-plan output never lacks a default | verify: human — selection must align with bare lifecycle behavior
- [ ] Define no-overwrite, preview/dry-run, idempotence, and invalid partial-discovery behavior | verify: human — mutation must not begin from unresolved or conflicting evidence
- [ ] Make human `init` and agent workflows consume one canonical generator/preset rather than copied templates | verify: human — ownership and generation direction must be named
- [ ] Freeze a backward-compatibility matrix for `minimal`, `rails`, `node`, `python`, `go`, `--recursive`, `--devcontainer`, `--all`, `config init`, and the visible top-level `init` alias; every surface must be preserved, explicitly deprecated, or deliberately removed with migration evidence | verify: human — exact argv/help/output expectations are required
- [ ] Define census owner, canonical repository IDs/revisions, input inventory, cadence, and the change threshold that can revise defaults | verify: human — a bare count without revision is insufficient
- [ ] Record the selected contract and alternatives rejected in this card | verify: `make doc-check`

## Rejected baseline

Do not generate a fixed three-plan template merely because those names are common in the measured
corpus. Frequency is input evidence, not proof that a particular repository has those capabilities.
Capability evidence can justify a plan's existence, but does not by itself justify one of the three
rejected labels.
