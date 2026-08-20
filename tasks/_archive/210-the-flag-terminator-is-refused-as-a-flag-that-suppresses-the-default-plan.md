---
id: TASK-210
title: "The flag terminator is refused as a flag that suppresses the default plan"
type: bug
priority: P3
effort: S
created-at: 2026-08-20T15:30:00+09:00
source: "found while fixing the HIGH finding of TASK-207's adversarial review — the fix restored the terminator/bare identity on the stack route and could not restore it here"
scope: "internal/cli/plan_lifecycle.go detectPlanRoute AND rejectSuppressedDefaultPlan, both shared by up/down/stop/restart. The refusal for real flags is correct and stays; only the terminator's classification changed. Scope grew by one function during the work: the site this card named cannot reach the outcome this card wanted — see Ruling."
status: done
completed-at: 2026-08-20T17:05:00+09:00
---

# Task 210: The flag terminator is refused as a flag that suppresses the default plan

## Summary

In a config with a resolvable default plan, the flag/name separator is reported
as a flag:

```
$ dva restart --
ERROR: flags suppress the default plan "p1"; name it explicitly: dva restart p1 --
```

No flag was given. `--` is what a caller writes to say "everything after this is
a name", and here it is the only token present, so the invocation means "no names
given" — which is what a bare `dva restart` means, and a bare `dva restart` runs
the default plan without complaint.

`rejectSuppressedDefaultPlan` (`internal/cli/plan_lifecycle.go:106-124`) classifies
by leading dash:

```go
if !strings.HasPrefix(args[0], "-") {
	return nil
}
```

`--` satisfies that test without being a flag.

## Measured, on two binaries

Fixture: two plans, `default_plan: p1`, entries `s1` and `s2` writing markers.

| binary | `dva restart` | `dva restart --` |
|---|---|---|
| master `8c48687` | rc=0, p1 runs (s1) | rc=1, refused |
| TASK-207 branch  | rc=0, p1 runs (s1) | rc=1, refused |

**It is not limited to configs that write the key.** Second fixture: **one** plan
`p1`, entries `s1`/`s2`, and zero occurrences of `default_plan` anywhere in the
file. Identical on master `2d8bc83e46a9` and the final TASK-207 binary
`f0abf9449eeb`:

```
dva restart      rc=0, p1 runs (s1)
dva restart --   rc=1  ERROR: flags suppress the default plan "p1"; name it explicitly: dva restart p1 --
```

`Config.DefaultPlan` (`internal/config/config.go:585-591`) makes a lone plan the
implicit default, so the helper fires in a config that never mentions the key —
and the error names a plan the author never declared as default. A reader who
checks their `dva.yml` for `default_plan` and finds none will conclude this card
does not describe them. Whatever is ruled here has to cover both fixtures, which
is why the criteria below name the lone-plan shape explicitly.

Identical, so this is pre-existing and not a TASK-207 regression. TASK-207 is
where it became visible: that card ruled `dva restart --` means the same as a
bare `dva restart`, and its fix makes the two agree on the stack route — the
plan-less shape and the several-plans-no-default shape both match now. This
shape is the one remaining exception, and it is refused before restart's own
code runs.

## Why TASK-207 did not fix it

`rejectSuppressedDefaultPlan` is shared by `up`, `down`, `stop` and `restart`.
Narrowing it changes all four, and deciding three commands from inside a card
scoped to restart's name guard is the trap TASK-198 recorded when it deferred the
unmatchable-name class to TASK-207. That much stands.

**The reason given for it did not.** This card claimed the outcome for the other
three was "already decided elsewhere: `dva up --` is rc=1 `unknown flag \"--\"`
from `rejectUnknownFlags`". Measured on the baseline binary (`9bf3ee0`) in both
fixtures of this card, all three say the same thing restart does:

```
dva up --     rc=1  ERROR: flags suppress the default plan "alpha"; ...
dva down --   rc=1  ERROR: flags suppress the default plan "alpha"; ...
dva stop --   rc=1  ERROR: flags suppress the default plan "alpha"; ...
```

`rejectSuppressedDefaultPlan` runs *before* `rejectUnknownFlags`, so that guard is
never reached in this shape. The quoted message is real, but it belongs to a
plan-less config — a different fixture from the one this card is about. The change
was therefore behavioural for four verbs, not one, which is the opposite of the
risk profile this section described.

## Ruling, and what changed

`--` means "no names follow", so it is consumed where the plan name is read, and
what follows it is classified as if it had been written alone. Two sites, both in
`internal/cli/plan_lifecycle.go`:

```go
// detectPlanRoute — the terminator occupies the plan-name slot, so consume it
args = dropLeadingTerminator(planRoutingArgs(args))

// rejectSuppressedDefaultPlan — classify what the terminator separates
head := dropLeadingTerminator(args)
```

**The single site this card named cannot produce the outcome this card predicted.**
The proposal above said the minimal patch makes `dva restart --` "reach
`detectPlanRoute` with no args and run the default plan". `detectPlanRoute` runs
*first* in every plan-aware `RunE`, several lines before `rejectSuppressedDefaultPlan`
is consulted, so a patch in the later helper can only change which *fallthrough*
happens. Built and measured (`210-probe1`), it restarts **s1 and s2 — the whole
stack** where a bare `dva restart` runs the default plan's s1 alone. That is
TASK-198's escalation returning in the default-plan shape: `--` doing strictly
more than the bare form is permitted to do. Rejected on the measurement.

The second site is still needed, for a case this card did not list: `dva restart -- s1`
names an entry exactly as `dva restart s1` does, and on master it was refused with
the same "flags suppress" message. `detectPlanRoute` declines it either way (`s1`
is not a plan), so only the later helper can let it through.

`dropLeadingTerminator` is new and deliberately **not** `dropFlagTerminator`, whose
contract is "the first `--` anywhere" — correct for a positional name list, wrong
here. `detectPlanRoute` hands everything after the plan name to the plan runner, so
consuming a terminator further in would strip the separator out of a plan's own
extra args. Measured: `dva restart alpha -- --bogus` reports `unsupported plan flag: --`
before and after the fix; under the reused helper it would have become `--bogus`,
silently promoting a name to a flag.

## Measured after the fix

`210-before` = `9bf3ee0` (baseline), `210-fix` = this branch. Fixture **C**: two
plans, `default_plan: alpha`, entries s1/s2. Fixture **F2**: one plan `p1`, zero
occurrences of the key. Both shapes gave identical verdicts, so one table:

| invocation | before | after |
|---|---|---|
| `dva restart` (bare) | `[plan: alpha]` s1 | unchanged |
| `dva restart --` | rc=1 `flags suppress the default plan` | `[plan: alpha]` s1 — **identical to bare** |
| `dva up --` | rc=1 `flags suppress` | `[plan: alpha]` — identical to bare `dva up` |
| `dva down --` | rc=1 `flags suppress` | rc=0 `[plan: alpha]` s1 — identical to bare `dva down` |
| `dva stop --` | rc=1 `flags suppress` | rc=0 `[plan: alpha]` s1 — identical to bare `dva stop` |
| `dva restart -- alpha` | rc=1 `flags suppress` | `[plan: alpha]` s1 — identical to `dva restart alpha` |
| `dva restart -- s1` | rc=1 `flags suppress` | `stopping s1` — identical to `dva restart s1` |
| `dva restart s1`, `dva restart alpha` | unchanged | unchanged |

Run under `DOCKER_HOST=unix:///nonexistent-dva-review.sock`, so every backend call
fails instantly and the evidence is *which entries were selected*, not whether they
started. That is why `restart`/`up` show rc=1 even bare; the rc columns are only
comparable within a row.

Regression battery, fixtures A (two plans, no default) and B (no plans): every
`--` outcome unchanged, including `dva up --` → `unknown flag "--"` and
`dva restart --` → whole stack. `dva restart -- -- s1` is still refused: only a
leading terminator is consumed, so a second one stays an ordinary word, which is
what `dropFlagTerminator`'s own comment argues for.

**Two changes beyond the target, both recorded rather than folded into the above.**
Neither is a regression; both are the terminator ceasing to change the meaning of
what follows it:

1. `dva restart -- alpha` in a config with two plans and no default was
   `unknown stack entry "alpha"` and is now the plan route — matching
   `dva restart alpha`. Before the fix the terminator turned a plan name into a
   stack-entry lookup that then failed.
2. `dva up -- s1` in a default-plan config was `flags suppress the default plan`
   and is now `unknown flag "--"` — which is what that same invocation already
   said in every config shape without a resolvable default. The message stopped
   depending on a key that has nothing to do with it.

## Caller census — the surface is seven commands, not four

The criteria below ask for `up`, `down` and `stop` beside `restart`. That is the
population the card imagined, and it is smaller than the real one. Measured:

```
$ grep -rn 'detectPlanRoute(' internal --include='*.go' | grep -v _test.go | grep -v 'func detect'   # 7
$ grep -rn 'rejectSuppressedDefaultPlan(' internal --include='*.go' | grep -v _test.go | grep -v 'func reject'   # 7
```

Seven each: `up`, `down`, `stop`, `restart`, `build`, `logs`, `status`. Satisfying
this card's third criterion in full would still have left three commands
unmeasured, which is worth recording as a property of the criterion and not only
of the work — a criterion that names its population understates coverage exactly
as far as that population is wrong.

All three were then measured, on fixtures A/B/C/F2, before and after:

| command | `--` before | `--` after |
|---|---|---|
| `logs` | unchanged in all four | unchanged — cobra strips the terminator before `RunE` (no `DisableFlagParsing`) |
| `status` | unchanged in all four | unchanged, same reason |
| `build` | rc=1 `flags suppress the default plan` (C, F2) | runs the default plan, identical to a bare `dva build` |

So `build` is a fourth verb the fix corrected, and `logs`/`status` never reach the
changed code at all. No regression in any of the three.

**One pre-existing defect surfaced by the census, filed rather than fixed here.**
In the several-plans-no-default shape, `dva build --` does not refuse where a bare
`dva build` does — it starts building. Identical on `9bf3ee0` and on this branch,
so it is not a TASK-210 regression, and it is the one caller `rejectUnknownFlags`
cannot backstop because build must forward unknown flags to docker (TASK-172).
Filed as TASK-217.

## Completion Criteria

- [x] A fixture that actually declares a default plan exists, not only prose about one; the lone-plan shape needs one too, and it declares no key at all | verify: `grep -c 'default_plan: ' internal/cli/restart_names_test.go` returns ≥ 1 **→ 1** (was 0 — the bare word appears 7 times in that file, every one of them prose such as "no default_plan" in a shape label or a comment, which is why this binds on the YAML key and not on the word)
- [x] The differential test grows a default-plan shape, so the ruling is measured against a bare invocation rather than hardcoded | verify: `go test ./internal/cli/ -run TestRestartBareTerminatorMeansABareRestart -count=1 -v | grep -c -- '--- PASS: TestRestartBareTerminatorMeansABareRestart/'` returns **4** (was 2 — this counts subtests that actually ran, so unlike a grep for a fixture name it cannot be satisfied by naming a helper)
- [x] Whatever is ruled, `up`, `down` and `stop` are measured on the same fixture and the result recorded on this card | verify: human — **done, see Measured after the fix**: all four verbs measured on fixtures C and F2, before and after
- [x] `dva restart -- p1` still selects the plan | verify: human — **done**: `dva restart -- alpha` (C) and `dva restart -- p1` (F2) both print `[plan: …]` and touch only that plan's entry, identical to naming the plan without the terminator
- [x] `make test` passes | verify: `make test` — rc=0, 9 packages ok, 0 FAIL; `make lint` rc=0, 288 files checked, 0 issues

## References

- `internal/cli/plan_lifecycle.go:106-124` — `rejectSuppressedDefaultPlan`, the dash test
- `internal/cli/compose.go` — `restartCmd`'s `dropFlagTerminator` call and the plan gate beside it, which carry a pointer here
- `internal/cli/restart_names_test.go` — `TestRestartBareTerminatorMeansABareRestart`, the differential test this shape is missing from
- `tasks/_archive/207-restart-exits-0-on-an-unknown-service-name-and-the-test-pinning-it-cites-a-deleted-command.md` — the ruling whose identity claim this shape contradicts

## Technical Notes

`dropFlagTerminator` is deliberately restart-local (`internal/cli/selectors.go`),
because `parseDvaFlags` keeps the terminator on purpose for callers that take no
positional names. This card expected the fix to make it "the first second caller",
and revisiting its comment settled the question the other way: the two jobs have
different contracts — "first `--` anywhere" for a name list, "only args[0]" for the
plan-name slot — so the answer was a separate `dropLeadingTerminator`, not a shared
one. The measurement that decides it is in the Ruling section; reusing the existing
helper changes `dva restart alpha -- --bogus`.

Both halves of the fix are pinned by their own test, verified by sabotaging each
one separately: removing the `detectPlanRoute` consumption drops
`TestRestartBareTerminatorMeansABareRestart` from 4 passing subtests to 2 while the
new test stays green, and removing the `rejectSuppressedDefaultPlan` consumption
fails `TestRestartTerminatorNamesEntriesUnderAResolvableDefaultPlan` while the
bare-terminator test stays at 4. Neither test answers for the other half.
