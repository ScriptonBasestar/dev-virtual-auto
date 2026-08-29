---
id: TASK-240
title: "Public docs contradict bare lifecycle plan selection"
type: docs
priority: P1
effort: S
created-at: 2026-08-29T22:47:35+09:00
source: "post-wave review found README and USAGE still claiming bare up always widens to the whole stack"
scope: "README and USAGE bare lifecycle/default_plan/default_mode guidance, changelog"
status: todo
---

# Task 240: document bare lifecycle selection accurately

## Problem

The CLI selects an explicit `default_plan` or an implicit lone plan for a bare lifecycle command,
refuses an ambiguous multi-plan configuration, and reaches the whole-stack path only when no plans
exist. README and USAGE still contain unconditional whole-stack wording that can make an operator
expect a wider or narrower action than the command actually performs.

## Completion criteria

- [ ] README describes effective-default, ambiguous, and no-plan behavior for all bare lifecycle verbs | verify: `make doc-check`
- [ ] USAGE applies the same contract to lifecycle, status, profile, default_mode, and default_plan guidance | verify: `make doc-check`
- [ ] Public wording remains pinned to the existing CLI selection tests | verify: `go test ./internal/cli -run 'TestRequirePlanSelection|TestUpBareUsesDefaultPlan|TestStatusWithoutPlansKeepsWorkspacePath'`
- [ ] Repository documentation and commit gates pass | verify: `make doc-check && make commit-check`

## Decision

Describe one effective-default rule across `up`, `down`, `stop`, `restart`, `build`, `logs`, and
`status`: explicit `default_plan`, then an implicit lone plan, then refusal when multiple plans are
ambiguous. Whole-stack execution is the compatibility path only when the configuration declares no
plans; bare `status` reports the whole workspace in that no-plan case. Keep Compose profile advice,
but scope it to that whole-stack path rather than presenting it as the default for every project.
