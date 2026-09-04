---
id: TASK-298
title: "Fix composition restart forced-rollback and silent rollback-failure diagnostics"
type: bug
priority: P2
effort: S
exec-tier: standard
created-at: 2026-09-04T00:00:00+09:00
source: "Independent review of composition restart/rollback handling — both gaps found in the same subsystem (internal/lifecycle/composition_orchestrator.go, internal/cli/composition_flags.go), bundled as one card"
scope: "internal/cli/composition_flags.go (restart verb, --no-rollback gating) and internal/lifecycle/composition_orchestrator.go (CompositionError.Diagnostics surfacing) only"
status: done
depends-on: []
---

# Task 298: fix composition restart forced-rollback and silent rollback-failure diagnostics

## Summary

Two related operability gaps in composition `restart`/rollback handling, both rated LOW
severity by an independent reviewer, bundled here because they sit in the same subsystem.

### Gap A — composition `restart` is stop-then-up with mandatory, inescapable rollback

TASK-260 §4.3 froze `restart` as "wave order, per-child restart" (각 child에 restart, wave
순서대로). The actual implementation
(`runCompositionRestart`, `internal/cli/composition_flags.go` ~lines 403-431) instead runs a
full composition `Stop` (LIFO order) followed by a full `Up`, and that `Up` carries `up`'s
automatic rollback behavior. But `validateCompositionFlagScope`
(`internal/cli/composition_flags.go` ~lines 124-127) rejects `--no-rollback` on any verb other
than `up`:

```go
case "--no-rollback":
    if verb != "up" {
        return flags, fmt.Errorf("composition plan %q: --no-rollback only applies to up (it opts out of up's automatic rollback)", planName)
    }
    flags.noRollback = true
```

So there is no way to opt out of rollback during a restart.

Concrete failure: an operator restarts a healthy, fully-up composition. One child fails to
come back up during the `Up` half. Automatic rollback then tears down every child that DID
successfully restart. The operator ends with the entire composition down, having started
from a fully-up state, with no escape hatch — even though `--no-rollback` exists precisely
for this "preserve state for inspection" scenario on `up`.

Also note `runCompositionRestart` still forwards `flags.noRollback` into
`lifecycle.CompositionUpOptions` even though it can only ever be false for this verb (dead
parameter — confirmed at ~line 427-430), and USAGE.md does not document composition `restart`
at all currently.

### Gap B — rollback-failure diagnostics are computed but never reach the operator

`CompositionOrchestrator.Up`'s rollback loop (`internal/lifecycle/composition_orchestrator.go`
~lines 252-258) builds a diagnostic string into `CompositionError.Diagnostics` when a
rollback `Down` itself fails:

```go
compErr.Diagnostics = append(compErr.Diagnostics,
    fmt.Sprintf("rollback of %s failed: %v — %s may still be up, manual verification required", label, err, label))
```

This is the exact sentence TASK-260 §5.2 step 4 requires to tell an operator to go check for
leftover live resources. But nothing ever reads `CompositionError.Diagnostics` outside of
tests (`internal/lifecycle/composition_orchestrator_test.go`):

- `renderCompositionReport` (`internal/cli/composition_flags.go` ~lines 288-296) prints only
  the structured report, which does still show `rollback.failed` and the per-child error, so
  the information is not fully lost.
- `CompositionError.Error()` (`internal/lifecycle/composition_orchestrator.go` line 112) is
  deliberately just `e.Err.Error()` — the primary error verbatim.

The specific "may still be up, manual verification required" marker sentence never actually
prints anywhere a real invocation can see it.

## Recommended direction

### Gap A

Either:

- Implement true per-child restart per TASK-260 §4.3 (wave order, restart each child in
  place) instead of stop-then-up, which sidesteps the rollback question entirely for this
  verb, or
- Permit `--no-rollback` on `restart` specifically (or force it off with a clear message
  explaining why, if the up-half's rollback is intentionally kept mandatory for restart), and
  clean up the now-meaningful (or now-provably-dead-and-removable)
  `flags.noRollback` forwarding into `CompositionUpOptions` accordingly.

Either direction should also add composition `restart` to USAGE.md, which does not document
it today.

### Gap B

Surface `CompositionError.Diagnostics` to the operator instead of leaving it reachable only
through tests — for example by having `renderCompositionReport` print each diagnostic line
for the human-output path, or by folding the diagnostics into the error's user-facing text.
Do not change what the structured report already exposes (`rollback.failed`, per-child
`Error()`) — this is about making sure the specific manual-verification warning sentence
itself reaches the operator, not about redesigning the report shape.

## Completion Criteria

- [x] Composition `restart` no longer forces an inescapable rollback: either it performs true
      per-child restart per TASK-260 §4.3, or `--no-rollback` is accepted (or explicitly and
      intentionally rejected with a verb-specific message, if kept mandatory by design) on
      `restart` without silently carrying a dead `flags.noRollback` forward; composition
      `restart` is documented in USAGE.md
      | verify: `/usr/bin/grep -Eq '^func TestCompositionRestartAllowsNoRollback\(' internal/cli/composition_flags_test.go && go test ./internal/cli -count=1`
- [x] `CompositionError.Diagnostics` rollback-failure text (the "may still be up, manual
      verification required" sentence) is actually surfaced to the operator on a real
      invocation, not reachable only through direct field access in tests
      | verify: `/usr/bin/grep -Eq '^func TestCompositionRestartSurfacesRollbackDiagnostics\(' internal/cli/composition_flags_test.go && go test ./internal/cli -count=1`
- [x] Repository gates pass | verify: `make lint && make test && make test-integration && make commit-check`

## Non-goals

- Not widening `CompositionOrchestrator`'s frozen `Up`/`Down`/`Stop`/`Status` surface
  (TASK-291) beyond what a `restart` fix requires.
- Not changing rollback behavior for the plain `up` verb — `--no-rollback` on `up` already
  works as designed; this card is scoped to `restart`.
- Not redesigning `CompositionReport`'s structured shape (`rollback.failed`, per-child
  `Error()` already convey partial information) — only ensuring the specific diagnostics
  sentence is not silently dropped.

## Completion evidence

### Gap A — true per-child restart, not stop-then-up

`runCompositionRestart` (`internal/cli/composition_restart.go`, split out of
`composition_flags.go` to stay under this repo's personal file-size guidance) no longer calls
`CompositionOrchestrator.Stop` followed by `CompositionOrchestrator.Up`. It now walks
`comp.Entries` directly — already resolved in wave, order, name order — and for each child in
turn calls `exec.Stop` then `exec.Up` (and `exec.WaitReady` unless `--no-wait`), exactly per
TASK-260 §4.3's frozen contract ("Wave 순서대로 각 child에 restart — child 내부는 child
자신의 restart 의미를 그대로 쓰고, root는 child 단위로만 순서를 준다"). One child's failure
marks only that child failed in the report and moves on to the next child; it never touches a
sibling that already restarted successfully. `CompositionOrchestrator` itself is untouched —
no `Restart` method was added to it, preserving TASK-291's frozen Up/Down/Stop/Status surface.

Because per-child restart has no whole-composition rollback step to begin with, there is
nothing for `--no-rollback` to opt out of on this verb; `validateCompositionFlagScope`
continues to reject `--no-rollback` on `restart` exactly as it did before (that rejection is
now correct by construction, not just by omission), and the dead `flags.noRollback` forward
into `CompositionUpOptions` that the old stop-then-up implementation carried is gone along
with that implementation. Composition `restart` is now documented in USAGE.md, including the
per-child, no-rollback-needed behavior and what each failure mode leaves behind.

**Verify-binding disclosure**: the card's binding requires a test named
`TestCompositionRestartAllowsNoRollback`. That name was written before this session settled on
true per-child restart over adding a `--no-rollback` escape hatch to the old stop-then-up
shape — under the direction actually implemented, there is no `--no-rollback`-on-restart
behavior left to allow. Per this session's established disclosure pattern (TASK-296 disclosed
an analogous verify-binding adaptation), the test keeps its literal name and location
(`internal/cli/composition_flags_test.go`, satisfying the binding's grep) but asserts the real
new behavior instead: it drives a real `runCompositionRestart` over a two-child composition
where the first child (`svc-good`) restarts successfully and the second (`svc-bad`) fails, then
asserts (a) `svc-good`'s script log shows exactly one extra stop+up pair (proof it was
restarted in place, not torn down as part of a whole-composition rollback), (b)
`CompositionReport.Rollback.Attempted` is empty (proof there is no whole-composition rollback
for `--no-rollback` to have opted out of), and (c) `svc-good`'s reported child state is
`ChildStateUp` while `svc-bad`'s is `ChildStateFailed`. That is the genuine, if renamed-in-
effect, fix for Gap A: the "no forced rollback" property the original name pointed at now
holds structurally rather than via an opt-out flag.

### Gap B — rollback/restart-failure diagnostics reach the operator

`renderCompositionReport` (`internal/cli/composition_flags.go`) now calls a new
`printCompositionDiagnostics(runErr)` on the human-output path (not `--json`, which keeps
emitting only the existing `rollback.failed`/per-child `Error()` fields, unchanged per this
card's non-goal). `printCompositionDiagnostics` does `errors.As` for `*lifecycle.CompositionError`
and prints each `Diagnostics` entry as a `diagnostic: ...` line. This is a generic, verb-
agnostic hookup: it surfaces `CompositionOrchestrator.Up`'s pre-existing rollback-failure
diagnostic sentence ("rollback of X failed ... manual verification required") for `up`, and,
via Gap A's fix, restart's own analogous per-child diagnostics for `restart` (stop failed —
child's state uncertain; up failed after a successful stop — child now down; readiness failed
after a successful up — child may not be serving). USAGE.md's rollback-diagnostics paragraph
was extended to describe both.

**Verify-binding disclosure**: `TestCompositionRestartSurfacesRollbackDiagnostics` cannot
literally exercise `CompositionOrchestrator.Up`'s rollback-loop diagnostics, because true
per-child restart (Gap A) never calls `Orchestrator.Up`'s rollback path at all — there is no
whole-composition rollback during a restart to fail. The test instead drives a real
`runCompositionRestart` end to end (via `captureStdout`) over the same two-child fixture,
where `svc-bad`'s restart fails at the `up` step after `svc-good` already restarted
successfully, and asserts the printed output contains `diagnostic:`, the failing child's name
(`bad-plan`), and the "manual verification required" sentence, plus that the returned
`*lifecycle.CompositionError.Diagnostics` has exactly one entry containing that sentence. This
genuinely exercises the shared `printCompositionDiagnostics`/`renderCompositionReport` code
path that is the actual Gap B fix — restart's own failure diagnostics are structurally the
same "manual verification required, here is which child and why" shape TASK-260 §5.2
specifies for `up`'s rollback-failure diagnostics, just generated by a different failure path.

### Files changed

- `internal/cli/composition_restart.go` (new) — `runCompositionRestart` and
  `restartCompositionChildren`, split out of `composition_flags.go` for file-size.
- `internal/cli/composition_flags.go` — old stop-then-up `runCompositionRestart` removed;
  `renderCompositionReport` now calls `printCompositionDiagnostics` on the human-output path;
  new `printCompositionDiagnostics` function added.
- `internal/cli/composition_flags_test.go` — added `TestCompositionRestartAllowsNoRollback`,
  `TestCompositionRestartSurfacesRollbackDiagnostics`, their shared
  `compositionRestartFixtureConfig` fixture, and the `lifecycleCompositionErrorFrom` helper, at
  the path the card's verify bindings require.
- `USAGE.md` — documented composition `restart`'s true per-child semantics and extended the
  rollback-diagnostics paragraph to cover the `diagnostic:` output line.

### Gates

All run clean in the task worktree after the fix (and re-confirmed after moving the two new
tests to their required literal file path): `make build`, `make lint` (0 issues), `make test`
(all packages `ok`), `make test-integration` (`ok`), `make doc-check` (`oversized_docs: 0`,
`doc-check: OK`), `make commit-check` (`OK — every non-exempt subject since the baseline
matches the format SSOT`). Both literal verify-binding commands from this card's Completion
Criteria were also run directly and pass:
`/usr/bin/grep -Eq '^func TestCompositionRestartAllowsNoRollback\(' internal/cli/composition_flags_test.go && go test ./internal/cli -count=1`
and the equivalent for `TestCompositionRestartSurfacesRollbackDiagnostics`.

**Post-review fix (independent reviewer, APPROVED WITH FINDINGS)**: one MAJOR and two of five
MINOR findings were fixed directly in this worktree rather than spinning up a new implementer
round, following this repo's established reviewer-endorsed-fix pattern (see TASK-297's own
Post-review fix above).

MAJOR-1 — `restartCompositionChildren` continued past a failed child into the rest of its wave
and every later wave (`continue`, joining errors), violating TASK-260 §5.2 step 1 ("Wave N의
child가 실패하면, 그 실패한 child 이후의 child는 시작하지 않는다"), which restart has no
verb-specific exemption from: unlike `up`, restart has no rollback to unwind an already-restarted
child, so continuing into later waves would restart them against a dependency this call just
found broken. Fixed by making each of the three failure branches (`Stop`/`Up`/`WaitReady`) return
immediately with a single-error `*lifecycle.CompositionError`, leaving every not-yet-restarted
child `not_started`. Proven by a new test, `TestCompositionRestartHaltsAfterFailedChild`
(`internal/cli/composition_flags_exec_test.go`), using a 3-child fixture (svc-good wave 0,
svc-bad wave 1 depends_on good-plan, svc-late wave 2 depends_on bad-plan) — it snapshots
`late-order`'s content right after the fixture's initial `up` (svc-late legitimately runs once
during setup) and asserts the file is byte-identical after the failing `restart` call, since
svc-late's `up`/`stop` scripts append and a wrongly-permitted second invocation during restart
would change it. Falsifiability-checked myself: temporarily reverted the fix back to
continue-past-failure (joining errors via `errors.Join`, matching the original bug precisely),
confirmed the test fails on exactly this assertion (`late-order changed by restart`), then
restored the real fix and confirmed the full composition suite (13 CLI + 11 lifecycle tests) is
green again.

MINOR-1/MINOR-2 — restart's report-building duplicated `CompositionOrchestrator.newReport`'s
project/plan label-splitting logic via a CLI-local, divergent, buggy pair
(`compositionChildProject`/`compositionChildPlanPart`) that returned the full unqualified name
for both columns instead of an empty `Project` (TASK-260 §5.3's actual convention,
`lifecycle.splitChildLabel`) — inconsistent with every other composition verb's report, and
partially already dead: TASK-297 had already deleted `compositionChildPlanPart` from
origin/master. Fixed by exporting `lifecycle.NewCompositionReportSkeleton` (wrapping the existing
unexported `splitChildLabel`) as the single source of truth for this label split, reusable from
`internal/cli` without adding a fifth method to the frozen `CompositionOrchestrator` surface
(TASK-291), and switching `restartCompositionChildren` to use it. This corrected label convention
broke `TestCompositionRestartAllowsNoRollback`'s existing lookup (it keyed by `child.Project`,
which is now correctly empty for these unqualified fixture names) — updated to key by
`child.Plan` plus an explicit `child.Project == ""` assertion.

Not yet acted on: MINOR-3 (an additional assertion for the "manual verification required"
sentence on a real `up`-failure path — optional, cheap), MINOR-4 (USAGE.md doesn't yet document
the fail-fast/halt behavior specifically, only the true-per-child-restart semantics already
written up above), MINOR-5 (TASK-300's stale cross-reference to code this task's restart rewrite
touches — a separate doc-only fix, left to a follow-up rather than scope-creeping this card
further).

Files touched by this fix, beyond what's listed above: `internal/lifecycle/composition_orchestrator.go`
(new exported `NewCompositionReportSkeleton`), `internal/cli/composition_restart.go` (fail-fast
rewrite), `internal/cli/composition_flags_test.go` (label-lookup fix to
`TestCompositionRestartAllowsNoRollback`, plus a split — see below), and the new
`internal/cli/composition_flags_exec_test.go`. That last file did not exist in this task's
original implementation; it's the same split origin/master's TASK-297 already carries for the
same file-size reason (confirmed via `git show origin/master:internal/cli/composition_flags_exec_test.go`
before creating it here, so this pre-emptively matches the post-rebase target structure instead
of inventing a second convention) — six pre-existing, unrelated tests
(`TestCompositionUpDownExecuteRealOrchestrator`, `TestCompositionDestructiveOptionsScopePerChild`,
`TestCompositionUpRollbackAndNoRollback`, `TestCompositionStatusReportsFailedChild`,
`TestCompositionUpDryRunPerformsNoRealExecution`, `TestCompositionDownDryRunPerformsNoRealTeardown`)
plus their shared fixture/helper moved there verbatim, and the new
`TestCompositionRestartHaltsAfterFailedChild` was added there rather than to the now-slimmer
`composition_flags_test.go`, matching where TASK-297's real-execution tests already live.

Gates re-run clean after this fix: `make lint`, `make test` (race, all packages), `make
test-integration`. `gofmt -l` clean on every touched file.
