---
id: TASK-038
title: "dva stack status silently omits entries whose plugin cannot be constructed, and reports success"
type: bug
priority: P2
status: done
effort: S
created-at: 2026-07-17T04:40:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: fresh Phase 1 sweep (examples runnability)
source-severity: MEDIUM
---

# Task 038: Three Loops Say "entry is broken", The Fourth Says Nothing At All

## Summary

`dva stack status` drops any entry whose plugin cannot be constructed — no warning, no line in the
output, exit 0. The same entry makes `dva stack up`, `down`, and `stop` **fail immediately and name
it**. So a config that cannot start reads as a healthy stack, and the entry does not merely show as
unhealthy — it is not there at all.

The four loops are the same shape, in the same file, resolving the plugin the same way. Only their
error handling disagrees.

## Evidence

Measured against `bin/dva` built via `make build` in a clean worktree at HEAD (`631ec1c`), because
the main worktree carries other agents' in-flight edits to `orchestrator.go`.

`dva validate` exits **0** on both probe configs first, so "the entry vanished" is not the trivial
consequence of a config that failed to parse.

### Probe A — realistic shape: two runners, no `default_runner`

```yaml
stack:
  s1:            { order: 10, runners: { script: { up: echo S1_UP, down: echo S1_DOWN } } }
  s2_tworunners:
    order: 20
    runners:
      script:  { up: echo S2_UP, down: echo S2_DOWN }
      process: { command: echo S2_PROC }
```

```
$ dva validate        ->  EXIT=0   "✅ dva.yml is valid"
$ dva stack status    ->  EXIT=0
                          Lifecycle:
                            [s1] script                    # <-- s2_tworunners is GONE
$ dva stack up        ->  EXIT=1
                          ERROR: entry "s2_tworunners": unknown lifecycle plugin ""
```

This config is not exotic. `runnerPluginName` returns `""` unless exactly one runner is declared or
`default_runner` names one — so **any** entry offering a choice of runners without picking a default
lands here.

### Probe B — an entry with no runner section at all

```
$ dva validate        ->  EXIT=0
$ dva stack status    ->  EXIT=0   only [s1] listed
$ dva stack up        ->  EXIT=1   ERROR: entry "s2_norunner": unknown lifecycle plugin ""
$ dva stack down      ->  EXIT=1   ERROR: entry "s2_norunner": ...
$ dva stack stop      ->  EXIT=1   ERROR: entry "s2_norunner": ...
```

`up` / `down` / `stop` are the control, and they are decisive: the entry is unambiguously broken,
all three siblings say so by name, and `status` alone reports a clean stack.

## Root cause

`internal/lifecycle/orchestrator.go` has four identical loops. Three of them (`Up`:98, `Down`:172,
`Stop`:214) do:

```go
pluginType := entry.DetectPlugin()
plugin, err := NewPlugin(pluginType)
if err != nil {
    return fmt.Errorf("entry %q: %w", entry.Name, err)   // named, fatal
}
```

`Status`:258 does:

```go
pluginType := entry.DetectPlugin()
plugin, err := NewPlugin(pluginType)
if err != nil {
    continue                                             // silent: no log, no warn, entry vanishes
}
```

The asymmetry is *inside `Status` itself*, which is the tell that the `continue` is an oversight and
not a considered choice: the very next error in the same loop body **is** reported —

```go
services, err := plugin.Status(ctx, pctx)
if err != nil {
    o.logger.Warn("plugin status query failed", "entry", entry.Name, ...)
}
```

So `Status` already knows how to report a per-entry problem without aborting. It just doesn't do it
for the one failure that means "this entry can never run".

How an entry reaches `DetectPlugin() == ""` (`internal/config/lifecycle.go:620` returns `""` when it
falls off the end of its switch): `resolveRunnerPlugin` (`lifecycle_helpers.go:61`) backfills
`Plugin` from the `runners:` map, but gives up silently when `runnerPluginName()` returns `""`
(no `default_runner` and not exactly one runner), when `GetRunnerConfig` errors, or when
`applyRunnerConfig` returns false. `NewPlugin("")` then fails `IsKnown()`.

Note this rules out the more alarming hypothesis: `Status` is **not** using a stale/legacy resolver
while the others use a modern one. All four call `DetectPlugin()`, and the backfill runs at load in
`SortedStack()`. The resolution is identical; only the error handling differs.

## Why this matters

`status` is the command a human or a health check runs to ask "is my environment up?". Its contract
is to report reality. Here it reports a *subset* of reality and calls it success — the failure mode
is invisible rather than wrong, which is worse, because there is no line to read and no exit code to
trip on. A CI gate or a `dva stack status`-based readiness check passes green on a stack that cannot
start.

This is the same family as TASK-031 (a verification surface that reports something other than the
truth) rather than TASK-032/033 (which mutate infrastructure). Nothing is destroyed, so it is not
P1 — but it is the mechanism by which the other findings stay hidden.

## Severity: MEDIUM / P2

No mutation of infrastructure. But a verification command reports green on a config that provably
cannot start, and omits the evidence entirely. Reachable from an ordinary config shape (two runners,
no default) with `dva validate` green.

## Scope note — the fix direction is settled by the repo, not by taste

Unlike TASK-035/036/037, no product decision is needed. Three sibling loops in the same file already
implement the answer, and `Status`'s own `plugin.Status` branch already implements the reporting
mechanism. The task is to make the fourth loop agree, at `Status`'s severity rather than `Up`'s.

`Status` should not `return` on a broken entry the way `Up` does — a status command that aborts on
the first bad entry would hide the *good* entries, trading one blindness for another. The entry
must appear in the output in a broken/unknown state, and/or a `Warn` must be emitted naming it, the
same way the adjacent `plugin status query failed` branch does. Whether the exit code should become
non-zero is a genuine question the implementer should raise rather than assume.

## Related observation — NOT filed, needs its own triage

Probe B showed `dva stack up` printing `S1_UP` **before** failing on `s2_norunner` (order 10 runs,
order 20 errors). Plugin constructibility is checked lazily inside the execution loop, so a broken
entry late in the order leaves the stack half-started with no rollback. A pre-flight pass that
constructs every plugin before executing any would fix both that and this task's finding at once.
Recorded here as an observation from this probe; it is a distinct defect and is not in this task's
scope. Do not fix it here.

## Completion Criteria

- [x] `dva stack status` on Probe A's config no longer omits `s2_tworunners` — it appears, marked broken/unknown | verify: `human — run the Probe A config; assert the entry name appears in stdout`
- [x] The reason is reported, naming the entry and the plugin problem, not just a blank line | verify: `human — assert the output or a Warn names 's2_tworunners' and conveys the plugin could not be constructed`
- [x] Healthy entries are still reported and status does NOT abort on the first broken entry | verify: `human — assert '[s1] script' is still present in the same run; a status command that hides good entries is not an acceptable fix`
- [x] DECISION raised (not assumed): does a broken entry make `dva stack status` exit non-zero? | verify: `human — implementer proposes, maintainer confirms; record the choice and why`
      → RAISED, not confirmed. Proposal recorded below; carried to TASK-041 so it stays in the queue rather than being buried in `_archive`.
- [x] A regression test asserts a non-constructible entry is surfaced by Status, and is proven to fail without the fix | verify: `human — restore the bare 'continue' at orchestrator.go:261, confirm the new test FAILS, restore the fix, confirm it passes`
- [x] `Up`/`Down`/`Stop` behavior is unchanged — they still fail fast and name the entry | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && go test ./internal/lifecycle/`
- [x] `make test` and `go vet ./...` pass | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && make test && go vet ./...`

## Outcome

`Status` now reports a non-constructible entry instead of dropping it. `EntryStatus` gained an
`Error` field; `Status` emits a `Warn` naming the entry (mirroring the adjacent
`plugin status query failed` branch) and appends the entry with `Error` set. `PrintStatus` renders
an empty plugin as `unknown` and prints a `BROKEN: <reason>` line.

Verified on the Probe A config (`dva validate` EXIT=0):

```
  [s1] script

  [s2_tworunners] unknown
  BROKEN: unknown lifecycle plugin "" (implemented: [...], planned: [])
```

`up`/`down`/`stop` re-checked on the same config: unchanged, still EXIT=1 naming `s2_tworunners`.

### DECISION — exit code: PROPOSAL, awaiting maintainer confirmation

**Implemented: exit code UNCHANGED (still 0).** The surfacing is in; the exit code was deliberately
not touched.

**Proposal: `dva stack status` SHOULD exit non-zero when an entry's plugin cannot be constructed.**

Rationale for the change:

- The task's own "Why this matters" is an exit-code argument: a CI gate or readiness check "passes
  green on a stack that cannot start". The `BROKEN:` line fixes the *human* reading of status but
  not the *machine* one — a script does `dva stack status || alert`, and that still passes.
- Plugin non-constructibility is not a transient runtime observation ("container is down"). It is a
  static config defect: the entry can *never* run, and all three sibling commands already treat it
  as fatal. Exit 0 keeps `status` disagreeing with `up` about the same input.

Rationale for caution (why this is a proposal, not a unilateral change):

- It changes `status`'s contract. Today exit code answers "did the query run?"; the proposal makes
  it answer "is the stack sound?" — a different question, and existing callers may depend on the
  old one.
- Blast radius beyond `dva stack status`: `stack up` (stack.go:106), `stack status` (:245),
  `plan_lifecycle.go` (:167, :233) and `status.go` (:91) all call `orch.Status()`. Notably
  `stack up` prints a status summary *after a successful up*; a non-zero status there would have to
  not turn a successful `up` into a failure.
- A distinct exit code (e.g. 2 = "config defect") rather than 1 may serve callers better, but that
  is a CLI-wide convention decision, not a local one.

Suggested scope if confirmed: non-zero only on the dedicated status commands, not on the
post-`up` summary path. Recommend deciding alongside the pre-flight-pass observation recorded above
("Related observation"), since a pre-flight construction pass would make this condition detectable
before execution and may subsume it.

**Status of this decision: OPEN.** Filed as TASK-041 (`needs-human`) so it survives this task's
archival. The proposal above is the implementer's; a maintainer still has to confirm or reject it.

### Orchestrator verification (independent — not the implementer's report)

The implementing agent went idle without reporting, so every claim below was re-established from
the diff and re-run from scratch in an isolated worktree at `7f64f58` (`/tmp/dva-t038`), whose
`internal/lifecycle/` diff was confirmed byte-identical to the main tree's before any gate was run.

**The bug reproduces at current HEAD.** Probe A (`s1` healthy; `s2_tworunners` declaring two runners
and no `default_runner`), `dva validate` EXIT=0 — the liveness gate fired once on the first draft
(`status:` is not a valid script-runner key, schema EXIT=1) and that draft was discarded as vacuous
before use:

```
HEAD binary   $ dva stack status  ->  EXIT=0
              Lifecycle:
                [s1] script                      # s2_tworunners VANISHED — reads as a clean stack

FIXED binary  $ dva stack status  ->  EXIT=0
                [s1] script
                [s2_tworunners] unknown
                BROKEN: unknown lifecycle plugin "" (implemented: [...], planned: [])
              + WARN "plugin could not be constructed" entry=s2_tworunners
```

**RED control A** — restore the bare `continue`, keep the tests. Exactly ONE test fails in the
**whole unit suite** (`make test` EXIT=2), for the right reason:

```
orchestrator_test.go:347: entry 'bad' missing from status; got entries [{ok script [] [] }]
--- FAIL: TestStatus_SurfacesUnconstructibleEntry
```

That failure text carries its own over-filter control: the healthy `ok` entry is still present in
the observed output, so the test cannot pass by hiding good entries. Restored → green.

Deliberately checked across **all** packages, not just `internal/lifecycle` — the TASK-031 lesson in
this run was that a green sibling in one package proves nothing about another. Here the whole-suite
check confirms this test is the only cover.

**RED control B** — `TestPrintStatus_BrokenEntry` covers a different seam (rendering in
`status.go`) and stayed green through control A, so it needed its own. Deleting just the
`BROKEN:` line while keeping the `Error` field:

```
orchestrator_test.go:376: expected the plugin problem to be reported, got "Lifecycle:\n\n  [s2_tworunners] unknown\n\n"
```

Right reason, and it isolates the claim: `unknown` still renders, only the reason is missing.
(Reverting `status.go` wholesale instead fails to *compile* — proof the `Error` field is
load-bearing, but not a clean control, so it was redone surgically.)

**Sibling commands unchanged**, re-run on the same probe against both binaries:

```
stack up / down / stop    FIXED_EXIT=1  HEAD_EXIT=1   ERROR: entry "s2_tworunners": unknown lifecycle plugin ""
```

`make test` EXIT=0 (5 packages), `go vet` EXIT=0, `make build` EXIT=0. `internal/lifecycle`
coverage 52.8% → 53.8%.

## References

- [031-integration-suite-red-and-excluded-from-ci.md](./031-integration-suite-red-and-excluded-from-ci.md) — same theme: a verification surface reporting something other than the truth
- [032-up-widens-scope-when-no-plans-configured.md](./032-up-widens-scope-when-no-plans-configured.md) — sibling commands in one file disagreeing about the same input, decided by in-repo precedent
- [017-runners-docker-native-semantics.md](./017-runners-docker-native-semantics.md) — the other `runners:`-resolution finding
