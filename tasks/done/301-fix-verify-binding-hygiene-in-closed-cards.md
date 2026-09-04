---
id: TASK-301
title: "Fix weak/vacuous verify: bindings and overclaiming prose in TASK-294/295/296"
type: docs
priority: P3
effort: S
exec-tier: standard
created-at: 2026-09-04T00:00:00+09:00
source: "Independent review of TASK-294/295/296 completion criteria (reviewer findings across three closed cards)"
scope: "tasks/done/294-fix-restart-stale-pid-false-success.md, tasks/done/295-fix-orchestrator-down-swallows-entry-failures.md, tasks/done/296-fix-composition-readiness-gate-rollback-gap.md — the verify: bindings and surrounding prose text only"
status: done
depends-on: []
---

# Task 301: fix weak/vacuous verify: bindings and overclaiming prose in TASK-294/295/296

## Summary

Three closed task cards were independently reviewed after integration. In all three cases the
underlying completion criterion is substantively satisfied — confirmed by reading the actual
current test code — but the specific `verify:` command written in the card either does not test
what the criterion text claims, or the prose around it overclaims relative to what the bound test
actually does. These are pure documentation defects in already-closed cards: no behavior, code, or
decided-scope changes are implied. This card bundles all three as one small backlog cleanup.

### 1. TASK-294, completion criterion 3 — binding doesn't test what the criterion claims

`tasks/done/294-fix-restart-stale-pid-false-success.md` criterion 3 claims: "No change to
`Down`/`removeProcess`/`Stop`'s existing preserved-PID-file semantics for the non-restart stop case
(`dva stop <plan>` alone must still leave the PID file in place per the existing 'Vagrant halt
semantics' comment)", bound to:

```
verify: go test ./internal/lifecycle -run TestProcessPlugin_StopProcess_NoPidFile -count=1 -v
```

Reading `internal/lifecycle/process_test.go:137-151`, `TestProcessPlugin_StopProcess_NoPidFile`
actually calls `p.Down(...)`, not `Stop`, and only asserts that tearing down a directory with no PID
file present does not error. It never starts a process, never calls `Stop`, and never asserts that a
PID file is preserved afterward — it does not verify the PID-file-preservation semantics the
criterion text describes. (Reviewer noted this mismatch predates TASK-294 — the test already existed
with this shape and behavior before TASK-294's changes; TASK-294 did not introduce it.)

### 2. TASK-296, completion criteria 2 and 3 — a vacuous grep and an under-matching -run regex

`tasks/done/296-fix-composition-readiness-gate-rollback-gap.md` criterion 3 claims: "The `readyErr`
hook on `fakeChildExecutor` (composition_orchestrator_test.go:29) is exercised by at least one test,
closing the gap noted above", bound to:

```
verify: grep -Eq 'readyErr\[' internal/lifecycle/composition_orchestrator_test.go
```

Reading `internal/lifecycle/composition_orchestrator_test.go:87`, the only match for this pattern is
`return f.readyErr[child.Name]` — the fake executor's own field access inside its `WaitReady`
implementation, not a test setting the hook. The test that actually exercises it,
`TestCompositionReadinessFailureRollsBackSucceededSiblings`, lives in the new sibling file
`internal/lifecycle/composition_orchestrator_readiness_test.go` (the card's own criterion 1 note
explains why it landed in a separate file — the 600-line file-size hook), which this binding never
references. The bound grep passes regardless of whether the gap was actually closed.

Criterion 2 claims: "The existing `exec.Up`-failure rollback behavior is unchanged — a child whose
`up` itself failed is still not treated as a rollback target ..., confirmed by the pre-existing
up-failure rollback tests in `composition_orchestrator_test.go` continuing to pass unmodified", bound
to:

```
verify: go test ./internal/lifecycle -run TestCompositionRollback -count=1
```

Reading `internal/lifecycle/composition_orchestrator_test.go`, the pre-existing up-failure rollback
tests are `TestCompositionUpRollsBackSucceededChildrenOnFailure` (line 260) and
`TestCompositionRollbackFailurePreservesOriginalError` (line 333). The `-run TestCompositionRollback`
regex only matches the second name (by prefix) — `TestCompositionUpRollsBackSucceededChildrenOnFailure`
does not start with `TestCompositionRollback`, so it is silently excluded from this binding's run.

### 3. TASK-295, completion criterion 3 — prose overclaims relative to the bound test

`tasks/done/295-fix-orchestrator-down-swallows-entry-failures.md` criterion 3 claims: "`dva down
<plan>` exits non-zero when a real entry teardown fails, verified against the real binary, not just
unit-level", bound to:

```
verify: go test ./internal/cli -run TestPlanDown -count=1 -v
```

The binding itself is fine — it does exercise real teardown-failure behavior. But
`TestPlanDownExitsNonZeroOnEntryDownFailure` (`internal/cli/plan_resolution_test.go:167-189`) calls
`runPlanDown(c, planEnv(e), "demo", nil)` directly, in-process, matching how the card's own
"Completion evidence" section later and correctly describes it ("calls `runPlanDown` directly ...
rather than spawning a subprocess"). That is unit-level, not "against the real binary" as criterion
3's prose claims. The prose and the card's own completion evidence disagree with each other.

## Completion Criteria

- [x] TASK-294 criterion 3's `verify:` binding is corrected to actually test PID-file preservation
      after `Stop` (e.g. point it at a test that calls `Stop` on a running process and asserts the
      PID file still exists afterward), or the criterion text is edited to accurately describe what
      `TestProcessPlugin_StopProcess_NoPidFile` covers instead | verify: `grep -Fq 'TestProcessPlugin_StopProcess_NoPidFile' tasks/done/294-fix-restart-stale-pid-false-success.md && ! grep -Fq 'go test ./internal/lifecycle -run TestProcessPlugin_StopProcess_NoPidFile -count=1 -v' tasks/done/294-fix-restart-stale-pid-false-success.md`
- [x] TASK-296 criterion 3's `verify:` binding is corrected to reference the actual test that sets
      `readyErr` (`TestCompositionReadinessFailureRollsBackSucceededSiblings` in
      `internal/lifecycle/composition_orchestrator_readiness_test.go`) instead of a grep that only
      matches the fake's own field access | verify: `grep -Fq 'composition_orchestrator_readiness_test.go' tasks/done/296-fix-composition-readiness-gate-rollback-gap.md`
- [x] TASK-296 criterion 2's `verify:` `-run` regex is widened to match both pre-existing up-failure
      rollback tests by name (`TestCompositionUpRollsBackSucceededChildrenOnFailure` and
      `TestCompositionRollbackFailurePreservesOriginalError`) | verify: `grep -Fq 'TestCompositionUpRollsBackSucceededChildrenOnFailure' tasks/done/296-fix-composition-readiness-gate-rollback-gap.md`
- [x] TASK-295 criterion 3's prose is corrected to stop claiming real-binary verification, describing
      it instead as in-process verification via `runPlanDown` (consistent with the card's own
      "Completion evidence" section) | verify: `grep -Fq 'in-process' tasks/done/295-fix-orchestrator-down-swallows-entry-failures.md && ! grep -Fq 'verified against the real binary, not just' tasks/done/295-fix-orchestrator-down-swallows-entry-failures.md`
- [x] Repository doc gates pass | verify: `make doc-check`

## Completion evidence

All four edits applied directly to the three closed cards (doc-only, no implementation files
touched, no decided scope reopened):

- TASK-294's closed card, criterion 3: rewrote the criterion text to describe what
  `TestProcessPlugin_StopProcess_NoPidFile` actually covers (`Down`'s no-PID-file tolerance) and
  explicitly notes PID-file preservation after `Stop` remains uncovered by an automated test;
  dropped the old binding's `-count=1 -v` command in favor of a plain existence-check grep.
- TASK-296's closed card, criterion 3: repointed the binding to
  `TestCompositionReadinessFailureRollsBackSucceededSiblings` in
  `composition_orchestrator_readiness_test.go`.
- TASK-296's closed card, criterion 2: widened `-run` to
  `'TestCompositionRollback|TestCompositionUpRollsBackSucceededChildrenOnFailure'`.
- TASK-295's closed card, criterion 3: reworded prose from "verified against the real binary, not
  just unit-level" to "verified in-process via `runPlanDown` (not a spawned subprocess against the
  compiled binary)", matching the card's own Completion evidence section.

All four TASK-301 verify: bindings for these criteria confirmed passing before commit. `make
doc-check` clean.

## Non-goals

- Does not reopen or change any decided scope, behavior, or implementation from TASK-294, TASK-295,
  or TASK-296 — those cards' underlying fixes are correct and already integrated.
- Does not touch any implementation file — this card is scoped to editing text inside the three named
  closed task cards only.
- Does not require writing new tests. If closing TASK-294's gap needs a new/adjusted test to have
  something correct to bind to, that is an acceptable option for a future implementer, but this card
  itself only requires the binding and/or prose text to become accurate — not new test coverage.
