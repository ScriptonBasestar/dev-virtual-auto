---
id: TASK-207
title: "`dva restart` exits 0 on an unknown service name, pinned by a test citing a deleted command"
type: bug
priority: P2
effort: S
created-at: 2026-08-20T14:06:00+09:00
source: "found by the TASK-198 corpus sweep — the name-shaped twin of 198's flag defect, surviving one line away from the guard 198 added"
scope: "internal/cli/compose.go restartCmd RunE and internal/cli/restart_names_test.go TestRestart_UnknownNameTouchesNothing. Decide the ruling first; the code change is one call to an existing helper."
status: todo
---

# Task 207: `dva restart` exits 0 on an unknown service name, pinned by a test citing a deleted command

## Summary

TASK-198 closed `dva restart --no-wat` (unknown **flag** read as a service name,
exit 0, nothing restarted). The same user-visible signature — exit 0, nothing
done, no error — survives through the name half of the same path, and `restart`
is the only lifecycle verb still exposed to it.

Measured against the **post-198** binary, so this is not a regression 198
introduced and not something 198 fixes:

```
restart zzznosuchservice     rc=0   [warn] no lifecycle entries matched filters
up      zzznosuchservice     rc=1   unexpected argument 'zzznosuchservice'
down    zzznosuchservice     rc=1   'dva down' downs all declared entries. Name a plan instead
stop    zzznosuchservice     rc=1   'dva stop' stops all declared entries. Name a plan instead
```

Identical result in a plans-present fixture, and via a tag filter matching
nothing (`restart -T zzznosuchtag` → rc=0). `restart` is the only verb that
legitimately takes positional service names, which is exactly why it is the only
one that can misread a typo as one — the same structural reason it was the
outlier in TASK-198.

`rejectUnknownFlags` cannot catch this: it fires only on a leading dash
(`internal/cli/selectors.go:57`, `len(a) >= 2 && strings.HasPrefix(a, "-")`).
The guard TASK-198 added sits one line above the code that discards the
unmatched name.

## The four tokens this covers

An adversarial review of TASK-198 found three more ways into the same rc=0,
nothing-done outcome. They are not separate defects — they are the same
unmatchable-name path reached by different tokens, which is why they belong in
one ruling instead of three:

```
restart zzznosuchservice       rc=0   nothing ran      an ordinary typo
restart --                     rc=0   nothing ran      terminator, no names follow
restart -                      rc=0   nothing ran      too short for the dash guard (len < 2)
restart -- --no-wat s1         rc=0   s1 ran           after `--` a flag IS a name; typo discarded
```

The last row is the worst of the four and matches what TASK-198 calls the worse
half of its own defect: something *does* happen, and the argument the user typed
is silently dropped. The bare `-` case is shared with `up` and `down` and is
pre-existing in all three.

TASK-198 deliberately declined to settle any of these. Its guard leaves `--` in
the name list rather than removing it, precisely so that this card decides what
an unmatchable name means instead of inheriting an answer from slice arithmetic
— an earlier draft removed the token and turned `dva restart --` into a full
stack bounce, which is recorded in `TestRestartBareTerminatorChangesNothing`.

## Why this is a decision and not just a fix

The behaviour is **already pinned by a passing test**, whose stated
justification names a command that no longer exists:

```go
// TestRestart_UnknownNameTouchesNothing matches the 'dva stack up bogus-name'
// reference path: warn, change nothing, exit 0.
```

`dva stack` was removed with the `applications:` section in the intent-centric
plan refactor (`6710766`, and CLAUDE.md records the removal). The reference path
this test conforms to is gone, and the verb that replaced it — `up` — took the
opposite ruling: `up zzznosuchservice` is rc=1. So the current behaviour is not
a decision that survived the refactor; it is a decision whose premise the
refactor deleted, left unexamined because the test kept passing.

That is what makes this a card rather than a one-line change. Whichever way it
is ruled, the test comment must stop citing `dva stack up`.

## The ruling to make

Three defensible outcomes, in the order I would argue them:

1. **Reject, matching `up`.** An unknown name is a typo far more often than an
   intentional no-op, and `restart` is the last verb that disagrees with its
   siblings. Cost: `dva restart $SERVICE` in a script now fails where it used to
   warn — and note `dva restart -- "$@"` with an empty `"$@"` becomes an error
   under this reading, which is the idiom `--` exists for.
2. **Reject only when *no* name matched**, keeping a partial match (`restart s1
   zzztypo`) as a warn. Narrower, and it preserves batch invocations.
3. **Keep exit 0 deliberately**, and rewrite the test comment to say so on its
   own terms rather than by reference to a deleted command.

This card does not assume (1). It requires that one of them be chosen and that
the test's rationale be replaced either way.

Note the interaction with TASK-198's Open Question — "should the empty selection
itself be an error?" — which covers the tag-filter arm of the same exit-0 path.
Settle the name arm here; a tag filter that legitimately matches nothing is a
different case and is not in scope.

## Completion Criteria

- [ ] The ruling is stated in the card's disposition, naming which of the three outcomes was chosen and why | verify: human — read the disposition
- [ ] `TestRestart_UnknownNameTouchesNothing` no longer justifies itself by `dva stack up` | verify: `grep -c 'dva stack up' internal/cli/restart_names_test.go` returns 0 (today: 1)
- [ ] The ruling is pinned by a test named for it, so the behaviour stops being inherited from the flag guard | verify: `grep -c 'func TestRestartUnknownNameRuling' internal/cli/restart_names_test.go` returns 1 (today: 0)
- [ ] That test exercises a plans-present config too, since the stack path is reachable with plans configured | verify: `grep -A30 'func TestRestartUnknownNameRuling' internal/cli/restart_names_test.go | grep -c 'writeRestartPlanProbeConfig'` returns ≥ 1 (today: 0, the function does not exist)
- [ ] All four tokens from "The four tokens this covers" are ruled on together, not just the typo | verify: human — the disposition states the outcome for `zzznosuchservice`, `--`, `-`, and a flag after `--`; a ruling that leaves any of the four unnamed is incomplete
- [ ] `TestRestartBareTerminatorChangesNothing` is updated rather than left asserting a behaviour this card overturned | verify: human — read the test and confirm it asserts the chosen ruling for `--`; a presence grep would pass unchanged and prove nothing
- [ ] The whole cli package passes | verify: `go test ./internal/cli/ -count=1`
- [ ] Confirmed against the built binary | verify: human — rebuild and re-run the 4-verb `zzznosuchservice` table from Summary; state which rows changed
- [ ] `make test` passes | verify: `make test`

## References

- `internal/cli/compose.go` — `restartCmd` RunE, the name path below TASK-198's guard
- `internal/cli/selectors.go:57` — `rejectUnknownFlags`, and the leading-dash condition that makes it blind to this
- `internal/cli/restart_names_test.go` — `TestRestart_UnknownNameTouchesNothing`, the test with the stale rationale
- `tasks/todo/198-restart-reports-success-on-a-typo-d-flag-while-doing-nothing.md` — the flag half, and its Open Question on the empty selection
- `tasks/_archive/087-unrecognized-stack-args-become-entry-names.md` — the name-fallthrough class, filed against the removed `stack` family
- `internal/cli/restart_names_test.go` — `TestRestartBareTerminatorChangesNothing`, which pins the `--` row of the table above so this card's ruling has to be explicit about it

## Technical Notes

Both fixtures used by the sweep behave identically, which is the point of
measuring both: `detectPlanRoute` returns `ok=false` for "no plans" *and* for
"several plans, none selected", so the stack path — and this defect — is
reachable with plans configured. The plans fixture is two plans (`p1`, `p2`) and
no `default_plan`; see `writeRestartPlanProbeConfig` in the test file, added by
TASK-198.

Exit codes must be captured without a pipeline in the way (`cmd >/dev/null 2>&1;
echo $?`). `cmd | head` reports head's status, and that has been misread as rc=0
twice in this repo's measurements.
