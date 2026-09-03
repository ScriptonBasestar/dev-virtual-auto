---
id: TASK-288
title: "Decide how service-orchestration.yml should model compose overlays"
type: chore
priority: P3
effort: S
exec-tier: standard
created-at: 2026-09-03T21:30:00+09:00
source: "TASK-276 ruling 2026-09-03 — the one strict warning that is not compose absence"
scope: "examples/service-orchestration.yml semantic overlay warning, and whether the warning or the example is wrong"
status: todo
needs-human: true
decision-status: pending
depends-on: []
---

# Task 288: model compose overlays in the orchestration example

## Summary

`examples/service-orchestration.yml` draws one `semantic:` warning from `dva config validate
--strict` saying its four compose entries "can run in the same invocation set", so an overlay
entry cannot patch another entry's services. It is the only strict warning left in the tracked
corpus that is not the documented compose-absence pair, and it was split out of
[TASK-276](276-correct-example-corpus-and-close-md-yaml-gap.md) because it is a different
defect with a different owner.

## Problem

The mechanical fix the warning suggests — merge `infra-compose` and `frontend` into one entry —
collapses `frontend`'s `order: 40` and `depends_on: [api]` into `infra-compose`'s `order: 10`.
That destroys the ordered startup the example exists to demonstrate. Applying the suggestion
would make the validator quiet and the example wrong, which is strictly worse than the current
state.

So the warning and the example disagree, and **which one is wrong is not yet established**.
Three readings are live, and they lead to different work:

1. **The example is wrong.** Overlays genuinely cannot work this way, and the example teaches a
   shape DVA does not support. Fix: rewrite the example to model ordered startup without
   relying on same-file overlay entries.
2. **The warning is wrong.** Multiple entries pointing at one compose file with distinct
   `order:`/`depends_on:` is a legitimate shape, and the warning's "same invocation set"
   predicate is too coarse — it does not account for entries that are separated by ordering.
   Fix: narrow the predicate.
3. **Both are right and the message is wrong.** The shape is legitimate but genuinely does
   prevent overlay patching, and the warning is correct to fire while its suggested remedy
   (merge) is unsafe. Fix: keep the warning, change what it advises.

## Decision required

Which of the three readings holds. Answering it requires reading the overlay/invocation-set
logic in `internal/lifecycle/` and `internal/config/validate_warnings.go` against what the
example is trying to teach, and deciding whether DVA *intends* to support ordered entries over
a shared compose file.

## Recommended direction

Reading 3 is the most likely and the cheapest to verify first: the warning's diagnosis is
probably accurate and only its remedy is unsafe. Confirm by checking whether the invocation-set
grouping actually ignores `order:`; if it does, the shape really is unsupported for overlay
purposes and reading 1 or 2 follows depending on whether that is intended. Do not apply the
suggested merge under any reading — it is the one action already known to be wrong.

## Completion Criteria

- [ ] Establish which of the three readings holds, from the overlay/invocation-set source rather than from the warning text | verify: human — the reading, the source citations that establish it, and the two rejected readings must be recorded
- [ ] Apply the fix the chosen reading implies, without merging the entries | verify: human — the ordered startup the example demonstrates (`order:` and `depends_on:` on `frontend`) must survive the change
- [ ] `examples/service-orchestration.yml` draws no strict warning other than the documented compose-absence pair | verify: `make test && make doc-check`

## Non-goals

- No merge of `infra-compose` and `frontend`. Recorded as known-wrong above.
- No change to the compose-absence warnings — those are TASK-276's, and its ruling exempts them
  for the corpus while keeping them for real projects.
