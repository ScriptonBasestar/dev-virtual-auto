---
id: TASK-216
title: "The bare and `--` forms diverge for up, down and stop in 12 of 18 fixture pairs"
type: chore
priority: P3
effort: S
created-at: 2026-08-20T19:01:00+09:00
source: "TASK-210 made `dva restart --` identical to a bare `dva restart` in every config shape; running the same comparison across the other lifecycle verbs showed the identity stops at restart"
scope: "A ruling, not a patch. internal/cli/selectors.go:81-91 records the opposite ruling in a comment (the ruling itself is the last paragraph, 89-91), so this card exists to overturn it or to confirm it in writing — either outcome is the deliverable."
status: done
completed-at: 2026-08-21T14:30:00+09:00
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
`DOCKER_HOST=unix:///nonexistent-dva-review.sock`. The fixtures are defined in
TASK-218's `## Measured`; what matters here is the `shape` column, and A, C and
F2 are one config each while B, D and E are three variants of the same shape.

| fixture | shape | bare form | `--` form | verdict |
|---|---|---|---|---|
| C, F2 | a default plan resolves | `[plan: X] entries=1` | `[plan: X] entries=1` | **6 pairs agree** |
| B, D, E | no `plans:` at all | runs the whole stack (`down`/`stop` exit 0) | `unknown flag "--"`, rc=1 | **9 pairs diverge in outcome** |
| A | 2 plans, no `default_plan` | `multiple plans configured; specify one` | `unknown flag "--"` | **3 pairs diverge in wording only** |

Pre-existing: the six default-plan rows are the only ones TASK-210 changed, and
it changed them *into* agreement. The twelve are identical between master and
the branch.

`build` diverges in the opposite direction — its `--` form does *more* than the
bare form — and that one is TASK-217, not this card.

## The prior ruling

This is not an oversight. `internal/cli/selectors.go:81-91` states it. The
rationale is at 83-85:

> `parseDvaFlags` deliberately KEEPS the terminator in its output, and that is
> right for its other callers: `dva up` takes no positional names, so the
> surviving `--` is what makes `rejectUnknownFlags` refuse a stray one.

and the ruling those three lines support is the comment's last paragraph, 89-91:

> Restart-local on purpose. Dropping it inside `parseDvaFlags` would newly
> ACCEPT a stray terminator on every other caller, which is the regression
> `parseDvaFlags`' own closing comment warns about. TASK-207.

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

## Ruling

**Extend.** `up`, `down` and `stop` consume a leading `--` exactly as `restart`
does. `internal/cli/selectors.go`'s closing paragraph, which called the identity
restart-local, is overturned in the half that named the verbs and kept in the
half that named the *layer*.

Three things decided it, in the order they did.

**The divergence is 12 of 18, and 9 of those fail in the silent direction.**
The `## Measured` table above splits them: 9 pairs where the bare form ran the
whole stack and the `--` form refused with `unknown flag "--"`, and 3 where both
refused but said different things. The 9 are the ones that matter. `dva down --
"$@"` with an empty `"$@"` is the ordinary way a wrapper script writes
"tear down whatever is declared" — the terminator is there precisely because the
list may be empty — and it was refused in exactly the config shape (`no plans:`
at all) where `dva down` is most often run bare. A rule that breaks the
defensive form of a command and not its careless form is backwards.

**The change adds no reachable guard and removes none.** This was the second
reason, and the first draft of it overstated into a claim the review refuted, so
it is written here in the form that survived measurement. What is true is
structural: on the whole-stack path `dva up -- X` reaches
`rejectSuppressedDefaultPlan`, `rejectUpPositionalArg` and `rejectUnknownFlags`
with exactly the arguments `dva up X` reaches them with, and `dva down -- X`
reaches `teardownCommon`'s `len(remaining) > 0` block with exactly the arguments
`dva down X` does. Every refusal in the file is still reachable and still
reached.

What that does **not** license is "so only the empty case moves". It moves every
case where `dva <verb> X` already succeeded and `dva <verb> -- X` did not, and
`rejectUnknownFlags` is reached only after `parseDvaFlags` has consumed the
token, so that set is not empty. Measured, master → this branch:

| invocation | master | here | why |
|---|---|---|---|
| `dva up -` | rc=0 | rc=0 | unchanged; TASK-218's open bug |
| `dva up --debug` | rc=0 | rc=0 | unchanged |
| `dva up -- -` | rc=1 `unknown flag "--"` | **rc=0**, whole stack | inherits `dva up -` |
| `dva up -- --debug` | rc=1 | **rc=0** | inherits `dva up --debug` |
| `dva down -- --debug` | rc=1 | **rc=0** | inherits `dva down --debug` |
| `dva stop -- --debug` | rc=1 | **rc=0** | inherits `dva stop --debug` |
| `dva down -- -` | rc=1 | rc=1 `unknown flag "-"` | `teardownCommon` refuses `-` itself |
| `dva up -- --` | rc=1 | rc=1 `unknown flag "--"` | one token consumed, `up` takes none |
| `dva up --debug --` | rc=1 | rc=1 | not leading |

The identity is applied faithfully and it inherits whatever `dva up X` does,
including where that is wrong. `dva up -` accepting a bare dash is TASK-218's;
this ruling gives that bug a second spelling and fixes neither.

**There was no claim to retract, only a rule to widen.** `## What to decide`
budgeted this on a grep: the `up`/`down`/`stop` help bodies said nothing about
`--`, so nothing had to be un-said. That held — the three `Long` strings gained
a paragraph each rather than losing one, and `restart`'s existing terminator
documentation did not have to be narrowed to stay true.

**Where the drop goes is the ruling's real content.** It is
`dropLeadingTerminator` (args[0] only), called in each `RunE` above
`requirePlanSelection` — not `dropFlagTerminator` (first `--` anywhere), and not
inside `parseDvaFlags`.

- Above `requirePlanSelection`, because that guard is the first one that *counts*
  tokens rather than rejecting them: with two plans and no `default_plan` it
  reads a surviving `--` as "the user named something" and lets the invocation
  through, where a bare `dva up` is refused. A drop placed beside the guard that
  actually says `unknown flag` would have fixed 9 rows and broken 3 in the
  mirror direction.
- `dropLeadingTerminator` and not `dropFlagTerminator`, because on the
  whole-stack path a `--` further along is a token these verbs' own guards must
  classify; a helper that reached in and removed it would silence them. This is
  why "leading" and not "the first one anywhere" is load-bearing, and why the
  two verbs' families differ on purpose: `dva up --debug --` is still refused,
  while `dva restart s1 -- s2` is rc=0 because `restart` cuts its *name list* at
  the first `--` wherever it sits.
- Not in `parseDvaFlags`, and this is the half of the prior ruling that survives
  intact. Dropping it there would newly accept a stray terminator on every
  caller at once, including `build`, which diverges in the *opposite* direction
  and is TASK-217's to rule on. Three `RunE` bodies opting in one at a time, each
  saying so where it does, is what keeps the table ruled rather than arbitrary.

**What the ruling does not say.** It does not make `--` a no-op. Only the leading
one is a separator: `dva up -- --` is still `unknown flag "--"`, exactly as
`dva restart -- -- s1` is still `unknown stack entry "--"`. And it does not reach
`build` — TASK-217 owns that verb and the opposite divergence.

## Re-measured after the ruling

The six fixtures of `## Measured` rebuilt from their definitions, now with
`restart` as a fourth verb: 6 x 4 = **24 pairs**, each pair being a bare
invocation and a `--` invocation of the same verb in the same shape. Run through
`RunE` rather than a built binary — every guard in question lives there, all
four commands set `DisableFlagParsing`, and driving the functions directly makes
the *selection* observable instead of a docker failure. Stack entries are
`script:` with `up`/`stop`/`down` hooks that `touch` a marker, so "ran the whole
stack" is a marker set rather than an exit code.

The "master" column is this tree with the three `args = dropLeadingTerminator(args)`
lines replaced by a comment — a differential against the ruling's entire
mechanism, so the two columns differ by exactly the change under judgement and
nothing else.

| | master (drop removed) | after the ruling |
|---|---|---|
| pairs measured | 24 | 24 |
| bare form == `--` form | 12 | **24** |
| diverge in outcome (bare ran, `--` refused) | 9 | 0 |
| diverge in wording only (both rc=1) | 3 | 0 |
| distinct bare-form results across the 24 rows | 16 | 16 |

The last row is the vacuity control, and it is why the 24/24 is worth writing
down. An earlier run of this same sweep also printed 24/24 MATCH and was
worthless: `loadConfig` memoizes into a package global, the fixture writer did
not reset it, and all 24 rows re-measured fixture A while reporting six fixture
letters. Sixteen distinct bare-form results is the evidence that the fixtures
were actually distinguished; agreement alone never is.

Per-fixture, after the ruling — bare and `--` identical on every line:

| fixture | shape | `up` | `down` | `stop` | `restart` |
|---|---|---|---|---|---|
| A | 2 plans, no `default_plan` | rc=1 `specify one` | rc=1 `specify one` | rc=1 `specify one` | rc=1 `specify one` |
| B | no `plans:`, 2 entries | rc=0 `s1_up s2_up` | rc=0 `s1_down s2_down` | rc=0 `s1_stop s2_stop` | rc=0 stop+up both |
| C | `default_plan: alpha` | rc=0 `s1_up` | rc=0 `s1_down` | rc=0 `s1_stop` | rc=0 `s1_stop s1_up` |
| D | no `plans:`, tags + `modes:` | rc=0 `s1_up s2_up` | rc=0 `s1_down s2_down` | rc=0 `s1_stop s2_stop` | rc=0 stop+up both |
| E | no `plans:`, 3 entries | rc=0 3 x `_up` | rc=0 3 x `_down` | rc=0 3 x `_stop` | rc=0 stop+up all 3 |
| F2 | lone plan promoted | rc=0 `s1_up` | rc=0 `s1_down` | rc=0 `s1_stop` | rc=0 `s1_stop s1_up` |

`restart` is the control column: it held the identity before this card, so its
24/24 contribution measures the harness, not the ruling.

## Completion Criteria

- [x] The ruling — extend or keep — is written on this card with its reason | verify: human — `## Ruling` above; **Extend**, with the three deciding reasons and the placement argument
- [x] The 18-pair table is re-measured after the ruling and every row matches what the ruling predicts | verify: human — `## Re-measured after the ruling`. 24 pairs (the 18 plus `restart` as a control), 24/24 identical, against 12/24 with the mechanism removed. Read the vacuity-control row before the verdict row: this sweep printed 24/24 once already while measuring one fixture six times
- [x] USAGE.md's terminator section states the ruling instead of deferring it to this card | verify: `/usr/bin/grep -c '다시 판정할지는' USAGE.md` returns 0 (today: 1 — that clause is the deferral, USAGE.md:213-214). Bound on the deferral disappearing, not on `grep -c 'TASK-216' USAGE.md`, which returns 1 today and would mark this criterion passed before any work started — **re-run: 0**
- [x] If the ruling is "extend": the identity is pinned by a differential test over all three verbs, not by expected strings | verify: `/usr/bin/grep -c 'func TestLoneTerminatorMatchesTheBareForm' internal/cli/plan_lifecycle_test.go` returns 1 (today: 0). Skip this criterion, marking it N/A on the card, if the ruling is "keep" — **re-run: 1**
- [N/A] If the ruling is "keep": `selectors.go:81-91`'s comment is updated to say the identity deliberately stops at `restart`, naming this card | verify: `grep -c 'TASK-216' internal/cli/selectors.go` returns 1 (today: 0). Skip, marking N/A, if the ruling is "extend"

  **This binding does not discriminate and must not be read as a verdict.** The ruling is "extend", so this criterion is N/A — but its command returns **1** anyway, because the "extend" work rewrote that same comment to record the overturn and named the card while doing it. Any validator that executes bindings without reading the `If the ruling is` prefix reports the *keep* criterion as passed on a card that ruled the opposite way. The pair was written as mutually exclusive prose over a shared grep; only the prose is exclusive.

  The first replacement drafted here was `grep -cE 'restart-local|stops at .restart' internal/cli/selectors.go`, asserted to be 0 under "extend". **Measured: 1.** It fails for a reason worth keeping: overturning a ruling means quoting it, so selectors.go:90 now contains the sentence *"TASK-207 ... called the identity restart-local"* under a paragraph that reverses it. No phrase drawn from the losing ruling can discriminate, because the winning text has to say the phrase to overturn it.

  What discriminates is the mechanism, not the prose: `grep -c 'dropLeadingTerminator' internal/cli/compose.go` is **4** here and **0** at `git show master:internal/cli/compose.go` — 0 is the only value "keep" can produce, since "keep" is defined by those call sites not existing
- [x] `make test` passes | verify: `make test`

## References

- `internal/cli/selectors.go:81-100` — `dropFlagTerminator` (comment 81-91, function 92-100); the ruling is in the comment's last paragraph
- `internal/cli/selectors.go:58-79` — `rejectUnknownFlags`, how `up` refuses
- `internal/cli/compose.go:261` — `teardownCommon`, how `down`/`stop` refuse
- `internal/cli/plan_lifecycle.go` — `dropLeadingTerminator`, what `restart` does instead
- `USAGE.md` — the terminator section; its last paragraph is the divergence this card would remove or keep
- `tasks/_archive/207-restart-exits-0-on-an-unknown-service-name-and-the-test-pinning-it-cites-a-deleted-command.md` — where the identity was first ruled, for `restart`
- `tasks/_archive/210-the-flag-terminator-is-refused-as-a-flag-that-suppresses-the-default-plan.md` — where it was completed for `restart`, and where this table was measured
- `tasks/todo/217-a-lone-terminator-disarms-build-s-plan-selection-guard-and-builds-the-whole-stack.md` — `build`, diverging the other way
- `tasks/todo/218-a-lone-dash-escapes-up-s-flag-guard-so-dva-up-dash-starts-what-a-bare-up-refuses.md` — the same question for `-`, which is a live bug rather than a ruling

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
after the card was written. TASK-210's second fix commit made
`rejectSuppressedDefaultPlan` step aside whenever a terminator occupied the
plan-name slot, and that moved the `-- <token>` shape onto the
`rejectUnknownFlags` path in the two default-plan fixtures as well. Bisected
across the branch, so the attribution is measured rather than inferred:

| fixture | `dva up -- --bogus` at `dc762ca` | at `d51dc98` fix 1 | at `51bbf79` fix 2 |
|---|---|---|---|
| C, F2 | `flags suppress the default plan "<plan>"` | = | `unknown flag "--" for "dva up"` |
| A, B, D, E | `unknown flag "--" for "dva up"` | = | = |

Fix 1 moved nothing here; the whole change is fix 2's. `down -- --bogus` moves
the same way, and all rows are rc=1 at all three commits, so no invocation
changed from refused to accepted. The consequence for this card is
that `unknown flag "--"` is now the answer in **all six** fixtures rather than
four, so whichever way the ruling goes it applies uniformly — there is no
longer a config shape where a different message would have to be preserved.
