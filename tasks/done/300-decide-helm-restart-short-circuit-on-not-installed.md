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
status: done
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

- [x] Decision recorded on which direction (a), (b), or (c) above is adopted, with
      rationale, before any implementation change | verify: human — decision documented in
      this card's Completion evidence section
- [x] If (a): `HelmPlugin.Stop` returns `nil` (not an error) when the release is not
      installed, verified by a new unit test that stops/downs a `HelmPlugin` pointed at a
      release name helm reports as not found | verify: `/usr/bin/grep -Eq
      '^func TestHelmPlugin_Stop_ReleaseNotInstalled\(' internal/lifecycle/helm_test.go &&
      go test ./internal/lifecycle -count=1`
- [x] If (a): `Orchestrator.Restart` proceeds to `Up` when the only "failure" was a helm
      entry that was never installed, verified against the real orchestrator (not a fake)
      | verify: `/usr/bin/grep -Eq
      '^func TestOrchestratorRestartProceedsWhenHelmEntryNotInstalled\('
      internal/lifecycle/orchestrator_test.go && go test ./internal/lifecycle -count=1`
- [x] (b) not applicable — (a) was adopted, so no short-circuit-locking test was written for
      the not-installed case; see Decision Record.
- [x] Repository gates pass | verify: `make lint && make test && make test-integration &&
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

## Decision Record

**Adopted: (a)** — make `HelmPlugin.Stop` tolerate a not-installed release as a no-op
success.

Rationale: this is the actual false-negative in play — an operator's `dva restart` fails
for no real reason, on an entry that was never even running yet, because
`Orchestrator.Restart`'s `Stop`-then-`Up` short-circuit (correctly made live by TASK-295)
cannot tell "teardown genuinely failed" apart from "there was nothing to tear down."
`HelmPlugin` is the odd one out here: `ProcessPlugin.haltProcess`
(`internal/lifecycle/process.go`) already treats a missing PID file — its own
nothing-to-stop signal — as a no-op success rather than an error, so (a) does not invent a
new idiom, it extends an idiom this codebase already committed to for exactly this class of
problem. (b) would leave the false-negative in place and just make it easier to diagnose,
and (c) would either narrow `Restart`'s short-circuit semantics or add best-effort-stop
behavior scoped to one call path — a larger, more speculative change than the bug warrants.
(a) is also the most bounded of the three: it touches one plugin method, reuses
`HelmPlugin.Status`'s existing not-found probe (`helm status <release> -o json`, any error
treated as not-installed) instead of inventing a second detection mechanism, and leaves
`Orchestrator.Down`/`Stop`'s TASK-295 error-aggregation completely untouched for entries
that are genuinely running and fail to tear down.

## Completion evidence

`HelmPlugin.Stop` (`internal/lifecycle/helm.go`) now probes `releaseInstalled` — a new
helper that runs `helm status <release> -o json` under the caller's context and the entry's
configured environment, reporting "not installed" only for helm's own not-found signal and
returning every other probe failure as a real error — before delegating to `Down`
(`helm uninstall`). When the release is not installed (and the call is not a dry run), Stop
returns `nil` immediately instead of surfacing helm's "release: not found" as a teardown
failure. Dry-run calls are untouched: they still flow through `Down`'s existing
`pctx.DryRun` branch, which logs and returns nil without touching the cluster, so
`releaseInstalled` never shells out to real `helm status` during a dry run. A nil
`pctx.Entry.Helm` (already-tested case) is guarded explicitly so the no-op check is skipped
and `Down`'s own nil-config short-circuit handles it, unchanged.

`Orchestrator.Restart` (`internal/lifecycle/orchestrator.go`) required no code change: its
`Stop`-then-`Up` short-circuit is correct as-is (per this card's non-goals) — it was only
ever wrong because `HelmPlugin.Stop` reported a false failure for the not-installed case,
which is now fixed at the source.

Both new tests use a `helmNotInstalledShim` test helper (`internal/lifecycle/helm_test.go`)
that puts a fake `helm` executable first on `PATH` — mirroring the existing
`installShims`/`kubectlShim` pattern in this package (`docker_daemon_test.go`,
`internal/runner/kubectl_steps_test.go`) rather than requiring a real helm binary or
cluster: `helm status` exits 1 with helm's own `Error: release: not found` on stderr,
`helm uninstall` exits 1, `helm upgrade --install` exits 0, and `PATH` is replaced (not
prepended) so a real helm on the machine cannot decide the result.

- `TestHelmPlugin_Stop_ReleaseNotInstalled` (`internal/lifecycle/helm_test.go`) — calls
  `HelmPlugin.Stop` directly against a release the shim reports as not found and asserts it
  returns `nil`.
- `TestOrchestratorRestartProceedsWhenHelmEntryNotInstalled`
  (`internal/lifecycle/orchestrator_test.go`) — builds a real `Orchestrator` (via
  `NewOrchestrator`, not a fake) with one helm-backed stack entry pointed at a
  never-installed release, calls `Restart`, and asserts it returns `nil` — proving `Up` is
  reached rather than the restart aborting after `Stop`.

### Files changed

- `internal/lifecycle/helm.go` — `HelmPlugin.Stop` gains the not-installed no-op check;
  new `HelmPlugin.releaseInstalled` and `helmReportsReleaseNotFound` helpers.
- `internal/lifecycle/helm_test.go` — new `helmShim`/`helmNotInstalledShim`/
  `helmStopContext` helpers, `TestHelmPlugin_Stop_ReleaseNotInstalled`, and the three
  post-review tests listed below.
- `internal/lifecycle/orchestrator_test.go` — new
  `TestOrchestratorRestartProceedsWhenHelmEntryNotInstalled`.

### Gates

All run clean in the task worktree: `make build` (generated files already up to date, no
diff), `make lint` (0 issues), `make test` (`-race -cover ./...`, all packages `ok`,
`internal/lifecycle` included), `make test-integration` (`ok`), `make doc-check`
(`oversized_docs: 0`, `status_mismatches: 0`, `doc-check: OK`), `make commit-check` (`OK —
every non-exempt subject since the baseline matches the format SSOT`). Both literal
verify-binding commands from this card's Completion Criteria were also run directly and
pass:
`/usr/bin/grep -Eq '^func TestHelmPlugin_Stop_ReleaseNotInstalled\(' internal/lifecycle/helm_test.go && go test ./internal/lifecycle -count=1`
and
`/usr/bin/grep -Eq '^func TestOrchestratorRestartProceedsWhenHelmEntryNotInstalled\(' internal/lifecycle/orchestrator_test.go && go test ./internal/lifecycle -count=1`.
Existing helm/orchestrator tests (including `TestHelmPlugin_Stop_NilConfig`,
`TestHelmPlugin_DryRun_Down`, `TestOrchestratorStopReturnsErrorOnEntryFailure`,
`TestOrchestratorDownReturnsErrorOnEntryFailure`) remain unaffected.

### Post-review fix (MAJOR-1: probe misclassification)

Independent review of the first implementation, before integration, rejected the original
`releaseInstalled` on two counts. Both are fixed here; the decision to adopt option (a) is
unchanged.

**What was wrong.** The first version ran `exec.Command(cmd, args...).Output()` and did
`return err == nil` — every failure meant "not installed". Two consequences:

1. *Over-broad classification.* An auth failure, an unreachable cluster, a bad kube
   context, or a malformed release name all exit non-zero, so `Stop` returned `nil` and
   reported a teardown that never happened as a success. That is precisely the class of
   silently-swallowed teardown failure TASK-295 closed, reopened one layer down, and it
   contradicts this card's non-goal of not weakening error aggregation for entries that
   are genuinely running.
2. *Environment mismatch.* `Up`/`Down` shell out via `dvaexec.ExecSubprocess`, which sets
   `c.Env = pctx.Env.EnvSlice()` — `os.Environ()` plus the config's `vars`. dva never
   exports config vars into its own process, so a bare `exec.Command` probe could not see
   a `KUBECONFIG` supplied through `dva.yml` that the `helm uninstall` it gates *would*
   see. The probe would then find no release and skip a teardown that was genuinely
   needed. The reviewer demonstrated this with a throwaway repro rather than arguing it.

**What changed.** `releaseInstalled(ctx, pctx) (bool, error)` now runs under
`exec.CommandContext(ctx, ...)` with `c.Env = pctx.Env.EnvSlice()`, checks `ctx.Err()`
first so a cancelled or timed-out probe is reported as the context error it is, and
otherwise reports not-installed only when an `*exec.ExitError`'s stderr matches the new
`helmReportsReleaseNotFound` — the `release: not found` substring, case-insensitively.
Anything else is wrapped and returned, and `Stop` propagates it instead of collapsing to
`nil`.

Note on the review instruction to reuse an existing not-found helper: there is none.
`HelmPlugin.Status` keys on nothing — a bare `if err != nil` mapping every failure to
`"stopped"` — so `helmReportsReleaseNotFound` is a single new helper rather than a second
copy of anything. `Status`'s own breadth is deliberately left alone: it is a read-only
display path outside this card's scope, and narrowing it would change reported state.
Worth a follow-up card, not a drive-by here.

`helmNotInstalledShim` had to change with the implementation: its `helm status` now emits
helm's real `Error: release: not found` on stderr. Under the narrowed classification a
shim that merely exited non-zero no longer models a not-installed release at all, so
`TestHelmPlugin_Stop_ReleaseNotInstalled` and
`TestOrchestratorRestartProceedsWhenHelmEntryNotInstalled` would have been testing the
wrong thing. Both still pass, unmodified, against the corrected shim.

Three tests were added, each verified to fail when its own implementation change is
reverted:

- `TestHelmPlugin_Stop_ProbeFailureIsNotTreatedAsNotInstalled` — the shim's `helm status`
  fails with `Kubernetes cluster unreachable`; asserts `Stop` returns an error naming the
  failed probe. Fails (`got nil`) if the classification is widened back to any-error.
- `TestHelmPlugin_Stop_ProbeUsesConfiguredEnv` — `KUBECONFIG` is supplied only through the
  entry's `vars`, and the shim's `status` succeeds only when it sees that value; asserts
  the gated `uninstall` actually runs. Fails if `c.Env` is dropped.
- `TestHelmPlugin_Stop_ProbeRespectsCancellation` — cancels while `helm status` is
  running; asserts `Stop` returns an error wrapping `context.Canceled` and never runs
  `uninstall`. Fails (`signal: killed`) if the `ctx.Err()` check is removed. (This test
  was rewritten in the second review round below; as first written it pre-cancelled and
  did not pin that behavior.)

The two marker-file shims write with the `:` redirect builtin rather than `touch`: the shim
replaces `PATH` outright, so no external binary is resolvable from inside it, and `touch`
would fail silently — making the cancellation test's negative assertion vacuous.

All six gates were re-run clean after these changes: `make build`, `make lint`
(`0 issues`), `make test`, `make test-integration`, `make doc-check`, `make commit-check`.

### Second review round (MINOR: overstated cancellation claim, unverified substring)

A second independent review of the fix above returned APPROVED WITH FINDINGS — no MAJOR;
the narrowed classification and env parity were both confirmed correct against a real helm
v4.2.0 and by independent falsification. Two MINORs were corrected here.

**1. The `ctx.Err()` check was real, but its stated justification was not.** The original
comment claimed the check prevented a cancelled probe from being misclassified as
not-installed. It cannot: a killed helm writes no stderr, so the not-found match fails with
or without the check. The reviewer also showed that
`TestHelmPlugin_Stop_ProbeRespectsCancellation` did not pin the check at all — it
pre-cancelled the context, and a pre-cancelled `exec.CommandContext` never starts the
process and returns a bare `context.Canceled`, which satisfies the assertion on its own.
Removing the check left the test passing.

The check's actual value shows only when the kill lands **mid-flight**: there `Output`
returns an `*exec.ExitError` (`signal: killed`) that does not unwrap to `context.Canceled`,
so a caller cannot tell "we gave up on the probe" from "helm rejected it". The comment now
says that, and the test was rewritten to cancel mid-flight — the shim records that `helm
status` has started, the test waits for that record before cancelling, and reverting the
`ctx.Err()` block now fails the test with `error = "helm status really-installed: signal:
killed", want it to wrap context.Canceled`. The behavior is pinned rather than assumed.

Two mechanics that rewrite depends on, both non-obvious: the shim needs a real `sleep`,
resolved via `exec.LookPath` *before* `helmShim` replaces `PATH` and symlinked in beside
the shim, because nothing external is reachable once `PATH` is replaced. And it must
`exec sleep` rather than run it as a child — otherwise `sleep` survives the kill aimed at
its parent shell, keeps the inherited stdout pipe open, and `Output` blocks on EOF for the
full sleep despite the cancellation. That pipe-inheritance behavior is a property of
`exec.CommandContext` generally, not of this fix; it is left alone here because helm is a
single binary that does not fork pipe-holding children, but `Cmd.WaitDelay` is the lever if
a future probe ever needs a hard bound.

**2. `"no release found"` was speculative and dead.** It appears nowhere in the real helm
v4.2.0 binary (checked with `strings`), and as an OR-arm it could only ever widen the set
of failures treated as nothing-to-tear-down — never narrow it. Dropped.
`helmReportsReleaseNotFound` now matches only `release: not found`, the verbatim text of
helm's `driver.ErrReleaseNotFound`, which `status`, `get`, `uninstall`, and `history` all
surface. The helper carries a comment saying so, so the single-substring choice is not left
looking like an oversight.

Rebased onto `master` at `4f1f0f5` (TASK-299) before this round; no conflicts, and the
incoming `internal/lifecycle/process.go` changes do not interact with the helm probe. All
six gates re-run clean after these corrections.
