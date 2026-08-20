---
id: TASK-218
title: "A lone dash escapes up's flag guard, so `dva up -` starts what a bare `dva up` refuses to start"
type: bug
priority: P2
effort: S
created-at: 2026-08-20T19:00:00+09:00
source: "found while writing TASK-210's restart table — the `-` row needed a second expected message depending on config shape, and asking why turned up an escalation behind it"
scope: "internal/cli/selectors.go:60 (rejectUnknownFlags' length test), internal/cli/plan_lifecycle.go:153 (rejectSuppressedDefaultPlan's dash test), internal/cli/selectors.go:140-141 (the message that already states the opposite rule). Not the terminator — that is TASK-216."
status: todo
---

# Task 218: a lone dash escapes up's flag guard

## Summary

Ten places in `internal/cli` decide whether a token is a flag by testing
`strings.HasPrefix(a, "-")`. Exactly **one** of the ten also tests the token's
length:

```go
// selectors.go:60, rejectUnknownFlags
if len(a) < 2 || !strings.HasPrefix(a, "-") {
    continue
}
```

So `-` is a flag to nine guards and not-a-flag to the tenth, and the tenth is
the only one standing between `dva up` and the whole stack. Where several plans
are declared and none is the default, a bare `dva up` refuses to guess; adding
a `-` makes it stop refusing and start everything.

```
$ dva up
ERROR: multiple plans configured; specify one: dva up <alpha|beta>

$ dva up -
[lifecycle] s1 (compose)
ERROR: entry "s1" up failed: compose up: Docker daemon is not reachable ...
```

That is TASK-087's defect exactly — an unrecognised token silently loses its
effect and the command runs anyway — surviving for one-character tokens because
the guard TASK-087 added excludes them.

## Measured

Six fixtures, run with `DOCKER_HOST=unix:///nonexistent-dva-review.sock` so
docker fails at once and the evidence is what was selected before it failed.

Two binaries: `dc762ca`, the current tip of `origin/master`, and `b293242`,
the head of the TASK-210 branch.

The baseline was named wrong twice before it was named as a commit, which is
the reason it is written this way now. The first pass measured `9bf3ee0` and
called it "master"; it was an ancestor. The second measured `36adfd4`, which
was the tip at the time, and `origin/master` then advanced ten commits to
`dc762ca` — `compose.go` +138 lines, `plan_lifecycle.go` 501 → 508 — while
this card sat open.

Both re-runs reproduced the table. The 24 rows at `dc762ca` are byte-identical
to the 24 at `36adfd4`, so none of those ten commits touched this behaviour,
and the table below is reproducible at either. The conclusion never moved;
only the provenance line did, and a baseline named wrong is a table nobody can
reproduce.

| fixture | shape | `dva up -` | `dva down -` / `stop -` | `dva restart -` |
|---|---|---|---|---|
| A | 2 plans, no `default_plan` | **`[lifecycle] s1`** — whole stack | `unknown flag "-"` | `unknown stack entry "-"` |
| B, D, E | no `plans:` at all | `[lifecycle] <first entry>` — whole stack | `unknown flag "-"` | `unknown stack entry "-"` |
| C, F2 | a default plan resolves | `flags suppress the default plan` | same | same |

**All 24 rows are byte-identical between the two binaries** (`diff` of the two
sweeps: no output), so none of this is a TASK-210 regression; TASK-210 is only
where it became visible. Stated with its denominator because an empty sweep
would also diff clean: 24 rows each side, 6 fixtures x 4 verbs, every row
carrying an rc and a first line.

Two readings of the escalation, and they differ:

- **A is the escalation.** A bare `dva up` refuses there. `dva up -` does not.
- **B, D, E are the silent drop.** A bare `dva up` starts everything there too,
  so the outcome matches — but the `-` the user typed was discarded without a
  word, which is the half of TASK-087 that has no visible symptom.

`down` and `stop` are safe for a different reason than `up`: `teardownCommon`
(`compose.go:261`) runs its own `strings.HasPrefix(remaining[0], "-")` with no
length test, so it catches what `rejectUnknownFlags` skips. `restart` is safe
because `rejectUnknownEntryNames` reports the token as a name. `build` has
neither and passes `-` to docker, which answers `no such service: -` — the same
`requirePlanSelection` line TASK-217 owns.

## Cause

Four classifiers see the same token and answer differently. The line cited
beside each guard is where it *tests*, not where it is declared, so the
citation and the `test` column name the same line:

| # | guard | call site | test | verdict on `-` |
|---|---|---|---|---|
| 1 | `requirePlanSelection` (`plan_lifecycle.go:73`) | `compose.go:110` | `len(args) > 0` | a selection was made |
| 2 | `rejectSuppressedDefaultPlan` (`plan_lifecycle.go:153`) | `compose.go:113` | `HasPrefix(head[0], "-")` | flag |
| 3 | `rejectUpPositionalArg` (`plan_lifecycle.go:205`) | `compose.go:116` | `HasPrefix(name, "-")` → return nil | flag (someone else's problem) |
| 4 | `rejectUnknownFlags` (`selectors.go:60`) | `compose.go:169` | `len(a) < 2 \|\| !HasPrefix(a, "-")` | **not a flag** |

The order matters and is the opposite of what reading the guards by name
suggests: `requirePlanSelection` runs **first**, on the raw `args`, and
`rejectUnknownFlags` runs **last**, on the `leftover` that survives
`parseDvaFlags` (`compose.go:120`). `rejectUpPositionalArg` runs twice, at
`:116` and again at `:172`.

So guard 1 decides the plan question before any classifier has looked at the
token, and it decides it by counting: one token present, therefore the user
named something, therefore do not ask which plan. Whether anything then refuses
comes down to guard 2, which fires only where a default plan resolves — that is
the whole difference between fixtures C/F2 and fixture A. In A guard 2 returns
nil for want of a default plan, guard 3 hands the token off on its dash test,
and guard 4 skips it on length. Nothing is left to refuse.

The disagreement also ships as two contradictory sentences. `selectors.go:140-141`
already tells the user the rule:

```
→ read as a stack entry name: a lone "-" is too short to be a flag
```

while `plan_lifecycle.go:156-159` tells the same user, about the same token:

```
ERROR: flags suppress the default plan "alpha"; name it explicitly: dva up alpha -
```

Both are shipped. Only one can be the rule.

## What to change

Rule first, code second: **is a lone `-` a flag or a name?** Both answers are
defensible — `-` conventionally means stdin, and DVA has no argument that could
be stdin — but the codebase has to pick one and say it in one voice.

Whichever way it goes, the fix is not "make the two guards agree" by copying one
test into the other. Copying `len < 2` into `rejectSuppressedDefaultPlan` makes
`dva up -` in fixtures C and F2 stop refusing and start the whole stack, trading
a wrong message for a wrong action. The refusal has to survive the alignment.

The narrow shape that keeps it has to reckon with the order above.
`requirePlanSelection` runs first, so it cannot defer to a classifier that has
not run yet: making it stop counting a lone `-` means giving it its own test, or
moving the plan question after the guards that classify. The alternative is to
give `up` the name-shaped check `down`/`stop` already have at `compose.go:261`,
which fires early enough to matter. Decide against the measured table, not from
the guard you happen to be editing.

## Completion Criteria

- [ ] `dva up -` and a bare `dva up` agree on whether anything starts, in the two-plans-no-`default_plan` shape | verify: human — paste rc and the first non-warning line of both, run in a config with two plans, no `default_plan`, and two stack entries
- [ ] The agreement is pinned by a differential test comparing the two invocations, not by an expected string | verify: `grep -c 'func TestUpLoneDashAgreesWithABareUp' internal/cli/plan_lifecycle_test.go` returns 1 (today: 0). Bound on the test's source because `go test -run` exits 0 when it matches nothing, and on a name that does not exist yet: `grep -c 'func Test' internal/cli/plan_lifecycle_test.go` is 18 today, so any criterion phrased as a count of existing test functions passes before the work starts
- [ ] No two shipped messages give contradictory accounts of what `-` is | verify: human — paste `dva up -` and `dva restart -` from the same config and confirm both call `-` the same thing
- [ ] The ruling — flag or name — is written on this card before the code changes | verify: human
- [ ] `dva up --force` and `dva up --no-wait` still run in a plan-less config, so the fix did not widen into real flags | verify: human — paste both; each must reach `[lifecycle] <entry>`. Not `-v`: `dva up` has no such flag and already answers `unknown flag "-v"`, so it would pass whatever the fix does
- [ ] `make test` passes | verify: `make test`

## References

- `internal/cli/selectors.go:58-79` — `rejectUnknownFlags`, the length test
- `internal/cli/selectors.go:128-162` — `rejectUnknownEntryNames`, the message that states the opposite rule
- `internal/cli/plan_lifecycle.go:68-78` — `requirePlanSelection`, where one surviving token means "do not ask"
- `internal/cli/plan_lifecycle.go:128-160` — `rejectSuppressedDefaultPlan`
- `internal/cli/compose.go:261` — `teardownCommon`'s dash test, the one with no length exception
- `internal/cli/restart_names_test.go` — `hintUnderDefaultPlan` pins the divergent message so this fails loudly when the ruling lands
- `tasks/_archive/087-unrecognized-stack-args-become-entry-names.md` — the defect this is the one-character remainder of
- `tasks/todo/217-a-lone-terminator-disarms-build-s-plan-selection-guard-and-builds-the-whole-stack.md` — same `requirePlanSelection` line, different token
- `tasks/todo/216-the-bare-and-terminator-forms-diverge-for-up-down-and-stop.md` — the `--` half; ruled deliberately the other way, so it is not this card

## Technical Notes

The count is the part worth keeping: **10 dash tests in non-test `internal/cli`
code, 1 with a length exception.** That is the census, and it is why the defect
reads as an oversight rather than a design — nine sites did not opt into an
exception, one did, and only that one guards `up`.

`grep -rn 'HasPrefix(.*"-")' internal/cli/*.go | grep -v _test` reproduces it.
Two of the ten (`build.go:167`, `logs.go:131`) are negated — they test for the
*absence* of a dash to decide something is a name — so they cannot leak a flag;
they are counted here because they still encode the same rule and would have to
move if the ruling changes.

Piping that same grep through `grep -c 'len('` answers **3**, not 1, and the two
extra hits are why the discriminator has to be stated: `build.go:167` and
`logs.go:131` test `len(extraArgs) > 0` — the length of the *list*, not of the
token. One site tests the length of the token itself. A census whose command
does not distinguish those two would have reported this defect as three-way
disagreement and sent the fix to the wrong files.

### Why this card is 218 and not 215

Filed as TASK-215; a concurrent session filed a different TASK-215 —
`tasks/todo/215-a-flag-typed-where-a-value-belongs-is-swallowed-as-that-value.md`
— and integrated it first. Same silent collision as TASK-217's, resolved the
same way: the integrated number keeps it, this card moves, and every inbound
link moves with it. References to "TASK-215" elsewhere in the tree now mean the
flag-as-value card.
