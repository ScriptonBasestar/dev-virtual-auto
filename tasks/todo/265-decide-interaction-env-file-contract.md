---
id: TASK-265
title: "Decide the interaction-level env_file compatibility contract"
type: chore
priority: P0
effort: M
exec-tier: strong
created-at: 2026-09-02T15:50:45+09:00
source: "TASK-247 full env-input audit"
scope: "interaction and subcommand env_file schema, inert runtime behavior, precedence and owner options, compatibility evidence, diagnostics, migration and implementation handoff"
status: todo
needs-human: true
decision-status: pending
depends-on: [TASK-247]
---

# Task 265: decide the interaction env_file contract

## Summary

Decide whether the schema-valid but currently inert `interaction.*.env_file` field becomes a supported
runtime input or is deprecated and rejected through a versioned migration. Do not let TASK-248 silently
choose either behavior while implementing top-level env-file failure policy.

## Problem

`InteractionCommand.EnvFile` is parsed, stored and merged at interaction/subcommand depth, and a tracked
example declares it, but no CLI or runner consumes it. A user can therefore write a valid configuration
that appears to require a file while the command runs without it. Starting to honor the field adds file I/O,
precedence, inheritance and child-owner semantics; removing it immediately is also a compatibility break.

## Recommended direction

Prefer versioned deprecation and rejection unless pinned usage evidence proves a distinct per-interaction
file is necessary beyond top-level `env_file` plus interaction `environment`. Silent inert acceptance is not
viable. If support is selected, freeze owner-relative path anchoring, parent/subcommand inheritance,
top-level-versus-command precedence, required/optional failure behavior and TASK-247 output parity first.

## Completion Criteria

- [ ] Inventory schema, decode, module/override merge, interaction-tree inheritance, tracked examples, documentation and every runtime consumer; record that current execution effect is absent or cite contrary code | verify: human — every claimed producer and consumer must cite a tracked symbol or fixture
- [ ] Build a pinned, secret-free usage corpus from tracked DVA files and canonical consumer repositories, recording repository IDs, revisions, paths, dynamic limitations and whether declarations rely on runtime loading | verify: human — unpinned or unavailable evidence remains an explicit finding
- [ ] Compare support, versioned deprecation/rejection and inert compatibility for owner/path anchoring, subcommands, precedence, required failures, text/JSON, security, migration and rollback | verify: human — silent inert behavior may not be selected as the permanent contract
- [ ] If support is selected, freeze exact precedence and inheritance plus root/direct-child/imported canonical/alias fixtures and assign implementation to TASK-248 or a bounded child before TASK-248 starts | verify: human — no runtime file I/O may be added from an unspecified contract
- [ ] If rejection is selected, freeze warning/error releases, config validate behavior, migration command or message, schema timing, rollback and the disposition of tracked examples | verify: human — immediate unannounced schema rejection is not allowed
- [ ] Record an independently reviewed `## Decision Record`, change `decision-status` to `decided`, and update TASK-248 dependencies if a new implementation child is required | verify: `make doc-check`

## Non-goals

- No field support, deprecation warning or schema removal in this decision card.
- No top-level `env_file` or encrypted-source contract change.
- No imported command ownership implementation; TASK-264 owns it.
