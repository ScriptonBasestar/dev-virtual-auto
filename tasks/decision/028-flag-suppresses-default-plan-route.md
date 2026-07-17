---
id: TASK-028
title: "A leading flag silently suppresses the default-plan route and widens scope to the whole stack"
type: bug
priority: P1
status: decision
effort: M
created-at: 2026-07-17T00:45:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: TASK-027 probing
source-severity: HIGH
needs-human: true
decision-status: pending
decision-recommendation: "Option B — error when a plan route is suppressed by flags — as the minimal honest fix; Option A is the better product but is feature work"
decision-confidence: medium
moved-at: 2026-07-17T10:55:00+09:00
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

`internal/cli/plan_lifecycle.go:21-38`:

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

`DefaultPlan()` (`internal/config/config.go:592`) returns the sole plan when `len(c.Plans) == 1`.
Because the `len(args) > 0` branch returns early, a single leading flag skips it entirely.

## Why this needs a decision rather than an obvious fix

The naive fix — skip flags when looking for the plan name, so `--dev` still reaches
`DefaultPlan` — **breaks immediately**. `parsePlanFlags` (`plan_lifecycle.go:64`) supports only
`--dry-run`, `--force`, `--no-wait`, `-v/--volumes`, `--var`. It does **not** accept `--dev`,
`--docker`, `-M`, `-E`, or `-T`. So routing `dva up --dev` to the plan path would turn today's
silent widening into `ERROR: unsupported plan flag: --dev`. The current fallthrough may well be
deliberate for exactly this reason.

So the options trade real things off:

### Option A — plan path learns the lifecycle flags (best product, most work)

Teach `parsePlanFlags` `--dev`/`--docker`/`-M`/`-E`/`-T`, then let `detectPlanRoute` skip
leading flags. `dva up --dev` would run plan `p1` in dev mode — almost certainly what a user
means. Effort: M-L · Risk: medium. This is feature work, not gap remediation.

### Option B — error when flags suppress an available plan route (**recommended**, minimal)

If plans exist, no positional was given, and `DefaultPlan()` is non-empty, refuse to silently
fall through: tell the user to name the plan explicitly (`dva up p1 --dev`). Effort: S ·
Risk: low. Turns a silent scope change into a loud one. Does not make `dva up --dev` work, but
stops it doing the wrong thing quietly.

### Option C — document current behavior, change nothing

Cheapest, and defensible only if implicit whole-stack startup is intended. Weigh against
`patterns.md:62`, which already advises "avoid relying on implicit all-service startup".

## Additional finding: the default-plan route is entirely undocumented

`DefaultPlan()` is not documented anywhere. There is no `default_plan` key in `schema.json`
(the plan-related property is only `plans`), and no `*.md` outside `tasks/` describes the
single-plan implicit route. Meanwhile `docs/31-execution-plan-resolution.md:55-59` documents
the opposite contract — a name is taken from CLI args and a miss is a validation error — with
no mention of an implicit default.

So `dva up` with one plan does something real, useful, and unwritten. Whichever option wins,
the behavior needs documenting.

## Completion Criteria

- [ ] Option A / B / C is chosen and recorded here | verify: `human — routing/scope semantics; product owner must decide`
- [ ] `dva up --dev` with one plan either runs the plan, or fails loudly — never silently starts unlisted entries | verify: `human — after the decision, re-run the Evidence probe`
- [ ] The default-plan route is documented or removed | verify: `human — currently absent from schema.json and every doc`

## References

- [027-up-silently-ignores-unknown-args.md](./027-up-silently-ignores-unknown-args.md) — same function, unknown-positional case (actionable, no decision needed)
- [031-execution-plan-resolution](../../docs/31-execution-plan-resolution.md) — §4-1, documents no default-plan route
