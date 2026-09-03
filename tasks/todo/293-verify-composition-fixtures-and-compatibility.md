---
id: TASK-293
title: "Verify composition fixtures and compatibility"
type: feature
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-04T10:00:00+09:00
source: "PLAN-005 implementation of TASK-260's frozen composition contract"
scope: "end-to-end fixtures for the four required TASK-260 §5 scenarios, existing-plan and imported-name compatibility regression, before/after documentation, and independent review evidence"
status: todo
depends-on: [TASK-289, TASK-290, TASK-291, TASK-292]
---

# Task 293: verify composition fixtures and compatibility

## Summary

Close the loop TASK-289 through TASK-292 opened: prove the full composition feature — schema, resolver,
runtime, and CLI together — reproduces every fixture TASK-260 requires, and prove it changes nothing for
configurations that do not use `composes:`.

## Recommended direction

Build the two-project fixture from TASK-260 §3 (an `api`/`web` pair with an imported `deploy` plan each,
composed by a root `release` plan with a `depends_on`) as a real end-to-end test fixture, not only a
schema-level parse test, and drive it through all four scenarios TASK-260 §5 names: two-project success;
a composition-of-composition rejected before any child starts; a rollback failure that preserves the
original error; and a resumable partial state recovered by a plain re-invocation with no new flags.

Prove compatibility (TASK-260 §6): an existing local (non-composed) plan and existing imported
`project/item` invocations behave identically before and after this feature lands — run the existing
`internal/lifecycle/imported_plan_test.go` and `internal/cli/imported_plan_lifecycle_test.go` suites
unchanged and require them to still pass, rather than writing new tests that could hide a regression.

Update `USAGE.md` and `ARCHITECTURE.md` with the before/after configuration and invocation examples
TASK-260 §6.3 gives, and record in this card's completion evidence which independent reviewer or session
performed TASK-260 completion-criterion 6's required review (this task does not itself satisfy that
review — see TASK-260 §7 and PLAN-005's own authorship constraint: the session that wrote TASK-260 §3-§6
and PLAN-005's cards is not an independent reviewer of them).

## Completion Criteria

- [ ] The two-project accepted fixture from TASK-260 §3 runs end-to-end through `up` and succeeds, with waves executed in the declared `depends_on` order | verify: `/usr/bin/grep -Eq '^func TestCompositionFixtureTwoProjectSuccess\(' internal/integration/composition_fixture_test.go && go test ./internal/integration -tags=integration -count=1`
- [ ] The rejected composition-of-composition fixture from TASK-260 §3 fails validation before any child starts, with zero children reported as started | verify: `/usr/bin/grep -Eq '^func TestCompositionFixtureRejectedCycle\(' internal/integration/composition_fixture_test.go && go test ./internal/integration -tags=integration -count=1`
- [ ] A rollback-failure fixture (second child fails, rollback of the first also fails) reports the original second-child error unchanged and marks the first child `rollback_failed` (TASK-260 §5.2, §5 scenario 3) | verify: `/usr/bin/grep -Eq '^func TestCompositionFixtureRollbackFailurePreservesError\(' internal/integration/composition_fixture_test.go && go test ./internal/integration -tags=integration -count=1`
- [ ] A resumable-partial-state fixture re-invokes the same composition command after a rollback failure, with no new flag or persisted file, and completes successfully (TASK-260 §5.4, §5 scenario 4) | verify: `/usr/bin/grep -Eq '^func TestCompositionFixtureResumesAfterRollbackFailure\(' internal/integration/composition_fixture_test.go && go test ./internal/integration -tags=integration -count=1`
- [ ] Every existing imported-plan and single-project lifecycle test (`internal/lifecycle/imported_plan_test.go`, `internal/cli/imported_plan_lifecycle_test.go`, and the full `internal/lifecycle`/`internal/cli` suites) passes unchanged, proving no regression for non-composed configurations (TASK-260 §6.1-§6.2) | verify: `go test ./internal/config ./internal/lifecycle ./internal/cli -count=1`
- [ ] `USAGE.md` and `ARCHITECTURE.md` carry the before/after configuration and invocation examples TASK-260 §6.3 gives, and this card's completion evidence names the independent reviewer/session that performed TASK-260 completion-criterion 6's required review | verify: `make doc-check`
- [ ] Repository gates pass | verify: `make lint && make test && make test-integration && make commit-check`

## Non-goals

- No new schema, resolver, runtime, or CLI behavior beyond what TASK-289 through TASK-292 already
  implement — this card verifies and documents, it does not design.
- Does not itself close TASK-260 completion-criterion 6; it only supplies the evidence an independent
  reviewer needs, and records who performed that review once it happens.
