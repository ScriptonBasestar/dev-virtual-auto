---
id: TASK-259
title: "Discover qualified project addressing"
type: chore
priority: P0
effort: L
exec-tier: strong
created-at: 2026-09-02T10:11:00+09:00
source: "PLAN-003 cross-project discovery"
scope: "current routing grammar, namespace identity, reachability options, corpus evidence, and decision dossier"
status: todo
---

# Task 259: discover qualified project addressing

## Summary

Produce a self-contained evidence dossier for addressing a root project and imported projects without
changing routing or schema. The investigation must reconcile current direct project selection (`:` and
`--project`) with `/` canonical names used by imported items and literal user keys.

## Completion Criteria

- [ ] Specify the current grammar and precedence for root routes, `project:item`, `--project`, imported `project/item` names, literal keys containing `:` or `/`, aliases, reserved prefixes, working-directory rebasing, lazy loading, and missing or ambiguous project errors | verify: human — every claim must cite current tests and exact tracked symbols
- [ ] Inventory how `run`, lifecycle verbs, provision, show, status, ls, manifest, help, and completion expose or omit project qualification and reachability | verify: `go test ./internal/config ./internal/cli -count=1`
- [ ] Build a pinned, secret-free corpus of real qualified invocations and project layouts with canonical repository IDs, revisions, path inventory, collision findings, and dynamic-invocation limitations | verify: human — local absolute paths and unpinned repositories are not acceptable evidence
- [ ] Compare retaining mixed grammar, making `/` canonical with an explicit compatibility period, and making `--project` the canonical explicit route; separately compare import/export reachability with automatic registration | verify: human — options must cover ambiguity, backwards compatibility, shell ergonomics, completion, machine discovery, and rollback
- [ ] Append a canonical `## Evidence and Recommendation` section to this card with the decision-ready recommendation and rejected alternatives for TASK-260 without registering a new route, changing schema, or promising automatic reachability | verify: `make doc-check`

## Non-goals

- No `/` grammar, alias, project registry, or auto-import implementation.
- No cross-project plan composition implementation.
- No vocabulary rename.
