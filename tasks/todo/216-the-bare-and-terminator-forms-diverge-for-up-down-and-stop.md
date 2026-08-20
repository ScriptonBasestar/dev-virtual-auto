---
id: TASK-216
title: "The bare and `--` forms diverge for up, down and stop in 12 of 18 fixture pairs"
type: chore
priority: P3
effort: S
created-at: 2026-08-20T19:01:00+09:00
source: "TASK-210 made `dva restart --` identical to a bare `dva restart` in every config shape; running the same comparison across the other lifecycle verbs showed the identity stops at restart"
scope: "A ruling, not a patch. internal/cli/selectors.go:81-91 records the opposite ruling in a comment (the ruling itself is the last paragraph, 89-91), so this card exists to overturn it or to confirm it in writing — either outcome is the deliverable."
status: todo
---

# Task 216: the bare and `--` forms diverge for up, down and stop

## Summary

TASK-207 and TASK-210 settled `dva restart --` ≡ `dva restart`: a `--` with
nothing after it names nothing, so it must mean exactly what the bare form
means, in both directions. That identity now holds for `restart` in all six
config shapes measured.

It does not hold for `up`, `down` or `stop`. Twelve of the eighteen
verb × fixture pairs disagree, and in nine of them the disagreement is not a
wording difference — the bare form runs the stack and the `--` form refuses.

```
$ dva down          # config with no plans:
[lifecycle] stopping s2 (compose)          rc=0

$ dva down --
ERROR: unknown flag "--" for "dva down"    rc=1
       → 'dva down' takes no service names or flags of its own
```

## Measured

Six fixtures × three verbs = 18 pairs, run against the TASK-210 branch with
`DOCKER_HOST=unix:///nonexistent-dva-review.sock`.

| fixture | shape | bare form | `--` form | verdict |
|---|---|---|---|---|
| C, F2 | a default plan resolves | `[plan: X] entries=1` | `[plan: X] entries=1` | **6 pairs agree** |
| B, D, E | no `plans:` at all | runs the whole stack (`down`/`stop` exit 0) | `unknown flag "--"`, rc=1 | **9 pairs diverge in outcome** |
| A | 2 plans, no `default_plan` | `multiple plans configured; specify one` | `unknown flag "--"` | **3 pairs diverge in wording only** |

Pre-existing: the six default-plan rows are the only ones TASK-210 changed, and
it changed them *into* agreement. The twelve are identical between master and
the branch.

`build` diverges in the opposite direction — its `--` form does *more* than the
bare form — and that one is TASK-214, not this card.

## The prior ruling

This is not an oversight. `internal/cli/selectors.go:81-91` states it, and the
ruling is the last paragraph of that comment, 89-91:

> `parseDvaFlags` deliberately KEEPS the terminator in its output, and that is
> right for its other callers: `dva up` takes no positional names, so the
> surviving `--` is what makes `rejectUnknownFlags` refuse a stray one.

The argument is coherent: a separator separates flags from *names*, and a
command that accepts no names has nothing to separate, so writing one is a
mistake worth reporting. USAGE.md carries the same reasoning.

The counter-argument is equally plain: `--` is what a shell wrapper writes when
its own argument list may be empty. `dva down -- "$@"` with an empty `"$@"` is
the exact case, and it fails today in the config shape where `dva down` is most
often used bare. TASK-207's own note about `dva restart -- "$@"` is the same
observation, made about the verb that got the identity.

## What to decide

One of two, written down here before any code moves:

1. **Extend the identity.** `up`/`down`/`stop` consume a leading `--` the way
   `restart` does. Cost: a stray `--` stops being reported, and `restart`'s help
   text has to be broadened rather than kept verb-local — `compose.go:436` and
   `441-453` are the only place in the file that documents the terminator at all,
   and they document it as a `restart` property. The `up`/`down`/`stop` help
   bodies (`compose.go:78-101`, `294-317`, `358-377`) say nothing about `--`:
   `grep -c 'terminator\|separator'` over each of the three ranges returns 0. So
   there is no claim to retract, only a rule to widen — check this before
   budgeting the change, because the reverse assumption makes it look larger.
2. **Keep the ruling.** The identity is a `restart` property, because `restart`
   is the only verb that takes names. Cost: USAGE.md must keep the divergence
   paragraph, and the wrapper-script case stays a documented trap.

Do not decide it verb-by-verb. `down` and `stop` refuse through
`teardownCommon` (`compose.go:261`) and `up` refuses through
`rejectUnknownFlags` (`selectors.go:60`) — two different code paths reaching the
same behaviour, so a partial fix would leave the table looking arbitrary rather
than ruled.

## Completion Criteria

- [ ] The ruling — extend or keep — is written on this card with its reason | verify: human
- [ ] The 18-pair table is re-measured after the ruling and every row matches what the ruling predicts | verify: human — paste the table
- [ ] USAGE.md's terminator section states the ruling instead of deferring it to this card | verify: `grep -c '다시 판정할지는' USAGE.md` returns 0 (today: 1 — that clause is the deferral, USAGE.md:213-214). Bound on the deferral disappearing, not on `grep -c 'TASK-216' USAGE.md`, which returns 1 today and would mark this criterion passed before any work started
- [ ] If the ruling is "extend": the identity is pinned by a differential test over all three verbs, not by expected strings | verify: `grep -c 'func TestLoneTerminatorMatchesTheBareForm' internal/cli/plan_lifecycle_test.go` returns 1 (today: 0). Skip this criterion, marking it N/A on the card, if the ruling is "keep"
- [ ] If the ruling is "keep": `selectors.go:81-91`'s comment is updated to say the identity deliberately stops at `restart`, naming this card | verify: `grep -c 'TASK-216' internal/cli/selectors.go` returns 1 (today: 0). Skip, marking N/A, if the ruling is "extend"
- [ ] `make test` passes | verify: `make test`

## References

- `internal/cli/selectors.go:81-100` — `dropFlagTerminator` (comment 81-91, function 92-100); the ruling is in the comment's last paragraph
- `internal/cli/selectors.go:58-79` — `rejectUnknownFlags`, how `up` refuses
- `internal/cli/compose.go:261` — `teardownCommon`, how `down`/`stop` refuse
- `internal/cli/plan_lifecycle.go` — `dropLeadingTerminator`, what `restart` does instead
- `USAGE.md` — the terminator section; its last paragraph is the divergence this card would remove or keep
- `tasks/_archive/207-restart-exits-0-on-an-unknown-service-name-and-the-test-pinning-it-cites-a-deleted-command.md` — where the identity was first ruled, for `restart`
- `tasks/_archive/210-the-flag-terminator-is-refused-as-a-flag-that-suppresses-the-default-plan.md` — where it was completed for `restart`, and where this table was measured
- `tasks/todo/214-a-lone-terminator-disarms-build-s-plan-selection-guard-and-builds-the-whole-stack.md` — `build`, diverging the other way
- `tasks/todo/215-a-lone-dash-escapes-up-s-flag-guard-so-dva-up-dash-starts-what-a-bare-up-refuses.md` — the same question for `-`, which is a live bug rather than a ruling

## Technical Notes

Worth keeping separate from the ruling: the six agreeing pairs agree because
`detectPlanRoute` consumes the terminator before anything classifies it, and
that helper is shared. The twelve disagreeing pairs disagree because two
*other* code paths classify what `detectPlanRoute` declined to route. So the
divergence is not "three verbs behave differently"; it is "one shared router
handles the default-plan shape, and each verb handles the rest for itself".
Any fix that adds a third private classifier makes the next table worse, not
better.

One more measurement, recorded here because it widened this card's subject
after the card was written. TASK-210's follow-up commit (`438eedd`) made
`rejectSuppressedDefaultPlan` step aside whenever a terminator occupied the
plan-name slot, and that moved the `-- <token>` shape onto the
`rejectUnknownFlags` path in the two default-plan fixtures as well:

| fixture | `dva up -- --bogus`, `3618257` | same, `5f2fff0` |
|---|---|---|
| C, F2 | `flags suppress the default plan "<plan>"` | `unknown flag "--" for "dva up"` |
| A, B, D, E | `unknown flag "--" for "dva up"` | unchanged |

`down -- --bogus` moves the same way; both are rc=1 before and after, so no
invocation changed from refused to accepted. The consequence for this card is
that `unknown flag "--"` is now the answer in **all six** fixtures rather than
four, so whichever way the ruling goes it applies uniformly — there is no
longer a config shape where a different message would have to be preserved.
