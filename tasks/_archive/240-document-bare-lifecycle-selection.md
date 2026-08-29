---
id: TASK-240
title: "Public docs contradict bare lifecycle plan selection"
type: docs
priority: P1
effort: S
created-at: 2026-08-29T22:47:35+09:00
source: "post-wave review found README and USAGE still claiming bare up always widens to the whole stack"
scope: "README and USAGE bare lifecycle/default_plan/default_mode guidance, status regression test, changelog"
status: done
completed-at: 2026-08-29T23:02:17+09:00
verification-status: verified
---

# Task 240: document bare lifecycle selection accurately

## Problem

The CLI selects an explicit `default_plan` or an implicit lone plan for a fully argument-free action,
and refuses an ambiguous multi-plan configuration. Without plans, four lifecycle actions use the
whole-stack path while build and logs use a primary-Compose passthrough. Status instead falls back to
the whole workspace whenever no effective default exists. README and USAGE still contain
unconditional whole-stack wording that can make an operator expect a wider or narrower action than
the command actually performs.

## Completion criteria

- [x] README distinguishes argument-free actions, status fallback, and stack-flag/passthrough compatibility paths | verify: `make doc-check`
- [x] USAGE applies the same distinctions to lifecycle, status, profile, default_mode, and default_plan guidance | verify: `make doc-check`
- [x] Public wording is pinned to CLI selection tests, including ambiguous-plan status fallback | verify: `go test ./internal/cli -run 'TestRequirePlanSelection|TestUpBareUsesDefaultPlan|TestStatusWithAmbiguousPlansFallsBackToWorkspace|TestStatusWithoutPlansKeepsWorkspacePath'`
- [x] Repository documentation and commit gates pass | verify: `make doc-check && make commit-check`

## Decision

Describe fully argument-free `up`, `down`, `stop`, `restart`, `build`, and `logs` as selecting an
explicit `default_plan`, then an implicit lone plan, then refusing an ambiguous multi-plan setup.
Without plans, only the first four use whole-stack lifecycle; build and logs use the legacy
primary-Compose passthrough. Status selects the effective default when one exists and otherwise
reports the whole workspace, including for ambiguous multi-plan configurations. Treat stack-path
flags and passthrough arguments as compatibility routes rather than calling them bare invocations.
Do not imply every stack-path flag narrows scope: a default-less multi-plan `up --force` can reach
whole-stack force-recreate. Keep Compose profile advice, but scope it to a fully argument-free,
no-plan whole-stack path.
