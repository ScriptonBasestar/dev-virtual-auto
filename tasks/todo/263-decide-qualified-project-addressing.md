---
id: TASK-263
title: "Decide qualified-project addressing and exposure"
type: chore
priority: P0
effort: M
exec-tier: strong
created-at: 2026-09-02T11:20:00+09:00
source: "PLAN-003 separation of addressing from composition"
scope: "interaction machine route and shorthand, imported item names, exposure, collision precedence, discovery surfaces, migration, and rollback"
status: todo
needs-human: true
decision-status: pending
depends-on: [TASK-259]
---

# Task 263: decide qualified-project addressing

## Summary

Use TASK-259 evidence to freeze project addressing and exposure independently of cross-project plan
composition. One separator does not need to serve direct interactions, imported items and configuration
references if doing so weakens compatibility or literal-key precedence.

## Recommended direction

Keep `dva run --project <project> <interaction>` as the collision-safe explicit machine route and retain
`project:interaction` as the human shorthand. Keep explicitly imported plan, interaction and provision names
as `project/item`, with aliases only when declared. Do not automatically expose every child item merely because
a subproject is registered.

This mixed grammar reflects different operations: direct child selection versus a parent-owned imported name.
The fail-closed fallback is the exact current grammar and explicit import policy. A `/` unification or automatic
reachability requires separate measured compatibility evidence and must not be smuggled into composition.

## Completion Criteria

- [ ] Use TASK-259's pinned grammar and consumer corpus to decide a canonical explicit route and allowed shorthand separately for direct interactions, imported plans, imported interactions and imported provision profiles | verify: human — every surface must name accepted, rejected and ambiguous examples
- [ ] Freeze literal `:` and `/` key precedence, reserved-prefix rejection, canonical/alias collision handling, missing project behavior, lazy child loading and working-directory selection | verify: human — no parser fallback may silently select a different project or command
- [ ] Decide explicit import/export versus automatic registration; the recommended default is explicit import with no flattening or automatic reachability | verify: human — any broader exposure requires a bounded namespace and compatibility proof
- [ ] Freeze help, completion, ls, show, status and manifest representation, including the collision-safe explicit invocation an agent should use | verify: human — machine discovery must not require guessing whether `:` or `/` applies
- [ ] Define compatibility duration, migration diagnostics and rollback for any change from the current mixed grammar; insufficient evidence selects the current grammar | verify: human — dynamic invocation findings remain unresolved rather than green
- [ ] Obtain independent product and compatibility review, append an approved `## Decision Record`, and change `decision-status` from `pending` to `decided` before TASK-260 begins | verify: `make doc-check`

## Non-goals

- No route, schema, resolver or completion implementation.
- No imported-plan ownership repair; TASK-262 owns it.
- No plan-composition decision or vocabulary rename.
