---
id: TASK-223
title: "`dva` sorts a lone dash ahead of the command name, so the shorthand form runs a different interaction than the one named"
type: bug
priority: P2
effort: S
created-at: 2026-08-21T14:30:00+09:00
source: "found by an audit of the six sites TASK-218 left alone; it refuted TASK-218's own justification for leaving this one"
scope: "internal/cli/root.go — isFlag (:247) and the flags-first partition in Execute (:210). Its pins move with it: root_test.go's `{\"-\", true}` row and TestDashPredicatesDisagreeOnPurpose. Not the plan-name or entry-name slots — TASK-218 settled those."
status: todo
---

# Task 223: a lone dash is sorted ahead of the command name

## Summary

`Execute` rewrites `dva <interaction> <args...>` into `dva run <flags...>
<nonflags...>`, partitioning by `isFlag`, which answers `true` for a lone `-`.
So `-` is moved **in front of** the command name. With an interaction named `-`
declared, `dva greet -` becomes `run - greet`: dva runs `-` and passes `greet`
to it as an argument.

The user named `greet`. Something else ran. Exit code 0.

## Measured

Fixture: `dva.yml` declaring two interactions, `greet` and `-`, each echoing its
own marker and its arguments. `dva validate` exits 0 on it — nothing in dva
validates the charset of an interaction name. Binary built from
`celee__fix__217-218-terminator-and-dash-guards`, 2026-08-21.

```
dva greet            rc=0   RAN_GREET_with=[]
dva greet -          rc=0   RAN_DASH_with=[] greet      ← wrong interaction, rc=0
dva run greet -      rc=0   RAN_GREET_with=[] -         ← explicit form: correct
dva greet -v         rc=1   ERROR: unknown shorthand flag: 'v' in -v
dva -                rc=0   (root help)
dva run -            rc=0   RAN_DASH_with=[]
```

Two rows carry the defect. `dva greet -` and `dva run greet -` are documented as
the same command, and they disagree. And the sugar form's disagreement is not a
refusal — it is an action.

Without an interaction named `-`, the same rewrite produces ``ERROR: command `-`
not recognized!`` — a confusing message that blames the wrong token, but at
least rc=1.

## Cause

`internal/cli/root.go`:

```go
:190   if !isTopLevelCommand(firstArg) && !isFlag(firstArg) {
:210           if isFlag(a) {
:214           } else { nonFlags = append(nonFlags, a) }
:217   args = append([]string{"run"}, append(flags, nonFlags...)...)

:247   func isFlag(s string) bool { return len(s) > 0 && s[0] == '-' }
```

The two call sites are not the same slot:

- `:190` gates the interaction lookup. A wrong answer here only *hides* an
  interaction named `-` — `dva -` prints root help instead of running it. Costly
  only to a config nobody has.
- `:210` classifies **every** argument, and the partition is a reordering. A
  wrong answer here changes which interaction runs.

TASK-218 left `isFlag` alone on the argument that its slot could only withhold a
shorthand. That was measured on `dva -` and `dva run -` only — the one-token
forms, which are answered at `:190`. `:210` needs two tokens to show anything,
and it was never measured. TASK-218's own card and code comments now record the
retraction.

## What to change

Undecided, and the card should not pretend otherwise. Two candidates:

1. **`isFlag` adopts the length test** (`isFlagToken`, `flagtoken.go`). One line,
   makes both slots agree with the plan-name and entry-name slots, and makes
   `dva greet -` match `dva run greet -`. Cost: `dva -` would then try to run an
   interaction named `-` rather than print root help.
2. **Leave `isFlag`, fix the partition.** `:210` only needs to move *dva's own*
   flags; a token that no dva flag matches is an argument wherever it sits. That
   is a larger change and touches every shorthand invocation, not just `-`.

Whichever lands must also answer the product question this card does not:

> Is a stack entry or interaction named `-` supported? Config validation accepts
> both today (measured). TASK-218 answered "yes" for entries by routing to them.
> If the answer is "no", the fix belongs in schema validation and all of this
> becomes a validation error instead.

## Completion Criteria

- [ ] `dva greet -` and `dva run greet -` run the same interaction, in a config that also declares an interaction named `-` | verify: human — paste both, with rc and the marker line; the two must name the same interaction
- [ ] The agreement is pinned by a differential test comparing the two spellings, not by an expected string | verify: `grep -c 'func TestSugarFormAgreesWithExplicitRun' internal/cli/root_test.go` returns 1 (today: 0). Bound on the test's source, not on `go test -run`, which exits 0 when it matches nothing
- [ ] `root_test.go`'s `{"-", true}` row and `TestDashPredicatesDisagreeOnPurpose` are updated with the argument replaced, not just the value | verify: human — both tests changed in the same commit as `root.go`, and neither is deleted
- [ ] The product ruling on names like `-` is written on this card before the code changes | verify: human — a `## Resolution` section stating it
- [ ] `dva greet --debug` and `dva greet -M dev` still reach the interaction with the flag applied, so the fix did not stop hoisting real flags | verify: human — paste both
- [ ] `make test` passes | verify: `make test`

## References

- `internal/cli/root.go:190` — the lookup gate; a wrong answer here only hides an interaction
- `internal/cli/root.go:210` — the flags-first partition; a wrong answer here reorders and acts
- `internal/cli/root.go:247` — `isFlag`, no length test
- `internal/cli/root_test.go:20` — the `{"-", true}` row that pins today's answer
- `internal/cli/flagtoken.go:89` — `isFlagToken`, the same predicate with the length test, and the doc comment carrying the measured rows above
- `internal/cli/dash_name_test.go` — `TestDashPredicatesDisagreeOnPurpose`, which fails on purpose when this card lands
- TASK-218 — settled the plan-name and entry-name slots; its `### Correction` section is where this card came from
