---
id: TASK-211
title: "A stack flag missing its value is dropped and the command runs as if unwritten"
type: bug
priority: P2
effort: S
created-at: 2026-08-20T15:35:00+09:00
source: "found by the adversarial review of TASK-207 — a second spelling reached this pre-existing hole, which is how the hole was noticed"
scope: "internal/cli/compose.go parseDvaFlags value-taking cases and internal/cli/flagtoken.go flagValue. All 12 parseDvaFlags callers are affected, not restart alone."
status: todo
---

# Task 211: A stack flag missing its value is dropped and the command runs as if unwritten

## Summary

`flagValue` (`internal/cli/flagtoken.go:114-122`) returns `ok=false` when a
value-taking flag is the last token, and every caller in `parseDvaFlags`
(`internal/cli/compose.go:762-780`) is written as

```go
case "--mode", "-M":
	if v, n, ok := flagValue(args, i, end, value, hasValue); ok {
		mode = v
		i += n
	}
```

with no `else`. The token is not stored, and — because the recognised cases do
not append to `filtered` — it is not forwarded either. It vanishes, and the
command proceeds as though the flag had never been typed.

Measured against `8c48687`, fixture with `s1`/`s2` and no plans:

```
dva restart --mode        rc=0   s1 and s2 both stopped and started
dva restart               rc=0   s1 and s2 both stopped and started
```

A caller who wrote `--mode` meant to narrow the run. They got the widest possible
run, reported as success. The same holds for `--env`, `--tag`, `--exclude-tag`
and `--var`, and for every command that parses its flags this way.

## Why it surfaced now

TASK-207 made `restart` consume the `--` terminator, which added a second
spelling that reaches the same hole:

| invocation | master `8c48687` | TASK-207 branch |
|---|---|---|
| `restart --mode` | rc=0, whole stack | rc=0, whole stack |
| `restart --mode --` (no plans) | rc=0, nothing ran | rc=0, whole stack |
| `restart --mode --` (plans, no default) | rc=0, nothing ran | rc=1, refused |

The middle row is a behaviour change owned by TASK-207 and is the consistent
one — `--mode --` now agrees with `--mode`, whose outcome was already this. The
row that should not exist is the first, on both binaries.

## What to change

Make a missing value an error, in `parseDvaFlags` rather than in `flagValue`:
the helper is also used where consuming the next token is optional, and its
`ok=false` return is the honest report. The caller is the code that knows a
value was required.

```go
case "--mode", "-M":
	v, n, ok := flagValue(args, i, end, value, hasValue)
	if !ok {
		return ..., fmt.Errorf("%s requires a value", name)
	}
```

`parseDvaFlags` already has the pattern for this: `takeBool` sets `err` on a
malformed boolean and TASK-172's comment argues exactly this point — *"no caller
can take over this job"* — for the flag whose meaning only this code knows.
Missing-value is the same shape as malformed-value and should be reported the
same way.

Check before changing: whether any caller depends on the silent drop, e.g. a
passthrough command that wants an unterminated flag forwarded to the external
tool. The 12 call sites are enumerated in TASK-208's correction (the real count
is 6 for the fallthrough class; confirm which set applies here rather than
reusing either number).

## Completion Criteria

- [ ] A missing value is an error for every value-taking case in `parseDvaFlags` | verify: `grep -c 'requires a value' internal/cli/compose.go` returns ≥ 1 (today: 0)
- [ ] A test pins it for at least `--mode` and one repeatable flag such as `--tag` | verify: `grep -rc 'func TestParseDvaFlagsRejectsAMissingValue' internal/cli/ | grep -v ':0'` names one file (today: no file matches)
- [ ] The test asserts nothing ran, not only that rc≠0 | verify: `grep -A30 'func TestParseDvaFlagsRejectsAMissingValue' internal/cli/*_test.go | grep -c 'ranMarkers'` returns ≥ 1 (today: 0 — an rc-only test passes on a command that errors after acting)
- [ ] Passthrough callers are checked for a dependence on the drop, and the finding recorded here | verify: human — name each caller and say whether it forwards or consumes
- [ ] `make test` passes | verify: `make test`

## References

- `internal/cli/flagtoken.go:114-122` — `flagValue`, whose `ok=false` is correct and ignored
- `internal/cli/compose.go:762-780` — the four value-taking cases with no `else`
- `internal/cli/compose.go:748-756` — `takeBool`, the pattern to follow, and TASK-172's argument for reporting here
- `tasks/_archive/207-restart-exits-0-on-an-unknown-service-name-and-the-test-pinning-it-cites-a-deleted-command.md` — the card whose review measured this
- `tasks/todo/208-five-comments-size-the-flag-fallthrough-class-at-twelve-call-sites-the-real-count-is-six.md` — before quoting a call-site count, read this

## Technical Notes

`dvaFlagEnd` sets `end` at the first `--`, so a flag immediately before the
terminator is "last" for `flagValue`'s purposes even though tokens follow it.
That is why `--mode --` reaches the same drop as a trailing `--mode`, and any
fix has to treat both, not only the end-of-argv case.
