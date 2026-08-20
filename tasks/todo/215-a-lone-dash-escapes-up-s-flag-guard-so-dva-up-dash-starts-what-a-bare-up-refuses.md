---
id: TASK-215
title: "A lone dash escapes up's flag guard, so `dva up -` starts what a bare `dva up` refuses to start"
type: bug
priority: P2
effort: S
created-at: 2026-08-20T19:05:00+09:00
source: "found while writing TASK-210's restart table — the `-` row needed a second expected message depending on config shape, and asking why turned up an escalation behind it"
scope: "internal/cli/selectors.go:60 (rejectUnknownFlags' length test), internal/cli/plan_lifecycle.go:153 (rejectSuppressedDefaultPlan's dash test), internal/cli/selectors.go:140-141 (the message that already states the opposite rule). Not the terminator — that is TASK-216."
status: todo
---

# Task 215: a lone dash escapes up's flag guard

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

Six fixtures, `bin` built from `9bf3ee0` (master) and from the TASK-210 branch,
run with `DOCKER_HOST=unix:///nonexistent-dva-review.sock` so docker fails at
once and the evidence is what was selected before it failed.

| fixture | shape | `dva up -` | `dva down -` / `stop -` | `dva restart -` |
|---|---|---|---|---|
| A | 2 plans, no `default_plan` | **`[lifecycle] s1`** — whole stack | `unknown flag "-"` | `unknown stack entry "-"` |
| B, D, E | no `plans:` at all | `[lifecycle] <first entry>` — whole stack | `unknown flag "-"` | `unknown stack entry "-"` |
| C, F2 | a default plan resolves | `flags suppress the default plan` | same | same |

**All 24 rows are byte-identical between the two binaries**, so none of this is
a TASK-210 regression; TASK-210 is only where it became visible.

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
`requirePlanSelection` line TASK-214 owns.

## Cause

Four classifiers see the same token and answer differently:

| guard | test | verdict on `-` |
|---|---|---|
| `rejectSuppressedDefaultPlan` (`plan_lifecycle.go:153`) | `HasPrefix(head[0], "-")` | flag |
| `rejectUnknownFlags` (`selectors.go:60`) | `len(a) < 2 \|\| !HasPrefix(a, "-")` | **not a flag** |
| `rejectUpPositionalArg` (`plan_lifecycle.go:205`) | `HasPrefix(name, "-")` → return nil | flag (someone else's problem) |
| `requirePlanSelection` (`plan_lifecycle.go:73`) | `len(args) > 0` | a selection was made |

`up` calls all four, in that order. With a default plan the first one fires and
the user gets a message. Without one it returns nil, the second skips the token
on length, the third hands it off, and the fourth reads the surviving `-` as
"the user named something, do not ask which plan". Nothing is left to refuse.

The disagreement also ships as two contradictory sentences. `selectors.go:140-141`
already tells the user the rule:

```
→ read as a stack entry name: a lone "-" is too short to be a flag
```

while `plan_lifecycle.go:155-158` tells the same user, about the same token:

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

The narrow shape that keeps it: let `requirePlanSelection` stop counting tokens
that no guard downstream will accept, or give `up` the name-shaped check
`down`/`stop` already have. Decide against the measured table, not from the
guard you happen to be editing.

## Completion Criteria

- [ ] `dva up -` and a bare `dva up` agree on whether anything starts, in the two-plans-no-`default_plan` shape | verify: human — paste rc and the first non-warning line of both, run in a config with two plans, no `default_plan`, and two stack entries
- [ ] The agreement is pinned by a differential test comparing the two invocations, not by an expected string | verify: `grep -c 'func TestUpLoneDashAgreesWithABareUp' internal/cli/plan_lifecycle_test.go` returns 1 (today: 0). Bound on the test's source because `go test -run` exits 0 when it matches nothing, and on a name that does not exist yet because every existing count in that file is already non-zero and would certify itself
- [ ] No two shipped messages give contradictory accounts of what `-` is | verify: human — paste `dva up -` and `dva restart -` from the same config and confirm both call `-` the same thing
- [ ] The ruling — flag or name — is written on this card before the code changes | verify: human
- [ ] `dva up --force` and `dva up --no-wait` still run in a plan-less config, so the fix did not widen into real flags | verify: human — paste both; each must reach `[lifecycle] <entry>`. Not `-v`: `dva up` has no such flag and already answers `unknown flag "-v"`, so it would pass whatever the fix does
- [ ] `make test` passes | verify: `make test`

## References

- `internal/cli/selectors.go:58-79` — `rejectUnknownFlags`, the length test
- `internal/cli/selectors.go:130-160` — `rejectUnknownEntryNames`, the message that states the opposite rule
- `internal/cli/plan_lifecycle.go:68-78` — `requirePlanSelection`, where one surviving token means "do not ask"
- `internal/cli/plan_lifecycle.go:140-162` — `rejectSuppressedDefaultPlan`
- `internal/cli/compose.go:261` — `teardownCommon`'s dash test, the one with no length exception
- `internal/cli/restart_names_test.go` — `hintUnderDefaultPlan` pins the divergent message so this fails loudly when the ruling lands
- `tasks/_archive/087-unrecognized-stack-args-become-entry-names.md` — the defect this is the one-character remainder of
- `tasks/todo/214-a-lone-terminator-disarms-build-s-plan-selection-guard-and-builds-the-whole-stack.md` — same `requirePlanSelection` line, different token
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
