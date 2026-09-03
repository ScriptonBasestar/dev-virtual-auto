---
id: TASK-289
title: "Implement the composition plan schema"
type: feature
priority: P1
effort: M
exec-tier: standard
created-at: 2026-09-04T10:00:00+09:00
source: "PLAN-005 implementation of TASK-260's frozen composition contract"
scope: "PlanConfig composes field, config validation, and import-time composition-of-composition rejection"
status: todo
depends-on: [TASK-260]
---

# Task 289: implement the composition plan schema

## Summary

Implement exactly the schema and validation rules TASK-260 §3 froze for composition plans — no new
identity, merge, or cycle rule beyond what that Decision Record already specifies.

## Recommended direction

Add a new, mutually exclusive `composes: []CompositionEntry` field to `PlanConfig`
(`internal/config/config.go:67`) alongside the existing `entries: []PlanEntry`. Name the Go struct
`CompositionEntry`, not `ComposeEntry` — `internal/cli/compose.go` already owns the name `compose` for
the unrelated raw Docker Compose passthrough command, and reusing it as a Go identifier invites reviewer
and reader confusion between two unrelated features (see PLAN-005 "Open questions" §1). The YAML key
itself is `composes:`, exactly as frozen; only the Go-side symbol name is a fresh implementation choice.

`CompositionEntry` carries `Plan string`, `Order int`, `DependsOn []string`, `Vars map[string]string` —
the same fields TASK-260 §3.1's example freezes.

## Completion Criteria

- [ ] `PlanConfig` accepts `composes: []CompositionEntry`; declaring both `entries:` and `composes:` on the same plan is a config validation error (TASK-260 §3.1) | verify: `/usr/bin/grep -Eq '^func TestCompositionPlanEntriesAndComposesAreExclusive\(' internal/config/composition_plan_test.go && go test ./internal/config -count=1`
- [ ] A composition plan cannot declare `environment:`, `site:`, or top-level `vars:` (TASK-260 §3.6); a composition plan cannot appear as its own `composes[].plan` target twice (duplicate inclusion, §3.4); two aliases resolving to the same canonical import in one `composes:` list are also rejected as the same duplicate | verify: `/usr/bin/grep -Eq '^func TestCompositionPlanRejectsSelfConfigAndDuplicates\(' internal/config/composition_plan_test.go && go test ./internal/config -count=1`
- [ ] `composes[].plan` must resolve to a local leaf plan or an already-imported canonical/alias name (`subprojects.<name>.import.plans`); referencing a non-imported `project/plan` name is rejected before any child loads (TASK-260 §3.2) | verify: `/usr/bin/grep -Eq '^func TestCompositionEntryRequiresImportedOrLocalPlan\(' internal/config/composition_plan_test.go && go test ./internal/config -count=1`
- [ ] A composition plan cannot reference another composition plan in `composes[].plan`, whether the target is local or imported; `resolveSubprojectImports` (`internal/config/subproject.go:88`) rejects importing a child's composition plan into a parent for the same reason (TASK-260 §3.3, structural cycle prevention — this is not a graph-traversal cycle detector, it is a flat "composition of composition is rejected" rule) | verify: `/usr/bin/grep -Eq '^func TestCompositionPlanCannotComposeAComposition\(' internal/config/composition_plan_test.go && go test ./internal/config -count=1`
- [ ] The accepted and rejected YAML fixtures in TASK-260 §3 (two-project `depends_on` composition; a composition-of-composition attempt) parse and validate exactly as that Decision Record specifies | verify: `/usr/bin/grep -Eq '^func TestCompositionPlanTaskDecisionFixtures\(' internal/config/composition_plan_test.go && go test ./internal/config -count=1`
- [ ] Repository gates pass | verify: `make lint && make test && make commit-check`

## Non-goals

- No resolver, orchestrator, or CLI changes — those belong to TASK-290, TASK-291, and TASK-292.
- No YAML key rename or new top-level composition vocabulary beyond `composes:`/`CompositionEntry`.
- No entry-override mechanism beyond `CompositionEntry.Vars` (TASK-260 §3.7 already rejects
  `Runner`/`Services`-level overrides from the root).
