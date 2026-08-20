---
id: TASK-210
title: "The flag terminator is refused as a flag that suppresses the default plan"
type: bug
priority: P3
effort: S
created-at: 2026-08-20T15:30:00+09:00
source: "found while fixing the HIGH finding of TASK-207's adversarial review — the fix restored the terminator/bare identity on the stack route and could not restore it here"
scope: "internal/cli/plan_lifecycle.go rejectSuppressedDefaultPlan, shared by up/down/stop/restart. The refusal for real flags is correct and stays; only the terminator's classification is in question."
status: todo
---

# Task 210: The flag terminator is refused as a flag that suppresses the default plan

## Summary

In a config that names a `default_plan`, the flag/name separator is reported as
a flag:

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

Identical, so this is pre-existing and not a TASK-207 regression. TASK-207 is
where it became visible: that card ruled `dva restart --` means the same as a
bare `dva restart`, and its fix makes the two agree on the stack route — the
plan-less shape and the several-plans-no-default shape both match now. This
shape is the one remaining exception, and it is refused before restart's own
code runs.

## Why TASK-207 did not fix it

`rejectSuppressedDefaultPlan` is shared by `up`, `down`, `stop` and `restart`.
Narrowing it changes all four, and on the other three the outcome for `--` is
already decided elsewhere: `dva up --` is rc=1 `unknown flag "--"` from
`rejectUnknownFlags`, because `up` takes no positional names and the surviving
terminator is exactly what that guard is there to catch. So the change is
plausibly message-only for three verbs and behavioural for one — but "plausibly"
is not a measurement, and making it inside a card scoped to restart's name guard
would have decided three commands by accident. That is the same trap TASK-198
recorded when it deferred the unmatchable-name class to TASK-207.

## What to change

Smallest form: skip a leading terminator before the dash test, so the helper
classifies what follows it instead of the separator itself.

```go
args = dropFlagTerminator(args)   // restart-local today; would move to a shared file
if len(args) == 0 {
	return nil
}
```

Then `dva restart --` reaches `detectPlanRoute` with no args and runs the default
plan, identical to a bare invocation, and `dva restart -- p1` still names a plan
rather than being read as a suppressing flag.

Alternative, if the behaviour is judged correct: keep the refusal and fix only
the wording, since "flags suppress" is false for a token that is not a flag.
Either way the claim in `restartCmd`'s long help and in USAGE.md — that
`dva restart --` is the empty name list — needs the default-plan exception
stated, which is why TASK-207 states it there today.

## Completion Criteria

- [ ] A fixture that actually declares a default plan exists, not only prose about one | verify: `grep -c 'default_plan: ' internal/cli/restart_names_test.go` returns ≥ 1 (today: 0 — the phrase appears 5 times in that file, every one of them the words "no default_plan" in a shape label, which is why this binds on the YAML key and not on the word)
- [ ] The differential test covers that fixture, so the ruling is measured against a bare invocation rather than hardcoded | verify: `grep -A12 'func TestRestartBareTerminatorMeansABareRestart' internal/cli/restart_names_test.go | grep -c 'DefaultPlan'` returns ≥ 1 (today: 0)
- [ ] Whatever is ruled, `up`, `down` and `stop` are measured on the same fixture and the result recorded on this card | verify: human — run all four verbs with `--` against a default-plan config and paste rc + what ran
- [ ] `dva restart -- p1` still selects the plan | verify: human — run it and confirm only p1's entries restart
- [ ] `make test` passes | verify: `make test`

## References

- `internal/cli/plan_lifecycle.go:106-124` — `rejectSuppressedDefaultPlan`, the dash test
- `internal/cli/compose.go` — `restartCmd`'s `dropFlagTerminator` call and the plan gate beside it, which carry a pointer here
- `internal/cli/restart_names_test.go` — `TestRestartBareTerminatorMeansABareRestart`, the differential test this shape is missing from
- `tasks/_archive/207-restart-exits-0-on-an-unknown-service-name-and-the-test-pinning-it-cites-a-deleted-command.md` — the ruling whose identity claim this shape contradicts

## Technical Notes

`dropFlagTerminator` is deliberately restart-local (`internal/cli/selectors.go`),
because `parseDvaFlags` keeps the terminator on purpose for callers that take no
positional names. A fix here would be the first second caller, so the helper's
comment — which argues for locality — has to be revisited rather than ignored.
