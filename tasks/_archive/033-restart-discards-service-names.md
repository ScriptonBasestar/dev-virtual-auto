---
id: TASK-033
title: "dva restart <name> discards the name and stops+restarts the entire stack, always"
type: bug
priority: P1
status: done
effort: S
completed-at: 2026-07-17T05:15:00+09:00
created-at: 2026-07-17T03:10:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: fresh Phase 1 sweep (scope-widening mutation surfaces)
source-severity: HIGH
---

# Task 033: `restart` Advertises `[SERVICE...]`, Then Throws The Services Away

## Summary

`dva restart <anything>` ignores its positional arguments and stops, then restarts, **every**
stack entry — exit 0, no warning. It does this with or without a `plans:` section, and whether
the name is real or a typo.

The command's own `Use` string advertises `[SERVICE...]`. The plumbing to honor it already
exists. One line discards it.

## Evidence

Measured at HEAD against a rebuilt `bin/dva`. `dva validate` exits 0 first, so the probe is live.

Config: two script entries `s1`/`s2`, no `plans:` key.

```
$ dva restart definitely-not-an-entry
  EXIT=0
  ran: S2_STOP  S1_STOP  S2_UP  S1_UP
```

A name that matches nothing at all **bounced the entire running stack**. Controls:

```
$ dva stack up s1     ->  ran: S1_UP only          # the tool can scope
$ dva down bogus      ->  EXIT=1                   # siblings reject stray args
$ dva stop bogus      ->  EXIT=1
```

## Why this is worse than TASK-027 / TASK-032

Both of those *over-start*: they bring up more than asked. This one **stops running
infrastructure first**. A mistyped service name takes down databases, queues, and caches that
were serving traffic, then brings the whole stack back — losing in-memory state, connections, and
any `stop`-hook side effects along the way. The user asked to restart one thing.

It is also **plan-independent**: TASK-032 needs a config with no `plans:`, TASK-027 needed one
with plans. This fires on every config, always.

## Root cause

In `restartCmd`'s `RunE` (`internal/cli/compose.go:328` at HEAD `f9c63c8`) the names are captured
into `_`:

```go
mode, envName, includeTags, excludeTags, _ := parseDvaFlags(args)
//                                        ^ the [SERVICE...] the Use string promises
```

`compose.go:351` then calls `orch.Restart(ctx, lifecycle.UpOptions{...})` with no `Names:` field,
and `filterEntries` reads an empty `Names` as "all entries".

> **Locate this by content, not by line number.** TASK-030 adds help text to `upCmd`/`downCmd`/
> `stopCmd` *above* `restartCmd`, shifting this whole region by +9 (`parseDvaFlags` 328 → 337,
> `orch.Restart` 351 → 360). The two edits are ~20 lines apart in separate hunks and merge in
> either order — only a hardcoded line number is fragile. An earlier draft of this file cited the
> post-TASK-030 worktree numbers by mistake; caught by task030-impl.

Contrast `stack.go:52`, which captures the same return value and uses it:

```go
mode, envName, includeTags, excludeTags, names := parseDvaFlags(args)
...
Names: filteredNames,      // stack.go:93
```

Root cause unifier for this family: **the TASK-027 guard was attached to the plan-ROUTING path,
but the defect lives in the loops that discard args.** `restart` is not plan-aware at all, so no
plan-path guard will ever reach it. `compose.go` passes `Names:` at zero call sites;
`stack.go`/`plan_lifecycle.go` pass it at six.

## Why this needs no product decision (verified, not assumed)

Unlike TASK-032, the fix here is to **honor** the names, not reject them — and that is settled by
the repo, not by preference:

- `Use: "restart [OPTIONS] [SERVICE...]"` (`compose.go:311` at HEAD) explicitly advertises the argument.
  Erroring on it would contradict the command's own documented interface.
- `orch.Restart(ctx, opts UpOptions)` (`orchestrator.go:239`) already takes `UpOptions`, which
  already carries `Names []string // specific stack entry names (empty = all)`
  (`orchestrator.go:22`) and already routes it through `filterEntries` → `filterByNames`.

So the fix is passing a value that is already parsed into a field that already exists on a struct
already being constructed. No new behavior is designed.

Unknown-name behavior comes along for free and matches the reference path exactly — verified,
not assumed:

```
$ dva stack up bogus-name
  EXIT=0
  [warn] no lifecycle entries matched filters      # warns, does nothing
```

That is the harmless direction (does *less*, and says so out loud). Whether that ought to exit
non-zero is a separate question about `filterEntries` affecting `stack up` equally — explicitly
**out of scope** here. Do not change it in this task.

## Severity: HIGH / P1

Destructive-direction mutation of real infrastructure, triggered by a typo, reported as success,
on every config shape. The blast radius is the whole stack.

## Completion Criteria

- [x] `dva restart s1` restarts only `s1` | verify: `human — run the Evidence probe; assert S1_STOP and S1_UP are emitted and NEITHER S2_STOP nor S2_UP is`
- [x] `dva restart` with no args still restarts the whole stack (the legitimate path is untouched) | verify: `human — assert all four of S1_STOP S2_STOP S1_UP S2_UP are emitted`
- [x] `dva restart <unknown>` no longer touches any entry, and says so | verify: `human — assert no S*_STOP / S*_UP markers are emitted and the 'no lifecycle entries matched filters' warning appears, matching 'dva stack up bogus-name'`
- [x] Flags still work alongside names, i.e. names are not confused with flag values | verify: `human — run 'dva restart s1 -E <env>' and confirm the env applies AND scoping to s1 still holds`
- [x] A regression test asserts restart passes Names through, and is proven to fail without the fix | verify: `human — revert compose.go:337 to '_', confirm the new test FAILS, restore, confirm it passes`
- [x] `make test` and `go vet ./...` pass | verify: `make test && go vet ./...`
- [x] `filterEntries` / `stack up` unknown-name behavior is left unchanged | verify: `go test ./internal/lifecycle/`

## Outcome

Done — but **this task's own premise was wrong, and the correction is the important part.**

This file claimed the fix was "passing a value that is already parsed into a field that already
exists". That was true and *insufficient*. `Orchestrator.Restart` runs `Stop` then `Up`, and it
built its `stopOpts` without forwarding `Names`:

```go
stopOpts := StopOptions{DryRun: opts.DryRun, IncludeTags: opts.IncludeTags, ...}  // Names dropped
```

So the CLI-only fix scopes the **Up** half while the **Stop** half still bounces every entry. Two
hunks were required:

- `internal/cli/compose.go` — capture `names` from `parseDvaFlags`, pass `Names: names` to `UpOptions`
- `internal/lifecycle/orchestrator.go` — forward `Names: opts.Names` into `Restart`'s `stopOpts`

The second is the same class of change (an existing value into an existing field: `StopOptions.Names`
already existed at `orchestrator.go:44` and already routed through `filterEntries`), and it touches
neither `filterEntries` nor `filterByNames`. It was in scope because criterion 1 — "NEITHER S2_STOP
nor S2_UP" — is unsatisfiable without it.

**The CLI-only fix would have been worse than the bug.** Verified, not reasoned: with only the
`compose.go` hunk, `dva restart <unknown>` produced markers `map[s1_stop:true s2_stop:true]` — stops
with no ups. The original defect at least bounced the stack back up (stop all → up all); the partial
fix stops the entire stack and brings nothing back, leaving it **down**. A half-applied fix to a
two-layer path inverted the outcome from "over-restarts" to "silently leaves your infrastructure
off". This is the strongest argument in this run for proving each layer independently rather than
stopping at the first green test.

Verified in a scratch worktree containing **only** this change (the main worktree held TASK-032's
edits to this same file plus two other agents' work):

- Positive control 1 — revert `compose.go`, keep the orchestrator hunk → 3 of 4 tests FAIL.
- Positive control 2 — revert `orchestrator.go`, keep the CLI hunk → `TestRestart_ScopesToNamedEntry`
  and `TestRestart_UnknownNameTouchesNothing` FAIL, on `s2_stop ran, but s2 was not named`. Each hunk
  is independently necessary; neither is cargo.
- `TestRestart_NoArgsRestartsAll` stayed green through both controls — the over-filter control,
  proving the fix did not simply turn `restart` into a no-op.
- Probes against a binary built from the isolated tree, `dva validate` exiting 0 first:
  `restart s1` → S1_STOP S1_UP only; `restart` → all four; `restart <unknown>` → no markers plus the
  'no lifecycle entries matched filters' warn, matching `stack up bogus-name` exactly;
  `restart s1 -E dev` → still scoped to s1.
- Flag-slot trap checked in the form that can actually detect it: `restart -E dev` with **no** name
  restarts all four entries. (`restart s1 -E dev` cannot distinguish a leak, since scoping to s1
  holds either way — if `dev` leaked into the name slot it would scope to nothing and restart
  nothing. It does not.) `restart --mode dev` exits 1, byte-identically to the HEAD binary — a
  pre-existing message about the probe config defining no `modes:`, not a regression.
- Isolation control: the binary from this worktree does **not** reject `dva up s1` (exit 0),
  confirming TASK-032's fix is absent and these results are attributable to this change alone.
- `make test` and `go vet ./...` exit 0 in that isolated worktree.

## Follow-up found while fixing this — NOT filed

`Restart`'s `stopOpts` also drops `Env` (the `Up` half receives it, the `Stop` half does not). Probe
`restart s1 -E dev` still applied the env correctly, so it did not bite here and is not proven to be
a defect. Recorded as a latent sibling of exactly the same shape — an option silently not forwarded
across the Stop/Up seam — and left alone as out of scope. Worth its own triage.

## References

- [032-up-widens-scope-when-no-plans-configured.md](./032-up-widens-scope-when-no-plans-configured.md) — same family, same root cause, opposite fix direction (reject vs honor), decided by each command's own `Use` string
- [027-up-silently-ignores-unknown-args.md](../_archive/027-up-silently-ignores-unknown-args.md) — the guard whose plan-path placement is why this was missed
- [030-help-surfaces-understate-working-flags.md](./030-help-surfaces-understate-working-flags.md) — elsewhere help *understates* the binary; here it *overstates* it
