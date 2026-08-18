---
id: TASK-189
title: "The guided path computes its own copy of the config-presence flag"
type: refactor
priority: P3
effort: XS
created-at: 2026-08-18T15:24:47+09:00
source: "4ec336b — 30-configure.yaml has no check_config step to borrow from"
scope: "dva repo — agent-mesh-flows/dva-improve-guided/30-configure.yaml"
status: todo
---

# Task 189: One producer for one fact

## Summary

`dva-improve.yaml` gates its backup steps on `check_config.has_dva_yml`, the flag that step
already publishes. `dva-improve-guided/30-configure.yaml` has no `check_config` step, so it
carries its own `backup_paths.has_config` computing the same thing with the same shell.

Two copies of one fact, in two files, each feeding a `when:` gate. The gate contract has
five rules and the producer must satisfy three of them; satisfying them twice is twice the
surface for a copy to drift. The copies are byte-identical today — both were run and
observed — which is the best a duplicate ever is.

The fix is not obvious enough to be worth forcing. The guided stages are separate flows
with their own parameters, so sharing means either adding a `check_config`-equivalent stage
to the guided pipeline or accepting the duplicate and pinning it with a test. Judge which
is smaller once TASK-186 lands, since a flowcheck rule that reads producers may make the
duplicate cheap to keep honest.

## Completion Criteria

- [ ] The guided path's config-presence flag has one definition, or the duplicate is pinned by a check that fails on drift | verify: human — read `30-configure.yaml` and the pinning test if one was added
- [ ] The gates still hold on both tracks | verify: human — run the stage against a fixture with and without a config; the backup steps run and skip respectively
- [ ] Flow still validates | verify: `am validate agent-mesh-flows/dva-improve-guided/30-configure.yaml`
- [ ] Corpus stays clean | verify: `go run ./tools/flowcheck`

## Technical Notes

- Both copies read `cd '{{param.target}}' && { [ -f 'dva.yml' ] || [ -f 'dva.yaml' ]; } && printf true || printf false`.
- The quoting is load-bearing, not style — see TASK-186.
- Lowest priority of the set. It is a maintainability cost, not a defect; nothing
  misbehaves today.
