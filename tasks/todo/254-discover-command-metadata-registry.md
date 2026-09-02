---
id: TASK-254
title: "Discover a command metadata registry boundary"
type: chore
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-02T10:06:00+09:00
source: "PLAN-003 command discovery ownership investigation"
scope: "command metadata inventory, derivation matrix, consistency gates, and implementation recommendation"
status: todo
---

# Task 254: discover command metadata ownership

## Summary

Map every owner of public command metadata and produce a decision-ready boundary for a possible registry.
The result is evidence for later architecture work; this task does not introduce a registry or change the
command surface.

## Completion Criteria

- [ ] Inventory Cobra registration, names, aliases, groups, arguments, flags and descriptions; reserved names; hookable commands; manifest type/options/descriptions; direct-help wrappers; collision validation; completion and generated AI documentation | verify: human — the inventory must cite exact tracked paths and symbols and distinguish generated projections from canonical owners
- [ ] For every metadata field, classify whether it can be derived, must remain hand-authored, or needs an explicit consistency assertion; document current duplication and the failure each existing guard prevents | verify: human — the matrix must cover runtime routing, help, manifest, completion, reserved-name validation, and generated documentation
- [ ] Compare at least a minimal shared descriptor, Cobra-as-SSOT derivation, and the current split ownership; state initialization-cycle, dynamic-command, passthrough-option, testability, and migration tradeoffs | verify: human — no option may be selected solely because it removes line duplication
- [ ] Append a canonical `## Evidence and Recommendation` section to this card with the smallest coherent ownership boundary and a staged migration seam, or record that current ownership should remain; any implementation must be proposed as new bounded cards | verify: `make doc-check`
- [ ] Existing command and reserved-name tests pass without production changes | verify: `go test ./internal/cli ./internal/config -count=1`

## Non-goals

- No `CommandSpec` or equivalent production registry.
- No route, alias, help group, manifest schema, or generated-document change.
- No weakening of current registration and reserved-name consistency checks.
