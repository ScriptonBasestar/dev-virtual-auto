---
id: TASK-233
title: "Plan presets do not close verified capabilities or enforce named lifecycle"
type: feature
priority: P1
effort: M
created-at: 2026-08-26T18:31:00+09:00
source: "DVA skill and Agent Mesh flow dogfood review"
scope: "plan preset corpus, generated AI references, DVA guide, and improve flows"
status: done
completed-at: 2026-08-26T18:34:00+09:00
completion-summary: "Generate evidence-backed capability closures as self-contained named plans and accept portfolio provider policy only through verified input bindings."
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "make test && make doc-check && make check-generate && make test-integration && make test-skill-dogfood && make commit-check"
    result: "all requested gates passed"
  - kind: baseline
    command-or-step: "dva lint in task worktree and source checkout"
    result: "task worktree retained two pre-existing findings also present on source: unusedwrite in internal/skillinstall/install_test.go and modernize in internal/skillinstall/agent_mesh.go"
quality-review: pass
quality-reviewed-at: 2026-08-26T18:34:00+09:00
quality-review-evidence:
  - "architecture judge selected prompt/library capability bindings over public plan inheritance or runtime capability profiles"
  - "independent review findings for stale analysis, masked lifecycle failure, ownership shapes, generic guide names, product coupling, and corpus coverage were fixed"
archived-at: 2026-08-26T18:34:00+09:00
verified-at: 2026-08-26T18:34:00+09:00
verification-summary: "Named-plan corpus, flows, generated guide, and binding contract pass unit, race, integration, documentation, generation, dogfood, and commit gates."
---

# Task 233: capability-driven named plan presets

## Summary

The generation corpus still treats plans as loosely named service groups and carries migration-era
mode guidance. Generation must instead resolve one verified provider per required capability, emit
self-contained named plans, and keep platform bindings as input evidence rather than unsupported
`dva.yml` keys.

## Completion Criteria

- [x] The canonical preset corpus defines deterministic capability closure, provider precedence,
  safe `default_plan` selection, and self-contained named plan matrices | verify: `go test ./internal/config -run 'TestPlanPresetPolicy|TestGuidedFlow'`
- [x] Guided, automatic, and discovery flows accept and forward verified capability bindings without
  reusing stale analysis or emitting migration-only modes | verify: `go test ./internal/config -run TestGuidedFlowUsesPlanAndCapabilityContract`
- [x] Generated AI guidance teaches exact plan discovery and `up`/`stop`/`down` symmetry and rejects
  wildcard or guessed plan selection | verify: `go test ./internal/cli -run TestDVAGuideUsesNamedPlanLifecycle`
- [x] Generated artifacts are derived from their canonical sources and remain reproducible | verify: `make check-generate`
- [x] Flow decision rules, documentation links, repository tests, and commit subjects pass their
  mechanical gates | verify: `make doc-check && make test && make commit-check`

## Decision

Use `local-infra` as the preferred generated default only when all selected providers are local,
verified, and non-destructive. `local-dev`, `full-stack`, `observability`, and `tools` repeat their
complete dependency closure because DVA plans do not inherit. Omit plans and defaults that lack
evidence. Portfolio-specific `capability_bindings` influence provider resolution only after their
target and ownership are verified; they are never serialized as new DVA configuration fields.

