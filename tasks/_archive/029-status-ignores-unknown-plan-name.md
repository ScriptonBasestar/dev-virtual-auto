---
id: TASK-029
title: "dva status silently ignores an unknown plan name and reports on the whole workspace"
type: bug
priority: P3
status: done
effort: XS
completed-at: 2026-07-17T02:15:00+09:00
verified-by: orchestrator (independent re-run of every criterion against a rebuilt binary, plus a positive control on the regression test)
created-at: 2026-07-17T01:45:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: TASK-027 verification (fresh Phase 1 sweep of every detectPlanRoute call site)
source-severity: LOW
---

# Task 029: `dva status <typo>` Answers A Different Question, Confidently

## Summary

`dva status <name>` silently discards an unrecognized plan name and prints **whole-workspace**
status with exit 0. The user asked for one plan's status and gets every entry's status, with
nothing indicating the name was never understood.

`status` is now the **only** plan-aware command that still does this. `up` (TASK-027), `down`,
and `stop` all reject an unknown plan name.

## Evidence

Reproduced at HEAD `77f5e8f` (the TASK-027 fix) with a two-entry stack and plan `p1` covering
only `s1`. Same config, same argument, across all four plan-aware commands:

```
up       p1-typo -> exit=1  s2-touched=0     (fixed in TASK-027)
down     p1-typo -> exit=1  s2-touched=0     (via teardownCommon)
stop     p1-typo -> exit=1  s2-touched=0     (via teardownCommon)
status   p1-typo -> exit=0  s2-touched=1     <-- reports on s2, outside the named plan
```

Control — every command accepts the real plan name `p1` with exit 0, so the probe distinguishes
"rejects unknown" from "rejects everything":

```
up p1 -> exit=0 · down p1 -> exit=0 · stop p1 -> exit=0 · status p1 -> exit=0
```

## Root cause

`internal/cli/status.go:22` takes the plan route but has no fallthrough guard:

```go
if planName, _, ok := detectPlanRoute(c, args); ok {
    return runPlanStatus(c, e, planName)
}
// ok == false -> args is never consulted again; whole-workspace status is printed
```

Confirmed by reading the rest of `status.go`: after line 22, **`args` is never referenced
again** on either the `--json` path or the human-readable path. So `NAME` in the command's own
`Use: "status [NAME]"` is only ever a plan name — every other value is discarded in full. It is
not overloaded to stack-entry names.

The fix is symmetric with TASK-027: call `rejectUnknownPlanArg(c, args)` on the fallthrough.
That helper already exists (`internal/cli/plan_lifecycle.go`) and already returns nil when no
plans are configured or when `args[0]` is a flag.

## Severity: LOW — and why it is not P1 like TASK-027

TASK-027 was P1 because the silent discard **started unintended infrastructure**. This command
is **read-only**: it mutates nothing. The harm is a confidently wrong answer, not a wrong
action. Recorded honestly as LOW rather than inflated by association with its P1 sibling.

## Risk of the fix (low, but stated)

`status` is deliberately failure-tolerant — it works with no `dva.yml` at all
(`status.go:54-58` prints "Config: not found" and returns nil), so scripts may rely on it not
erroring. The proposed guard does not disturb that:

- `dva status` with no args — unaffected (`len(args) == 0` returns nil).
- No plans configured — unaffected (`!c.HasPlans()` returns nil).
- No config at all — unaffected (the guard sits behind `err == nil`).

Only `dva status <unknown-name>` **when plans exist** changes, which is exactly the bug.

## Why this needs no product decision

Unlike TASK-017/019/028, nothing here is undecided. Three of four plan-aware commands already
reject an unknown plan name; `status` is the lone outlier, and `docs/31:59` specifies that an
unresolvable plan name is a validation error. Making the fourth match the three is consistency
work, not a semantics choice.

## Resolution

`status.go` now calls the existing `rejectUnknownPlanArg` helper (added by TASK-027) on the
plan-route fallthrough. Three lines, no new logic:

```go
if planName, _, ok := detectPlanRoute(c, args); ok {
    return runPlanStatus(c, e, planName)
}
if err := rejectUnknownPlanArg(c, args); err != nil {
    return err
}
```

```
$ dva status p1-typo
ERROR: plan 'p1-typo' not found. Available: p1
EXIT=1
```

All four plan-aware commands now agree. The helper was reused unchanged rather than
reimplemented, so `status` inherits its already-verified scoping: reads only `args[0]`, returns
nil on a leading `-`, nil when no plans exist.

**Placement is the load-bearing detail.** The guard sits *inside* the `if err == nil` block. Had
it gone outside, `status` would error when no `dva.yml` exists at all, breaking the deliberate
tolerance at `status.go:54-58` ("Config: not found", return nil). That tolerance is covered by a
dedicated regression test.

## Verification (orchestrator, 2026-07-17)

Every criterion re-run independently against a freshly built `bin/dva`, not taken from the
implementation agent's report:

```
C1  status p1-typo (plans exist)  -> exit=1, "plan 'p1-typo' not found. Available: p1"  ✅
C2  status p1                     -> exit=0 (real plan still reported)                  ✅
C3  no config at all              -> exit=0 (tolerance preserved)                       ✅
C4  config, no plans              -> exit=0 (workspace path intact)                     ✅
C4b no plans + positional arg     -> exit=0 (legacy tolerance preserved)                ✅
C5  go test -run TestStatus       -> 5 PASS                                             ✅
C6  make test -> exit=0, 0 FAIL  ·  go vet ./... -> exit=0, no output                   ✅
```

**Positive control on C5** — with the guard call disabled (`if false`),
`TestStatusRejectsUnknownPlanName` fails:

```
--- FAIL: TestStatusRejectsUnknownPlanName
    'dva status p1-typo' returned nil; an unknown plan name must not silently report on the whole workspace
```

while the four tolerance/regression tests still pass — correct, since they assert preserved
behavior the guard must not touch. The test binds to this fix rather than passing vacuously.

The full behavior table, re-measured at the fix:

```
up       p1-typo -> exit=1     down p1-typo -> exit=1
stop     p1-typo -> exit=1     status p1-typo -> exit=1   <-- was exit=0, reporting on s2
```

Test hermeticity: `status_plan_test.go` reuses the `useConfig` helper from
`plan_lifecycle_test.go` (`t.TempDir` + `t.Chdir` + cached `cfg` reset), rather than duplicating
the fixture.

## Completion Criteria

- [x] `dva status <unknown-name>` exits non-zero and names the unknown argument when plans exist | verify: `cd $(mktemp -d) && printf 'version: "0.1.0"\nstack:\n  s1:\n    default_runner: script\n    runners:\n      script:\n        up: echo S1\nplans:\n  p1:\n    entries:\n      - name: s1\n' > dva.yml && ! $OLDPWD/bin/dva status p1-typo`
- [x] `dva status p1` still reports the real plan | verify: `cd $(mktemp -d) && printf 'version: "0.1.0"\nstack:\n  s1:\n    default_runner: script\n    runners:\n      script:\n        up: echo S1\nplans:\n  p1:\n    entries:\n      - name: s1\n' > dva.yml && $OLDPWD/bin/dva status p1`
- [x] `dva status` with no config still succeeds (tolerance preserved) | verify: `cd $(mktemp -d) && $OLDPWD/bin/dva status`
- [x] `dva status` with a config but no plans still reports the workspace | verify: `cd $(mktemp -d) && printf 'version: "0.1.0"\nstack:\n  s1:\n    default_runner: script\n    runners:\n      script:\n        up: echo S1\n' > dva.yml && $OLDPWD/bin/dva status`
- [x] A regression test covers `status <unknown>` failing, with the no-plans and no-args cases held | verify: `make test`
- [x] `make test` and `go vet ./...` pass | verify: `make test && go vet ./...`

## References

- [027-up-silently-ignores-unknown-args.md](../_archive/027-up-silently-ignores-unknown-args.md) — same class, P1, mutating; introduced the `rejectUnknownPlanArg` helper this task reuses
- [028-flag-suppresses-default-plan-route.md](./028-flag-suppresses-default-plan-route.md) — same function, flag-suppression case, awaiting a human decision
- [031-execution-plan-resolution](../../docs/31-execution-plan-resolution.md) — §4-1 specifies an unresolvable plan name is a validation error
