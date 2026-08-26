---
id: TASK-224
title: "A second terminator carries `dva build` past the plan guard and into docker, in a config where a bare `dva build` is refused"
type: bug
priority: P2
effort: S
created-at: 2026-08-21T15:55:00+09:00
source: "found by an adversarial audit of 3e949f7; TASK-217 listed this argv as a control that does not move, and it does not move because it is broken on both sides"
scope: "internal/cli/plan_lifecycle.go — requirePlanSelection's dropLeadingTerminator, and whichever of build's call sites is chosen to drop the second. Not up/down/stop/logs: TASK-217 closed those. Not the entry-name or plan-name slots: TASK-218 settled those."
status: todo
---

# Task 224: a second terminator disarms build's plan guard

## Summary

TASK-217 fixed `dva build --`, which reached docker in a config where a bare
`dva build` is refused. `dva build -- --` still does.

`dropLeadingTerminator` drops exactly one `--`. `up`, `down` and `stop` drop a
second at their call sites (TASK-216) and `logs` has its consumed by
`consumeRootPersistentFlags`, so all four arrive at the guard empty and are
refused. `build` has only the guard's own drop, so it arrives holding `["--"]`,
and `len(args) > 0` reads that as a plan selection.

## Measured

Fixture: one directory, `dva.yml` declaring two plans, **no** `default_plan`, and
two compose entries backed by a real `docker-compose.yml`.
`DOCKER_HOST=unix:///nonexistent-dva-review.sock`. Binaries built from clean
checkouts of `c51dd95` (before TASK-217's fix) and `36d3068` (the then-current
measurement commit), each with `git rev-parse HEAD` recorded and
`git status --porcelain` empty at build time. `36d3068` remains the pinned
measurement baseline; it is not a claim about the repository's current `HEAD`.

```
                 c51dd95                            36d3068 (measurement baseline)
build            rc 1 multiple plans configured     rc 1 multiple plans configured
build --         rc 0 No services to build          rc 1 multiple plans configured   <- 217 fixed
build -- --      rc 1 no such service: --           rc 1 no such service: --         <- open
up -- --         rc 1 unknown flag "--"             rc 1 multiple plans configured
down -- --       rc 1 unknown flag "--"             rc 1 multiple plans configured
stop -- --       rc 1 unknown flag "--"             rc 1 multiple plans configured
restart -- --    rc 1 unknown stack entry "--"      rc 1 unknown stack entry "--"
```

`no such service: --` is docker's, not dva's — the two occurrences of that string
in this repo are both comments *about* docker's message
(`internal/runner/docker_compose.go:325`, `internal/runner/compose_detect_test.go:24`).
It is what compose says after it has loaded the project and looked `--` up as a
service name. So the guard did not refuse: execution reached the backend, and the
only reason nothing started is that no service is named `--`.

The rc is 1 on both sides, which is why the row was mistaken for a control. It is
the message that separates a refusal from a bypass, and here both sides bypass.

`restart -- --` is a real control by the same measurement: `unknown stack entry
"--"` at both revisions, refusing on either side.

## Cause

`internal/cli/plan_lifecycle.go`, `requirePlanSelection`:

```go
args = dropLeadingTerminator(planRoutingArgs(args))
if len(args) > 0 || c.DefaultPlan() != "" {
    return nil
}
```

`dropLeadingTerminator` is deliberately narrow — it removes a `--` occupying the
plan-name slot, and only there. That is the right rule for one terminator. With
two, the first is the separator and the second lands *in* the slot, where it is
an ordinary token that no longer looks like a terminator to a guard that has
already dropped one.

The asymmetry with up/down/stop is not a second rule in the guard; it is that
those verbs drop one before they call it and build does not.

## What to change

Open question, and it should be settled on this card before code moves:

> Does `dva build -- --` have any legitimate reading? A literal `--` passed
> through to compose is answered by `no such service: --` — docker has no use
> for it either. If the answer is "no", the plan guard should see the same empty
> slot for `build` that it already sees for `up`, and the fix is to drop the
> terminator at build's call site as TASK-216 did for the others, rather than to
> loop inside `dropLeadingTerminator`, which would also change `dva build -- --`
> for configs that *do* resolve a plan.

Whichever is chosen, `dva build -- web` and `dva build -- --no-cache` must still
reach docker spelled exactly as they are now. Those are the rows TASK-217
measured as unchanged and they are the reason the terminator survives the guard
at all.

## Completion Criteria

- [ ] `dva build -- --` is refused in a two-plan no-`default_plan` config, with the same message a bare `dva build` gets | verify: human — paste both with rc, on a fixture whose `dva.yml` declares two plans and no `default_plan`; the two messages must be identical
- [ ] The refusal is pinned by a test that fails without the fix | verify: `grep -c 'func TestSecondTerminatorDoesNotDisarmBuildsPlanGuard' internal/cli/plan_lifecycle_test.go` returns 1 (today: 0). Bound on the test's source, since `go test -run` naming a test that does not exist exits 0
- [ ] `dva build -- web` and `dva build -- --no-cache` still reach docker with the argument spelled as typed | verify: human — paste both against a compose fixture; the errors must name `web` and `--no-cache`, not a plan
- [ ] `dva build -- -- --` is decided deliberately, not left to fall out of the fix | verify: human — a sentence on this card saying what it does and why
- [x] TASK-217's correction section stops calling this argv a control | verify: `grep -c 'as controls that do not move' tasks/_archive/217-a-lone-terminator-disarms-build-s-plan-selection-guard-and-builds-the-whole-stack.md` returns 0 — verified 2026-08-26 at 5649d70 (already done in the commit that filed this card; the binding stays so a revert is caught)
- [ ] `make test` passes | verify: `make test`

## References

- `internal/cli/plan_lifecycle.go:95` — `requirePlanSelection`'s `dropLeadingTerminator`, TASK-217's fix line
- `internal/cli/plan_lifecycle.go:145` — `dropLeadingTerminator`; the "and only there" rule is in its doc comment
- `internal/cli/compose.go:150` — the `dropLeadingTerminator` TASK-216 put at `up`'s call site, one line above its `requirePlanSelection` call at `:151`; `build` has no equivalent
- `internal/cli/plan_lifecycle_test.go` — `TestSecondTerminatorMeetsThePlanGuardNotTheFlagGuard` pins up/down/stop for exactly this shape; build is the verb it does not cover
- `tasks/_archive/217-a-lone-terminator-disarms-build-s-plan-selection-guard-and-builds-the-whole-stack.md` — the card this is the remainder of; its correction section now says why the control was not one
