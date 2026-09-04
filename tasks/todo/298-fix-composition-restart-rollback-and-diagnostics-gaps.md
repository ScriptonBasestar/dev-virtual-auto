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
status: todo
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

- [ ] Composition `restart` no longer forces an inescapable rollback: either it performs true
      per-child restart per TASK-260 §4.3, or `--no-rollback` is accepted (or explicitly and
      intentionally rejected with a verb-specific message, if kept mandatory by design) on
      `restart` without silently carrying a dead `flags.noRollback` forward; composition
      `restart` is documented in USAGE.md
      | verify: `/usr/bin/grep -Eq '^func TestCompositionRestartAllowsNoRollback\(' internal/cli/composition_flags_test.go && go test ./internal/cli -count=1`
- [ ] `CompositionError.Diagnostics` rollback-failure text (the "may still be up, manual
      verification required" sentence) is actually surfaced to the operator on a real
      invocation, not reachable only through direct field access in tests
      | verify: `/usr/bin/grep -Eq '^func TestCompositionRestartSurfacesRollbackDiagnostics\(' internal/cli/composition_flags_test.go && go test ./internal/cli -count=1`
- [ ] Repository gates pass | verify: `make lint && make test && make test-integration && make commit-check`

## Non-goals

- Not widening `CompositionOrchestrator`'s frozen `Up`/`Down`/`Stop`/`Status` surface
  (TASK-291) beyond what a `restart` fix requires.
- Not changing rollback behavior for the plain `up` verb — `--no-rollback` on `up` already
  works as designed; this card is scoped to `restart`.
- Not redesigning `CompositionReport`'s structured shape (`rollback.failed`, per-child
  `Error()` already convey partial information) — only ensuring the specific diagnostics
  sentence is not silently dropped.
