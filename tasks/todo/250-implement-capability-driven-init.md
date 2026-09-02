---
id: TASK-250
title: "Implement evidence-based init for people and agents"
type: feature
priority: P1
effort: L
exec-tier: standard
created-at: 2026-09-01T19:26:00+09:00
source: "TASK-249 decision"
scope: "init discovery and generation, plan preset integration, skills/workflows, fixtures, usage documentation"
status: todo
depends-on: [TASK-244, TASK-249]
---

# Task 250: implement capability-driven init

## Summary

Generate only verified, self-contained plans through one canonical path shared by people and agents.

## Problem

`init` must offer a useful shared starting point without inventing capabilities or turning measured
plan names into a schema-level vocabulary.

## Completion Criteria

- [ ] Implement TASK-249's compose-only, native-only, hybrid, no-discovery, and public argv/help compatibility outcomes for all five templates, four flags, `config init`, and the top-level alias using one canonical generation path | verify: `/usr/bin/grep -Eq '^func TestInitPublicSurfaceCompatibility\(' internal/cli/init_test.go && go test ./internal/cli -count=1`
- [ ] Every generated plan contains a verified self-contained entry closure; absent evidence omits the plan instead of emitting an empty or placeholder plan | verify: `go test ./internal/cli -count=1`
- [ ] Generated configurations pass config validation, merged show, explicit lifecycle selection, and the decided bare lifecycle default behavior | verify: `make test-integration`
- [ ] Existing config files are never overwritten implicitly; preview/idempotence and conflicting discovery behave exactly as TASK-249 decided | verify: `go test ./internal/cli -count=1`
- [ ] Generated output does not immediately trigger D6/D7 and never authors `local-infra`, `local-dev`, or `full-stack`; those names survive only when already declared by the user | verify: `/usr/bin/grep -Eq '^func TestInitDoesNotAuthorRejectedPlanLabels\(' internal/cli/init_test.go && go test ./internal/config ./internal/cli -count=1`
- [ ] Human CLI and agent skill/workflow consume the same canonical preset/generator and generated projections remain reproducible | verify: `make check-generate`
- [ ] Usage docs explain evidence, omissions, editing, and default selection without presenting corpus frequency as a contract | verify: `make doc-check`
- [ ] Repository gates pass | verify: `make lint && make test && make test-integration && make check-generate && make commit-check`

## Non-goals

- No migration of existing devboxes.
- No fixed archetype labels in schema or validation.
- No guessed native command or runner.
