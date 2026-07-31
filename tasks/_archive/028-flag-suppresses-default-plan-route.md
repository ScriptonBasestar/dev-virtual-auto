---
id: TASK-028
title: "A leading flag silently suppresses the default-plan route and widens scope to the whole stack"
type: bug
priority: P1
status: done
effort: M
created-at: 2026-07-17T00:45:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: TASK-027 probing
source-severity: HIGH
needs-human: false
decision-status: decided
decision-recommendation: "Option B — error when a plan route is suppressed by flags — as the minimal honest fix; Option A is the better product but is feature work"
decision-confidence: medium
moved-at: 2026-07-17T10:55:19+09:00
decided-at: 2026-07-17T10:55:19+09:00
completed-at: 2026-07-17T11:02:43+09:00
decision: "Option B — error when flags suppress an available plan route"
completion-summary: |
  Option B: rejectSuppressedDefaultPlan errors when DefaultPlan is non-empty
  and leading flags would skip the plan route (e.g. dva up --dev). Wired into
  up/down/stop/restart/status. Bare dva up with one plan still uses DefaultPlan.
  Documented in USAGE.md and docs/31-execution-plan-resolution.md §4-1.
verification-status: verified
verification-evidence:
  - "RED: go test failed with undefined: rejectSuppressedDefaultPlan"
  - "GREEN: go test ./internal/cli/ -count=1 EXIT=0"
  - "Unit: --dev/--force/--docker with one plan → error naming dva up p1 ..."
  - "Unit: bare dva up with one plan still uses DefaultPlan"
  - "Unit: multi-plan + flags does not fire this guard (no DefaultPlan)"
  - "Docs: USAGE.md lifecycle flags + docs/31 §4-1 default-plan note"
---

# Task 028: A Flag Silently Widens `dva up` From One Plan To The Whole Stack

## Summary

When exactly one plan is defined, bare `dva up` runs that plan. Adding any flag — including
documented ones like `--dev` — silently drops the plan route and starts **every** stack entry
instead. Same command, same intent, one extra flag, and the scope changes with no warning.

This is distinct from TASK-027 (an unknown *positional* argument). Here the argument is a
**valid, documented flag** and the user did nothing wrong.

## Evidence

Reproduced at HEAD `d3f82d2`. Stack has `s1` + `s2`; plan `p1` covers only `s1`:

```
$ dva up
[plan: p1] environment= site= entries=1
MARKER s1                      # <-- plan-scoped. correct.

$ dva up --dev
MARKER s2
MARKER s1                      # <-- whole stack. no [plan:] line. no warning. exit 0.

$ dva up --var FOO=x
MARKER s1
MARKER s2                      # <-- same, and --var is silently dropped too
```

`--dev` is documented in `dva up --help` under "DVA-specific flags".

## Root cause

`internal/cli/plan_lifecycle.go` detectPlanRoute:

```go
if len(args) > 0 {
    if _, exists := c.Plans[args[0]]; exists {
        return args[0], args[1:], true
    }
    return "", nil, false      // args[0] is "--dev" -> not a plan -> bail
}
if p := c.DefaultPlan(); p != "" {   // <-- never reached when ANY arg is present
    return p, nil, true
}
```

`DefaultPlan()` returns the sole plan when `len(c.Plans) == 1`.
Because the `len(args) > 0` branch returns early, a single leading flag skips it entirely.

## Decision: Option B (implemented)

If plans exist, no positional plan name was given, and `DefaultPlan()` is non-empty, refuse to
silently fall through: tell the user to name the plan explicitly (`dva up p1 --dev`).

### Implementation

- `rejectSuppressedDefaultPlan` in `internal/cli/plan_lifecycle.go`
- Called after `detectPlanRoute` fails on: up, down, stop, restart, status
- Non-flag unknown tokens still handled by `rejectUnknownPlanArg` / `rejectUpPositionalArg`
- Bare `dva up` (no args) with one plan still uses `DefaultPlan`

### Docs

- `USAGE.md` — default-plan note under lifecycle flags
- `docs/31-execution-plan-resolution.md` §4-1 — single-plan default + flag error

## Completion Criteria

- [x] Option A / B / C is chosen and recorded here | verify: decision = Option B
- [x] `dva up --dev` with one plan either runs the plan, or fails loudly — never silently starts unlisted entries | verify: `go test ./internal/cli/ -run TestUpRejectsFlagsThatSuppressDefaultPlan -count=1`
- [x] The default-plan route is documented or removed | verify: USAGE.md + docs/31 §4-1

## References

- [027-up-silently-ignores-unknown-args.md](../_archive/027-up-silently-ignores-unknown-args.md) — same function, unknown-positional case
- [031-execution-plan-resolution](../../docs/31-execution-plan-resolution.md) — §4-1
