---
id: TASK-248
title: "Enforce the decided required env policy without breaking diagnostics"
type: feature
priority: P0
effort: L
exec-tier: standard
created-at: 2026-09-01T19:24:00+09:00
source: "PLAN-002 and TASK-247 current-loader safety decision"
scope: "loadEnv result model, all CLI callers, doctor hints, text/JSON fixtures, child-start guards"
status: todo
depends-on: [TASK-247]
---

# Task 248: enforce required env behavior by command

## Summary

Implement TASK-247's caller matrix before adding encrypted-source mutation, while preserving optional-file
semantics and complete doctor diagnostics.

## Problem

Every caller must implement TASK-247's public behavior without duplicating inconsistent policy or
cutting off the command that diagnoses the missing file.

## Completion Criteria

- [ ] Refactor environment loading so callers can distinguish required true/false, missing, inaccessible, malformed, and multi-file partial-merge state while successful precedence and caching remain unchanged | verify: `go test ./internal/config ./internal/cli -count=1`
- [ ] Implement every TASK-247 matrix row with table-driven text/JSON/exit tests and prove fail-closed rows start no external child process | verify: `go test ./internal/cli -count=1`
- [ ] Preserve complete doctor output in default and strict modes; refine existing env-file checks and source-aware hints rather than adding a duplicate check | verify: `go test ./internal/cli -count=1`
- [ ] Keep stdout to one JSON document, use the existing root error envelope where the decision calls for failure, and keep human diagnostics off JSON stdout | verify: `go test ./internal/cli -count=1`
- [ ] Optional missing files remain skipped, optional existing unreadable/malformed files remain explicit errors, no execution continues on accidental partial merge, and no command invents an unseal hint without recognized source metadata from a later approved bridge | verify: `go test ./internal/config ./internal/cli -count=1`
- [ ] Usage and migration documentation name the behavior of observation, execution, teardown, and doctor commands | verify: `make doc-check`
- [ ] Repository gates pass | verify: `make lint && make test && make test-integration && make commit-check`

## Non-goals

- No implicit unseal.
- No config env command, encrypted-source schema, or sops invocation.
- No change to optional env-file absence.
- No promotion of doctor default mode into a release gate.
