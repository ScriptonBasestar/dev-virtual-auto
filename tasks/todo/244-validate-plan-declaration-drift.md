---
id: TASK-244
title: "Validate duplicate plan declarations and missing multi-plan defaults"
type: feature
priority: P1
effort: M
exec-tier: standard
created-at: 2026-09-01T19:20:00+09:00
source: "PLAN-002 self-contained D6/D7 contract"
scope: "semantic validation warnings, stable text/JSON output, strict behavior, tests, current reserved-command documentation"
status: todo
---

# Task 244: detect plan declaration drift

## Summary

Add the two frozen plan diagnostics below without depending on ignored reports, claiming full runtime
equivalence, or changing default validation exit behavior.

## Problem

Repositories can carry two plan names whose agreed declaration fields are equal, or multiple plans
without an explicit default. The runtime catches the second case only when a bare lifecycle command
is attempted, and neither condition has a general semantic warning.

The fingerprint is deliberately narrower than a resolved execution plan. It compares plan
`environment`, `site`, `vars`, `endpoint_tags` and entry `name`, `runner`, `order`, `services`,
`depends_on`, `vars`. Map keys are sorted, list order is preserved, and nil/empty collections are
normalized as equal. The warning must describe equal declaration fields, not claim that site overrides
and every runtime input are identical.

## Completion Criteria

- [ ] D6 compares exactly the plan and entry fields listed in this card, sorts map keys, preserves list order, treats nil/empty collections as equal, emits each unordered pair once, and never recommends a canonical name | verify: human — focused tests must name every compared field and pin deterministic pair ordering
- [ ] D6 fixtures cover equal declarations, every one-field difference, map-order-only, list-order difference, nil/empty equality, and subproject namespaces; compare only within one owning config/`SubprojectPath` partition, exclude canonical/import-alias keys that reference the same plan pointer, and do not compare root↔child or child-A↔child-B lookalikes | verify: `go test ./internal/config -count=1`
- [ ] D7 warns only for two-or-more plans without `default_plan`, excludes the single-plan implicit default contract, and does not duplicate the compose-split-specific remedy | verify: `go test ./internal/config -count=1`
- [ ] Default text/JSON validation remains non-fatal while `--strict` promotes both new warnings to the existing non-zero contract; output order and category are stable | verify: `go test ./internal/cli -count=1`
- [ ] Current-state sources report the 24-command set including `skill`: `docs/43` current status, `USAGE.md`, `skills/dva-config/references/schema-reference.md`, its generated library projection, and `docs/51-flowcheck-rules.md`; historical 27→23 transition text in `docs/43` and `CHANGELOG.md` remains historical and separately notes the later addition where needed | verify: `make generate && make check-generate && make doc-check`
- [ ] Repository gates pass | verify: `make lint && make test && make test-integration && make commit-check`

## Non-goals

- No automatic deletion, rename, alias, or canonical-plan recommendation.
- No cross-repository plan-name policy.
- No change to bare lifecycle runtime selection.
