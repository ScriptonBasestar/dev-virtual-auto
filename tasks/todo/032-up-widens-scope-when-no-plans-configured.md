---
id: TASK-032
title: "dva up <name> silently starts the ENTIRE stack when dva.yml has no plans: section"
type: bug
priority: P1
status: todo
effort: S
created-at: 2026-07-17T03:00:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: fresh Phase 1 sweep (scope-widening mutation surfaces)
source-severity: HIGH
---

# Task 032: The TASK-027 Guard Is Conditional, So The Common Case Still Widens

## Summary

`dva up <anything>` on a `dva.yml` with **no `plans:` section** discards the argument and starts
every stack entry, exit 0. The TASK-027 fix does not cover this: `rejectUnknownPlanArg` returns
`nil` early when `!c.HasPlans()`.

Configs without plans are the majority case, so the harmful behavior TASK-027 set out to remove
is still reachable by most users.

## Evidence

Measured at HEAD against a rebuilt `bin/dva`. `dva validate` exits 0 first, so the probe is not
vacuous (a config that fails to parse would make "nothing happened" trivially true).

Config: two script entries `s1`/`s2`, **no `plans:` key**.

```
$ dva validate                    ->  EXIT=0        # probe is live

$ dva up s1                       ->  EXIT=0   ran: S1_UP S2_UP     # <-- widened
$ dva up notarealthing            ->  EXIT=0   ran: S1_UP S2_UP     # <-- widened

$ dva stack up s1                 ->  EXIT=0   ran: S1_UP           # control: scoping works
```

The `dva stack up s1` control is decisive: the tool *can* scope to a named entry, and does on the
`stack` path. `up` does not — it runs `s2` too, which the user never named.

### The asymmetry with its own siblings

```
$ dva down bogus                  ->  EXIT=1        # rejects, with or without plans
$ dva stop bogus                  ->  EXIT=1        # rejects, with or without plans
$ dva up   bogus                  ->  EXIT=0        # starts everything
```

`down`/`stop` reach `teardownCommon`, which validates leftover args and errors. `up` has no such
path. Three sibling commands in one file disagree about what a stray argument means.

## Root cause

`internal/cli/plan_lifecycle.go:52` — the guard is gated on plans existing:

```go
if c == nil || !c.HasPlans() || len(args) == 0 {
    return nil          // <-- no plans: arg silently permitted
}
```

Control then falls through `compose.go:98-109`, a switch with **only flag cases and no
`default:`**, so unrecognized positional tokens are dropped. `compose.go:135` calls `orch.Up`
with no `Names:` field, and `filterEntries` treats an empty `Names` as "all entries".

`compose.go` passes `Names:` at **zero** call sites. `stack.go` (3) and `plan_lifecycle.go` (3)
pass it. The `compose.go` command family never learned to scope at all.

## Why this needs no product decision (verified, not assumed)

The fair question is whether `dva up s1` should *scope* to `s1` (like `stack up`) rather than
error. It should not, and the answer is in the repo rather than in taste:

- `Use: "up [OPTIONS]"` (`compose.go:63`) advertises **no** positional arguments. Only
  `restart` advertises `[SERVICE...]` (that is TASK-033).
- `down [OPTIONS]` and `stop [OPTIONS]` advertise no args and **already error** on them.
- With plans configured, `up <unknown>` already errors (TASK-027).

So "positional arg on `up` that is not a plan name → error" is the behavior three of the four
plan-aware commands already implement. This task makes the fourth agree with them; it does not
invent a rule.

Scoping `up` to entry names would be a **new feature** and would contradict TASK-027's guard.
Out of scope. If it is ever wanted, `dva stack up` already does it.

## Severity: HIGH / P1

Unintended mutation of real infrastructure: a typo starts containers, ports, and volumes the
user did not ask for, and reports success. This is the same harmful direction as TASK-027
(validate-green / runtime-wrong), but reachable without any `plans:` section — i.e. by the
default config shape.

Recorded honestly: it starts *extra* things rather than destroying anything, so it is less severe
than TASK-033, which stops running infrastructure.

## Completion Criteria

- [ ] `dva up <unknown>` with no `plans:` section exits non-zero and starts nothing | verify: `human — run the Evidence probe; assert EXIT!=0 and neither S1_UP nor S2_UP is emitted`
- [ ] `dva up <real-entry-name>` with no `plans:` section also exits non-zero (an entry name is not a plan name) | verify: `human — run the Evidence probe with 'dva up s1'; assert EXIT!=0`
- [ ] The error names the problem and does not print an empty "Available:" list when no plans exist | verify: `human — confirm the message reads sensibly with zero plans configured`
- [ ] `dva up` with no args and no plans still starts the whole stack (the legitimate path is untouched) | verify: `human — assert EXIT=0 and both S1_UP and S2_UP are emitted`
- [ ] TASK-027's behavior with plans configured is unchanged | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && go test ./internal/cli/ -run 'TestUp'`
- [ ] A regression test covers the no-plans case, and is proven to fail without the fix | verify: `human — disable the guard (if false), confirm the new test FAILS, restore, confirm it passes`
- [ ] `make test` and `go vet ./...` pass | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && make test && go vet ./...`

## References

- [027-up-silently-ignores-unknown-args.md](../_archive/027-up-silently-ignores-unknown-args.md) — fixed the plans-configured half; this is the half its guard skips
- [029-status-ignores-unknown-plan-name.md](../_archive/029-status-ignores-unknown-plan-name.md) — claims "all four plan-aware commands now agree"; this task shows that agreement holds only when plans exist
- [033-restart-discards-service-names.md](./033-restart-discards-service-names.md) — same root cause, worse blast radius, plan-independent
