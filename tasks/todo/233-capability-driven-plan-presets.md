---
id: TASK-233
title: "Plan presets do not close verified capabilities or enforce named lifecycle"
type: feature
priority: P1
effort: M
created-at: 2026-08-26T18:31:00+09:00
source: "DVA skill and Agent Mesh flow dogfood review"
scope: "plan preset corpus, generated AI references, DVA guide, and improve flows"
status: doing
---

# Task 233: capability-driven named plan presets

## Summary

The generation corpus still treats plans as loosely named service groups and carries migration-era
mode guidance. Generation must instead resolve one verified provider per required capability, emit
self-contained named plans, and keep platform bindings as input evidence rather than unsupported
`dva.yml` keys.

## Completion Criteria

- [ ] The canonical preset corpus defines deterministic capability closure, provider precedence,
  safe `default_plan` selection, and self-contained named plan matrices | verify: `go test ./internal/config -run 'TestPlanPresetPolicy|TestGuidedFlow'`
- [ ] Guided, automatic, and discovery flows accept and forward verified capability bindings without
  reusing stale analysis or emitting migration-only modes | verify: `go test ./internal/config -run TestGuidedFlowUsesPlanAndCapabilityContract`
- [ ] Generated AI guidance teaches exact plan discovery and `up`/`stop`/`down` symmetry and rejects
  wildcard or guessed plan selection | verify: `go test ./internal/cli -run TestDVAGuideUsesNamedPlanLifecycle`
- [ ] Generated artifacts are derived from their canonical sources and remain reproducible | verify: `make check-generate`
- [ ] Flow decision rules, documentation links, repository tests, and commit subjects pass their
  mechanical gates | verify: `make doc-check && make test && make commit-check`

## Decision

Use `local-infra` as the preferred generated default only when all selected providers are local,
verified, and non-destructive. `local-dev`, `full-stack`, `observability`, and `tools` repeat their
complete dependency closure because DVA plans do not inherit. Omit plans and defaults that lack
evidence. Portfolio-specific `capability_bindings` influence provider resolution only after their
target and ownership are verified; they are never serialized as new DVA configuration fields.
