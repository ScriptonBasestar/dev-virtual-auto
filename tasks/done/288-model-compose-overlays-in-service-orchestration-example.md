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
status: done
needs-human: true
decision-status: decided
decided-at: 2026-09-03T22:40:00+09:00
closed-at: 2026-09-03T22:40:00+09:00
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

## Recommended direction (original, superseded by the Decision below)

Reading 3 is the most likely and the cheapest to verify first: the warning's diagnosis is
probably accurate and only its remedy is unsafe. Confirm by checking whether the invocation-set
grouping actually ignores `order:`; if it does, the shape really is unsupported for overlay
purposes and reading 1 or 2 follows depending on whether that is intended. Do not apply the
suggested merge under any reading — it is the one action already known to be wrong.

This turned out to be wrong. It was written from the warning's own text before measuring the
warning's actual predicate against the runtime code path; see the Decision below.

## Decision (frozen 2026-09-03)

**Reading 2 holds: the warning is wrong.** `warnMultiStackComposeSplit`'s "same invocation set"
predicate is too coarse — it groups compose entries sharing one literal `compose.project_name`
and warns unless `composeEntriesAreIsolated` proves the entries never co-occur, but "never
co-occur" was the only escape hatch it recognized. It had no way to recognize entries that *do*
co-occur but never conflict because each restricts itself to a disjoint `services:` subset —
`infra-compose`/`frontend`'s actual shape.

### Why reading 3 (this card's own original recommendation) does not hold

Reading 3 says the diagnosis is accurate and only the remedy (merge) is unsafe. Measuring the
runtime code path shows the diagnosis itself is the false positive, not just its remedy:

- `plans.local-full.entries[].services` (`PlanEntry.Services`, `internal/config/config.go:100`)
  is a real runtime restriction, not decorative: `internal/lifecycle/resolver.go:234` copies it
  into `ResolvedEntry.Services`, and `internal/lifecycle/plan_orchestrator.go:37-38` uses it to
  restrict which services a compose invocation actually starts
  (`if resolved.Runner == "compose" && resolved.Services != nil { composeServices[...] = ... }`).
- So `infra-compose` and `frontend` do not "patch" each other — they issue two separate
  `docker compose ... up postgres kafka` / `docker compose ... up frontend` invocations against
  the same file, starting disjoint services. Nothing is overlaid; the overlay-patch concern the
  warning exists for (one entry's `files:` list patching another's service definitions in a
  shared invocation) does not apply to this shape at all.
- `order:`/`depends_on:` (on the *plan entry*, not the stack entry — the card's Problem section
  misattributed them) are exactly what already sequences `infra-compose` before `frontend`; no
  merge or reordering was ever needed to preserve that.
- `plansIsolateEntries`/`plansWouldIsolateEntries` (pre-existing, unmodified) answer "does at
  most one of these entries ever run" via mutual exclusion between plans. They correctly return
  false here, because `local-full` selects both entries together — but mutual exclusion was
  never the right question for this shape; disjoint services under simultaneous invocation is a
  different, equally-safe shape that isolation-by-exclusion cannot express.

### Fix applied

Narrowed the predicate. `composeEntriesAreIsolated` (`internal/config/validate_warnings.go:798`)
now also treats a set of compose entries as isolated when a new `plansPartitionComposeServices`
holds: in every plan that selects two or more of the entries together *as compose invocations*
(a plan entry resolving to a non-compose runner — `api`/`worker`'s `runner: native` in this
example — is excluded from the count via a new `planEntryRunner` helper), every one of those
entries declares a non-empty `services:` and no service name repeats across them. An empty
`services:` (meaning "all services") or a repeated service name still falls through to the
original overlay warning, and so does any entry never claimed by any plan — both retain the
warning's original behavior. `plansPartitionComposeServices` is gated on `DefaultPlan() != ""`
exactly like `plansIsolateEntries`, for the same reason: partitioning inside every named plan
says nothing about the unnamed `dva up` lifecycle path.

Verified behaviorally: `make build` + `dva config validate --strict` against
`examples/service-orchestration.yml` now draws only the two documented compose-absence
config-drift warnings (TASK-276's territory, untouched); the overlay-split warning is gone.
Verified not to over-widen: new table-driven unit tests
(`TestWarnMultiStackComposeSplitServiceSubsetting`,
`internal/config/validate_warnings_test.go`) cover disjoint services (silent), a mixed
compose/native plan (silent), empty services (still warns), and overlapping services (still
warns); all four pre-existing `TestWarnMultiStackComposeSplit*` tests still pass unmodified.

### Rejected alternatives

- **Reading 1** (rewrite the example — overlays genuinely unsupported): rejected by the same
  runtime evidence above; `services:` subsetting is a supported, runtime-consumed shape (not a
  fiction the example invented), so there is nothing to design around.
- **Reading 3** (keep the warning, fix only its remedy): rejected — measurement shows the
  *diagnosis*, not only the remedy, is wrong on this shape. Fixing only the remedy text would
  leave a false-positive warning firing on a documented, supported configuration.

## Completion Criteria

- [x] Establish which of the three readings holds, from the overlay/invocation-set source rather than from the warning text | verify: human — the reading, the source citations that establish it, and the two rejected readings must be recorded
- [x] Apply the fix the chosen reading implies, without merging the entries | verify: human — the ordered startup the example demonstrates (`order:` and `depends_on:` on `frontend`) must survive the change
- [x] `examples/service-orchestration.yml` draws no strict warning other than the documented compose-absence pair | verify: `make test && make doc-check`

## Non-goals

- No merge of `infra-compose` and `frontend`. Recorded as known-wrong above.
- No change to the compose-absence warnings — those are TASK-276's, and its ruling exempts them
  for the corpus while keeping them for real projects.
