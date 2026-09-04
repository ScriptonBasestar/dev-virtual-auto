---
id: TASK-290
title: "Implement the composition plan resolver"
type: feature
priority: P1
effort: M
exec-tier: standard
created-at: 2026-09-04T10:00:00+09:00
source: "PLAN-005 implementation of TASK-260's frozen composition contract"
scope: "resolving composition plans into wave-ordered child ExecutionPlans, default-plan selection, and resolved-plan immutability"
status: todo
depends-on: [TASK-289]
---

# Task 290: implement the composition plan resolver

## Summary

Resolve a `composes:` plan into an ordered, wave-assigned list of child `ExecutionPlan`s, reusing the
existing `CalculateWaves` algorithm (`internal/lifecycle/resolver.go:291`) rather than inventing a second
DAG/ordering implementation.

## Recommended direction

Introduce a resolved composition type (name it `CompositionPlan`, mirroring `ExecutionPlan` — not a new
field on `ExecutionPlan` itself, since a composition plan resolves to *multiple* child `ExecutionPlan`s,
each fully resolved against its own owner as TASK-262 already established) whose entries are
`{ChildPlan *ExecutionPlan, Wave int, Order int, DependsOn []string}`. Feed `CompositionEntry.Order` and
`.DependsOn` into `CalculateWaves` exactly as `PlanEntry.Order`/`.DependsOn` are today — the wave
algorithm does not need to know whether it is ordering stack entries or child plans.

Each child's `ExecutionPlan` is resolved with its owning config exactly as a direct or imported-plan
invocation resolves today (TASK-262's contract, unchanged) — the composition resolver does not pass the
root's environment, site, or vars into that resolution; it only applies `CompositionEntry.Vars` as a
documented override, the same mechanism `PlanEntry.Vars` already uses.

## Completion Criteria

- [x] A `CompositionPlan` type resolves a `composes:` plan's entries into wave-numbered children using the existing `CalculateWaves`, without a second ordering algorithm | verify: `/usr/bin/grep -Eq '^func TestCompositionPlanCalculatesWavesFromCalculateWaves\(' internal/lifecycle/composition_resolver_test.go && go test ./internal/lifecycle -count=1`
- [x] Each resolved child `ExecutionPlan` uses its owning project's effective config (environment, site, env_file, vars, hooks, endpoints, readiness) exactly as TASK-262 already guarantees for a direct or imported-plan invocation; only `CompositionEntry.Vars` may override, and it overrides rather than merges (TASK-260 §3.6) | verify: `/usr/bin/grep -Eq '^func TestCompositionChildResolvesAgainstOwnConfig\(' internal/lifecycle/composition_resolver_test.go && go test ./internal/lifecycle -count=1`
- [x] `default_plan` and the "exactly one declared plan" auto-selection rule treat a composition plan identically to a leaf plan — no special-casing (TASK-260 §3.5) | verify: `/usr/bin/grep -Eq '^func TestCompositionPlanParticipatesInDefaultPlanSelection\(' internal/lifecycle/composition_resolver_test.go && go test ./internal/lifecycle -count=1` — note: the "implicit single plan" auto-selection case is structurally unreachable for a composition plan (composing at least one other plan means that plan is also always present in `cfg.Plans`, so a composition plan can never be the sole entry); the test instead proves no special-casing both ways — explicit `default_plan` naming a composition plan is honored, and multiple declared plans (leaf + composition) without an explicit `default_plan` are refused exactly as multiple leaf plans would be.
- [x] A resolved `CompositionPlan` (and its resolved children) is immutable for the remainder of a single invocation — a config reload mid-run does not change an already-resolved wave assignment or child owner (TASK-260 §3.9, mirrors existing `ExecutionPlan` immutability) | verify: `/usr/bin/grep -Eq '^func TestCompositionPlanResolutionIsImmutablePerInvocation\(' internal/lifecycle/composition_resolver_test.go && go test ./internal/lifecycle -count=1`
- [x] Repository gates pass | verify: `make lint && make test && make commit-check`

## Non-goals

- No orchestrator execution, rollback, or CLI wiring — those belong to TASK-291 and TASK-292.
- No change to `CalculateWaves` itself or to single-project resolution behavior.
- No new identity or merge rule beyond what TASK-289's schema and TASK-260 §3 already froze.
