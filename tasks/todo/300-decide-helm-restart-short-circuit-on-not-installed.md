---
id: TASK-300
title: "Decide handling for helm-backed Restart short-circuit when release is not installed"
type: bug
priority: P2
effort: S
exec-tier: standard
created-at: 2026-09-04T00:00:00+09:00
source: "Independent review of TASK-295 (internal/lifecycle/orchestrator.go Orchestrator.Down/Stop fix)"
scope: "internal/lifecycle/orchestrator.go Orchestrator.Restart short-circuit behavior and internal/lifecycle/helm.go Plugin.Stop only"
status: todo
depends-on: []
---

# Task 300: decide handling for helm-backed Restart short-circuit when release is not installed

## Summary

TASK-295 fixed `Orchestrator.Down` and `Orchestrator.Stop`
(`internal/lifecycle/orchestrator.go`) to collect per-entry teardown failures and
`return errors.Join(...)` instead of unconditionally returning `nil`. The fix itself is
correct and out of scope here — this card is about a behavioral consequence of that fix
in a caller that was reviewed independently after TASK-295 closed.

`Orchestrator.Restart` (`internal/lifecycle/orchestrator.go`, around lines 286-300) does:

```go
if err := o.Stop(ctx, stopOpts); err != nil {
    return err
}
return o.Up(ctx, opts)
```

This guard existed before TASK-295, but was previously dead code for entry-teardown
failures, since `Stop` always returned `nil` for those. Now that `Stop` can return a real
error, this guard is live: `dva restart <plan>` where one entry's stop fails will
short-circuit and never attempt `Up`, leaving the whole stack down where it previously
would have proceeded to bring everything back up.

**Concrete risk case**: `HelmPlugin.Stop` (`internal/lifecycle/helm.go`, around lines
79-82) delegates to `Down` (`helm uninstall`), which exits non-zero for a release that is
not currently installed:

```go
func (p *HelmPlugin) Stop(ctx context.Context, pctx *PluginContext) error {
    // Helm has no graceful stop; delegate to Down (uninstall).
    return p.Down(ctx, pctx)
}
```

So `dva restart <plan>` on a plan containing a helm entry whose release isn't installed
yet — e.g. first-ever restart, or after a manual `helm uninstall` outside dva — now aborts
the whole restart before reaching `Up`, where previously it would have proceeded (since
the old `Stop` swallowed that error).

This is disclosed in `internal/cli/composition_flags.go` (around lines 401-406) as the
*intended* documented semantics for composition restart's short-circuit:

```go
// runCompositionRestart stops then brings the composition back up, mirroring
// lifecycle.Orchestrator.Restart's own stop-then-up pattern at the single-plan level:
// ... A failed stop returns immediately without attempting the up half, the same
// short-circuit lifecycle.Orchestrator.Restart uses for a single plan.
```

So the short-circuit itself is a known, documented design choice — this card is not about
reversing that. What it does not yet address is that a *helm entry that was simply never
installed* now trips that same short-circuit as if its teardown had genuinely failed,
which is a different situation from "an entry that was running failed to stop."

**Precedent for tolerating a no-op stop**: `ProcessPlugin.haltProcess`
(`internal/lifecycle/process.go`, around lines 146-153) already treats "nothing to stop"
as success — if the PID file is missing, `haltProcess` returns `nil` immediately (`// not
running`) rather than erroring. `HelmPlugin.Stop` has no equivalent idempotent-no-op
handling: it always shells out to `helm uninstall` and surfaces whatever helm returns,
including "release: not found."

**No existing test coverage**: confirmed via grep across `internal/` for `not installed`,
`release: not found`, and helm-restart-related test names — none of the existing tests in
`internal/lifecycle/helm_test.go` or elsewhere exercise `HelmPlugin.Stop`/`Down` against a
release that was never installed, nor `Orchestrator.Restart` with a helm entry in that
state.

## Recommended direction

This is an open decision, not a predetermined fix — pick one before implementing:

- **(a) Make `HelmPlugin.Stop` tolerate a not-installed release as a no-op success**,
  mirroring `ProcessPlugin.haltProcess`'s idempotent pattern: detect the "release not
  found" case (e.g. by checking `helm status <release>` first, similar to
  `HelmPlugin.Status`'s existing not-found handling, or by inspecting the `helm uninstall`
  error text/exit behavior) and return `nil` instead of propagating the error. This keeps
  `Restart`'s short-circuit meaningful for genuine teardown failures while removing the
  false-positive case.
- **(b) Leave `Restart`'s short-circuit as intentional/documented behavior** and instead
  add test coverage for the helm-not-installed-restart interaction plus a clearer error
  message so operators understand why restart aborted (e.g. distinguish "entry was never
  installed" from "entry failed to tear down" in the surfaced error).
- **(c) Something else** — e.g. scope the short-circuit check to only entries that were
  actually running before `Restart` was invoked, or change `Restart`'s semantics to
  best-effort stop (log-and-continue) specifically for this composition-restart path while
  leaving plain `dva stop`/`dva down` strict.

Whichever direction is chosen, do not weaken `Orchestrator.Down`'s or `Orchestrator.Stop`'s
TASK-295 error-aggregation behavior for entries that are genuinely running and fail to tear
down — this card is about the not-installed/never-run edge case specifically.

## Completion Criteria

- [ ] Decision recorded on which direction (a), (b), or (c) above is adopted, with
      rationale, before any implementation change | verify: human — decision documented in
      this card's Completion evidence section
- [ ] If (a): `HelmPlugin.Stop` returns `nil` (not an error) when the release is not
      installed, verified by a new unit test that stops/downs a `HelmPlugin` pointed at a
      release name helm reports as not found | verify: `/usr/bin/grep -Eq
      '^func TestHelmPlugin_Stop_ReleaseNotInstalled\(' internal/lifecycle/helm_test.go &&
      go test ./internal/lifecycle -count=1`
- [ ] If (a): `Orchestrator.Restart` proceeds to `Up` when the only "failure" was a helm
      entry that was never installed, verified against the real orchestrator (not a fake)
      | verify: `/usr/bin/grep -Eq
      '^func TestOrchestratorRestartProceedsWhenHelmEntryNotInstalled\('
      internal/lifecycle/orchestrator_test.go && go test ./internal/lifecycle -count=1`
- [ ] If (b): a new test documents and locks in the current short-circuit behavior for a
      not-installed helm entry, and the surfaced error text distinguishes "never
      installed" from "teardown failed" | verify: `/usr/bin/grep -Eq
      '^func TestOrchestratorRestartShortCircuitsOnHelmEntryNotInstalled\('
      internal/lifecycle/orchestrator_test.go && go test ./internal/lifecycle -count=1`
- [ ] Repository gates pass | verify: `make lint && make test && make test-integration &&
      make commit-check`

## Non-goals

- Not reopening or relitigating TASK-295's decided scope — `Orchestrator.Down`/`Stop`
  aggregating and returning genuine per-entry teardown failures is correct and stays as-is.
- Not widening `CompositionOrchestrator`'s frozen `Up`/`Down`/`Stop`/`Status` surface
  (TASK-291) — this card concerns `lifecycle.Orchestrator.Restart` and `HelmPlugin.Stop`
  only, not adding a composition-level `Restart` method.
- Not changing `compose` or `kubectl` plugin `Stop`/`Down` behavior — this is scoped to the
  helm plugin's specific delegate-to-uninstall pattern.
- Not required to implement all three options in "Recommended direction" — pick one and
  implement it; the others are documented alternatives for the decision record.

## Completion evidence

_(fill in on completion)_
