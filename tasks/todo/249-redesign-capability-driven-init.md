---
id: TASK-249
title: "Redesign init around verified capabilities instead of a fixed three-plan template"
type: chore
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-01T19:25:00+09:00
source: "PLAN-002 tracked D8-compatible scaffold ruling"
scope: "init discovery contract, capability preset integration, plan naming/default rules, human-agent parity, census ownership"
status: todo
needs-human: true
decision-status: pending
---

# Task 249: redesign capability-driven init

## Summary

Replace the rejected fixed three-plan template with an evidence-driven generation contract aligned
with D8 and the repository's capability preset policy.

## Decision required

The proposed unconditional `local-infra`, `local-dev`, and `full-stack` scaffold conflicts with D8,
contains invalid top-level fields, and asks the current Compose-only detector to invent a native runner.
It can also generate empty or duplicate plans that immediately trigger D6.

## Recommended direction

검증된 provider closure 하나에서 plan 하나를 생성하는 보수적 기본값을 권장한다. 단일 plan은 기존 bare
lifecycle의 implicit default를 사용하고, 실제 evidence가 둘 이상의 독립 plan을 정당화할 때만 명시적
`default_plan`을 기록한다. 이름은 기존 사용자 선언을 보존하고, 새 이름은 entry/provider identity에서
기계적으로 도출한다. 충돌하거나 불완전한 discovery에서는 preview만 제공하고 파일을 쓰지 않는다.

현재 다섯 template과 `config init`/top-level `init` surface는 유지하되 모두 하나의 generator를 호출하게
한다. Corpus 빈도는 detector 개선의 입력으로만 사용하고 새로운 archetype이나 plan label의 근거로
사용하지 않는다.

## Completion Criteria

- [ ] Define expected discovery evidence and generated output for compose-only, native-only, hybrid, and no-discovery fixtures | verify: human — each fixture must list detected facts, unverified facts, generated entries/plans, and explicit omissions
- [ ] Reuse the repository's capability-driven preset policy: generate only self-contained closure from verified providers and omit plans that lack evidence | verify: human — no output may depend on a role label inferred only by a person
- [ ] Keep `local-infra`, `local-dev`, and `full-stack` out of generated defaults unless a future tracked decision explicitly reopens D8; preserve an existing user-declared name, otherwise derive a name mechanically from verified entry/provider identity or require explicit user choice | verify: human — the decision must contain no generator-authored exception for the three rejected labels
- [ ] Decide single-plan implicit default versus explicit `default_plan`, and prove generated multi-plan output never lacks a default | verify: human — selection must align with bare lifecycle behavior
- [ ] Define no-overwrite, preview/dry-run, idempotence, and invalid partial-discovery behavior | verify: human — mutation must not begin from unresolved or conflicting evidence
- [ ] Make human `init` and agent workflows consume one canonical generator/preset rather than copied templates | verify: human — ownership and generation direction must be named
- [ ] Freeze a backward-compatibility matrix for `minimal`, `rails`, `node`, `python`, `go`, `--recursive`, `--devcontainer`, `--all`, `config init`, and the visible top-level `init` alias; every surface must be preserved, explicitly deprecated, or deliberately removed with migration evidence | verify: human — exact argv/help/output expectations are required
- [ ] Define census owner, canonical repository IDs/revisions, input inventory, cadence, and the change threshold that can revise defaults | verify: human — a bare count without revision is insufficient
- [ ] Record the selected contract and alternatives rejected in this card | verify: `make doc-check`

## Rejected baseline

Do not generate a fixed three-plan template merely because those names are common in the measured
corpus. Frequency is input evidence, not proof that a particular repository has those capabilities.
Capability evidence can justify a plan's existence, but does not by itself justify one of the three
rejected labels.
