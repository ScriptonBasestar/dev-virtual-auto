---
id: TASK-033
title: "dva restart <name> discards the name and stops+restarts the entire stack, always"
type: bug
priority: P1
status: todo
effort: S
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

- [ ] `dva restart s1` restarts only `s1` | verify: `human — run the Evidence probe; assert S1_STOP and S1_UP are emitted and NEITHER S2_STOP nor S2_UP is`
- [ ] `dva restart` with no args still restarts the whole stack (the legitimate path is untouched) | verify: `human — assert all four of S1_STOP S2_STOP S1_UP S2_UP are emitted`
- [ ] `dva restart <unknown>` no longer touches any entry, and says so | verify: `human — assert no S*_STOP / S*_UP markers are emitted and the 'no lifecycle entries matched filters' warning appears, matching 'dva stack up bogus-name'`
- [ ] Flags still work alongside names, i.e. names are not confused with flag values | verify: `human — run 'dva restart s1 -E <env>' and confirm the env applies AND scoping to s1 still holds`
- [ ] A regression test asserts restart passes Names through, and is proven to fail without the fix | verify: `human — revert compose.go:337 to '_', confirm the new test FAILS, restore, confirm it passes`
- [ ] `make test` and `go vet ./...` pass | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && make test && go vet ./...`
- [ ] `filterEntries` / `stack up` unknown-name behavior is left unchanged | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && go test ./internal/lifecycle/`

## References

- [032-up-widens-scope-when-no-plans-configured.md](./032-up-widens-scope-when-no-plans-configured.md) — same family, same root cause, opposite fix direction (reject vs honor), decided by each command's own `Use` string
- [027-up-silently-ignores-unknown-args.md](../_archive/027-up-silently-ignores-unknown-args.md) — the guard whose plan-path placement is why this was missed
- [030-help-surfaces-understate-working-flags.md](./030-help-surfaces-understate-working-flags.md) — elsewhere help *understates* the binary; here it *overstates* it
