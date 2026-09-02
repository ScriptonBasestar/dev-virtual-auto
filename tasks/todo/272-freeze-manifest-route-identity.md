---
id: TASK-272
title: "Freeze the manifest command route-identity representation"
type: chore
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-03T09:00:00+09:00
source: "TASK-254 evidence — the manifest schema cannot express a canonical/compatibility route pair"
scope: "manifest consumers, static_commands subcommand coverage, route-identity representation, schema versioning, and the implementation split across TASK-256 and TASK-258"
status: todo
needs-human: true
decision-status: pending
depends-on: [TASK-254]
---

# Task 268: freeze manifest route identity

## Summary

TASK-254 measured that `dva manifest` cannot describe a command reachable under two names. `static_commands`
is a flat map keyed by one name whose entry carries only `description`, `type`, `options` and `subcommands`
(`internal/cli/manifest.go:105-110`), and only `skill` populates `subcommands`, so the five `config` children
— including the `config validate` route TASK-257 is choosing between — are absent from the document
altogether. Decide how the manifest represents route identity, so TASK-256 and TASK-258 implement an approved
representation instead of inventing one.

## Recommended direction

Separate the coverage defect from the identity question and prefer the smaller answer for each. Publishing
`config`, `ssh` and `console` children through the existing `subcommands` field needs no new schema and fixes
a document that today omits the routes DVA's own guidance teaches. Route identity is the part that needs a
field, and the recommendation is one optional marker naming the canonical invocation on the compatibility
entry — not a parallel route table — so a consumer that ignores it still reads a correct document.

## Completion Criteria

- [ ] Record every tracked consumer of the command manifest and what each reads from `static_commands`; state exactly which facts about a two-name route the current schema can and cannot carry | verify: human — the account must cite tracked paths and the measured manifest, and must distinguish a missing field from a missing entry
- [ ] Compare subcommand-coverage-only, canonical/compatibility fields on the static command entry, an invocation-keyed route list, and no change; state schema-version, legacy-consumer, completion and help consequences for each | verify: human — no option may be selected only because it is the smallest diff
- [ ] Freeze the representation, the `schema_version` policy for it, the meaning legacy fields keep, and which of TASK-256 and TASK-258 may implement which part | verify: human — an implementation task may not extend the representation beyond what is frozen here
- [ ] Append an approved `## Decision Record` to this card and change `decision-status` from `pending` to `decided` before TASK-256 or TASK-258 touches the manifest | verify: `make doc-check`

## Non-goals

- No route, alias, help group, or reserved-name change.
- No decision about which of `ktl`/`kubectl` or `validate`/`config validate` is canonical — that stays with TASK-255 and TASK-257.
- No command registry refactor; TASK-254 recommends keeping current ownership.
