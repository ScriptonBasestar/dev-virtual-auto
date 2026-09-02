---
id: TASK-262
title: "Restore imported-plan execution against the owning project"
type: bug
priority: P0
effort: L
exec-tier: strong
created-at: 2026-09-02T11:19:00+09:00
source: "PLAN-003 current-contract audit of subproject plan imports"
scope: "imported plan ownership, resolver inputs, child environment and working directory, lifecycle parity, validation, docs, and regression fixtures"
status: todo
---

# Task 262: restore imported-plan execution ownership

## Summary

Restore the advertised `dva up project/plan` contract. The loader currently clones a child plan into
the parent but not its stack, while the resolver looks up every plan entry in the parent stack. A normal
child plan therefore cannot resolve unless the parent happens to declare matching entry names.

## Recommended direction

Carry an explicit owning-project identity and effective child config into plan resolution. Resolve the
imported plan against the child stack, environments, sites, env files and project directory exactly as a
direct child invocation would, then apply only documented parent CLI overrides. Child lifecycle hooks,
endpoints and readiness definitions belong to that same owner; parent hooks and endpoints do not silently
wrap or replace them. Do not flatten child stack entries into the parent or let same-named parent declarations
satisfy child references accidentally.

Preserve the existing legal subproject location forms: the configured child path may be absolute or may use
`..` to live outside the parent tree. Runner-relative and config-relative assets must still resolve from the
resolved child root. This repair does not introduce a new filesystem-containment policy.

Until this is implemented, the fail-closed fallback is to reject imported plans during validation and remove
claims that they are executable. Silently executing against parent declarations is not an acceptable fallback.

## Completion Criteria

- [ ] Add a real parent/child fixture whose imported plan references a child-only stack entry and prove the pre-fix path fails at `stack_ref`; canonical and explicit alias names must cover the same owner | verify: `/usr/bin/grep -Eq '^func TestImportedPlanResolvesAgainstOwningProject\(' internal/lifecycle/imported_plan_test.go && go test ./internal/config ./internal/lifecycle -count=1`
- [ ] Represent imported-plan ownership explicitly enough that canonical name and alias share one immutable owner config; `SubprojectPath` alone must not permit lookup from the parent stack | verify: `go test ./internal/config ./internal/lifecycle -count=1`
- [ ] Resolve child stack, environment, site, vars, env-file inputs, runner-relative files and default working directory as a direct child invocation would; same-named parent declarations and parent global vars must not leak, while documented CLI `--var` overrides remain explicit | verify: `/usr/bin/grep -Eq '^func TestImportedPlanOwnerIsolation\(' internal/lifecycle/imported_plan_test.go && go test ./internal/lifecycle ./internal/cli -count=1`
- [ ] Canonical and alias invocations preserve lifecycle parity for up, down, stop, restart, status, logs and build, including reverse teardown, signals, text/JSON diagnostics and exit codes | verify: `/usr/bin/grep -Eq '^func TestImportedPlanLifecycleParity\(' internal/cli/imported_plan_lifecycle_test.go && go test ./internal/cli -count=1`
- [ ] Child endpoints, readiness checks and lifecycle before/replace/after hooks are used exactly as in direct child execution; same-named parent hooks and endpoints do not leak into a standalone imported-plan invocation | verify: `/usr/bin/grep -Eq '^func TestImportedPlanUsesOwnerHooksAndEndpoints\(' internal/cli/imported_plan_lifecycle_test.go && go test ./internal/cli -count=1`
- [ ] Missing child config, missing owner stack entry, ambiguous owner and import collision fail before any external child starts; existing manifest output remains backward compatible and canonical/alias entries do not leak local absolute paths, while any new public owner field is deferred to the conditional manifest-contract child | verify: `go test ./internal/config ./internal/lifecycle ./internal/cli -count=1`
- [ ] Absolute and parent-relative (`../`) subproject locations remain accepted, and runner/config-relative assets resolve from the resulting child root without adding a new containment restriction | verify: `/usr/bin/grep -Eq '^func TestImportedPlanExternalChildRoot\(' internal/lifecycle/imported_plan_test.go && go test ./internal/config ./internal/lifecycle -count=1`
- [ ] User and architecture documentation describe the verified owner and working-directory contract and no longer advertise behavior not covered by the fixture | verify: `make doc-check`
- [ ] Repository gates pass | verify: `make lint && make test && make test-integration && make commit-check`

## Non-goals

- No automatic project registration or separator change.
- No cross-project plan composition or recursive plan include.
- No parent ownership of child-native resources.
- No manifest schema extension or new filesystem-containment policy.
