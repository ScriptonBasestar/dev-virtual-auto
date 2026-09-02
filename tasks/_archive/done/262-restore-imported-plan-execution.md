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
status: done
completed-at: 2026-09-02T12:12:00+09:00
completion-summary: "Imported plans now resolve and execute entirely against their owning child config, environment, hooks, endpoints, readiness checks, and working directory."
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "go test ./internal/config ./internal/lifecycle ./internal/cli -count=1"
    result: "passed, including canonical/alias owner isolation and lifecycle parity fixtures"
  - kind: automated
    command-or-step: "dva lint && dva test && dva test integration"
    result: "passed; vet/format/static analysis, race/coverage suite, and integration suite all exited zero"
  - kind: automated
    command-or-step: "make doc-check && make commit-check"
    result: "passed; 612 links, document limits, CI labels, flow decision paths, and commit subjects verified"
quality-review: pass
quality-reviewed-at: 2026-09-02T12:12:00+09:00
quality-review-evidence:
  - "independent strong-tier review passed after fixes for hook routing, env snapshot reuse, readiness context, pre-hook validation, and LoadSubprojects compatibility"
  - "git diff --check and focused config/lifecycle/CLI regression suites passed"
archived-at: 2026-09-02T12:12:00+09:00
verified-at: 2026-09-02T12:12:00+09:00
verification-summary: "Imported plans and aliases execute with one child-owned config/environment contract across every lifecycle surface, with no parent declaration or local-path leakage."
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

- [x] Add a real parent/child fixture whose imported plan references a child-only stack entry and prove the pre-fix path fails at `stack_ref`; canonical and explicit alias names must cover the same owner | verify: `/usr/bin/grep -Eq '^func TestImportedPlanResolvesAgainstOwningProject\(' internal/lifecycle/imported_plan_test.go && go test ./internal/config ./internal/lifecycle -count=1`
- [x] Represent imported-plan ownership explicitly enough that canonical name and alias share one immutable owner config; `SubprojectPath` alone must not permit lookup from the parent stack | verify: `go test ./internal/config ./internal/lifecycle -count=1`
- [x] Resolve child stack, environment, site, vars, env-file inputs, runner-relative files and default working directory as a direct child invocation would; same-named parent declarations and parent global vars must not leak, while documented CLI `--var` overrides remain explicit | verify: `/usr/bin/grep -Eq '^func TestImportedPlanOwnerIsolation\(' internal/lifecycle/imported_plan_test.go && go test ./internal/lifecycle ./internal/cli -count=1`
- [x] Canonical and alias invocations preserve lifecycle parity for up, down, stop, restart, status, logs and build, including reverse teardown, signals, text/JSON diagnostics and exit codes | verify: `/usr/bin/grep -Eq '^func TestImportedPlanLifecycleParity\(' internal/cli/imported_plan_lifecycle_test.go && go test ./internal/cli -count=1`
- [x] Child endpoints, readiness checks and lifecycle before/replace/after hooks are used exactly as in direct child execution; same-named parent hooks and endpoints do not leak into a standalone imported-plan invocation | verify: `/usr/bin/grep -Eq '^func TestImportedPlanUsesOwnerHooksAndEndpoints\(' internal/cli/imported_plan_lifecycle_test.go && go test ./internal/cli -count=1`
- [x] Missing child config, missing owner stack entry, ambiguous owner and import collision fail before any external child starts; existing manifest output remains backward compatible and canonical/alias entries do not leak local absolute paths, while any new public owner field is deferred to the conditional manifest-contract child | verify: `go test ./internal/config ./internal/lifecycle ./internal/cli -count=1`
- [x] Absolute and parent-relative (`../`) subproject locations remain accepted, and runner/config-relative assets resolve from the resulting child root without adding a new containment restriction | verify: `/usr/bin/grep -Eq '^func TestImportedPlanExternalChildRoot\(' internal/lifecycle/imported_plan_test.go && go test ./internal/config ./internal/lifecycle -count=1`
- [x] User and architecture documentation describe the verified owner and working-directory contract and no longer advertise behavior not covered by the fixture | verify: `make doc-check`
- [x] Repository gates pass | verify: `dva lint && dva test && dva test integration && make commit-check`

## Implementation record

- Imported plan clones carry a private owner config shared by canonical and alias names; no owner data is serialized into manifest output.
- Resolver, lifecycle, logs, build, status, hooks, endpoints and readiness checks use the same effective child owner and child-rooted environment.
- Hook route normalization matches each built-in, invalid plans fail before hooks, and the built-in reuses the hook environment snapshot.
- Root/whole-stack health checks retain their invocation working directory; imported plans use the child root, including absolute and `../` subproject paths.

## Non-goals

- No automatic project registration or separator change.
- No cross-project plan composition or recursive plan include.
- No parent ownership of child-native resources.
- No manifest schema extension or new filesystem-containment policy.
