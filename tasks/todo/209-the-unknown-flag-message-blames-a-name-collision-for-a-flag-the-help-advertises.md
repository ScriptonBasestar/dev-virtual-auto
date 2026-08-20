---
id: TASK-209
title: "The unknown-flag message blames a name collision for a flag the help advertises"
type: bug
priority: P3
effort: S
created-at: 2026-08-20T14:20:00+09:00
source: "found by the adversarial review of TASK-198's guard — the rejection is right, the explanation is not"
scope: "internal/cli/selectors.go rejectUnknownFlags message construction, and the restart caller that supplies its known list. Rejection behaviour unchanged; only what the error says."
status: todo
---

# Task 209: The unknown-flag message blames a name collision for a flag the help advertises

## Summary

`rejectUnknownFlags` builds one explanation for every rejected token, and it is
the wrong one whenever the token is a real flag that simply belongs to a
different path:

```
$ dva restart --no-wait
ERROR: unknown flag "--no-wait" for "dva restart"
       → a stack entry name cannot start with "-", so this was read as one and matched nothing
       → accepted here: --mode, -M, --env, -E, --tag, --tags, -T, --exclude-tag, ...
```

`--no-wait` is not a typo and was not mistaken for a service name. It is a flag
`dva restart --help` lists four lines above "Stack usage", and it is refused
because this invocation is restarting the stack rather than a plan. The user is
told the parser confused their flag for a name, which is not what happened.

TASK-198 narrowed the gap from the other side by marking both flags `(plan
only)` in the help and stating the rule there. The message itself still
contradicts it.

## Second, smaller half: the `=` form is quoted whole

```
$ dva restart --var=FOO=bar
ERROR: unknown flag "--var=FOO=bar" for "dva restart"
```

`up` splits this at `internal/cli/compose.go:151` (`strings.HasPrefix(a,
"--var=")`); `restart` has no such case, so the value lands inside what the
message calls the flag name. That also defeats `similarTo`, which measures edit
distance against the advertised list (`internal/cli/selectors.go`) — `--var=FOO=bar`
is nowhere near `--var`, so no "Did you mean?" is offered for the one spelling
most likely to be a real mistake.

## What to change

Not the rejection. Both flags are correctly refused on the stack path: it
hardcodes `Wait: true` and has no plan variables, so accepting them would mean
accepting and ignoring, which `rejectUnknownFlags`' own contract forbids
(*"a command cannot advertise a flag it consumes and then ignores"*).

What is missing is a third category between "accepted here" and "unknown". A
flag can be **known to this command but not to this path**, and the message has
no way to say so. Options, smallest first:

1. Give `rejectUnknownFlags` an optional `elsewhere []string` and emit
   `→ --no-wait applies only when restarting a plan` when the token matches it.
2. Let the caller pass a per-flag explanation map, which generalises to any
   future path-scoped flag.
3. Leave the helper alone and pre-empt these two flags in `restartCmd` with
   their own error before the guard runs.

(1) is the least machinery and keeps the single-message shape. (3) duplicates
the flag list into a second place, which is what the helper's `known`-supplied-
by-caller design exists to avoid.

Trim the `=` value before both the message and `similarTo` regardless of which
is chosen; that half is independent.

## Completion Criteria

- [ ] A test pins the plan-only wording for `dva restart --no-wait`, so the message stops blaming a name collision | verify: `grep -rc 'func TestRejectUnknownFlagsExplainsPathScopedFlags' internal/cli/ | grep -v ':0'` names one file (today: no file matches)
- [ ] That test asserts what the message must NOT say, not only what it must | verify: `grep -A25 'func TestRejectUnknownFlagsExplainsPathScopedFlags' internal/cli/*_test.go | grep -c 'cannot start with'` returns ≥ 1 (today: 0 — a wording test that only checks for the new sentence passes while the wrong one is still printed beside it)
- [ ] The `=` form is split before the message and before `similarTo` | verify: `grep -rc 'func TestRejectUnknownFlagsSplitsFlagValue' internal/cli/ | grep -v ':0'` names one file (today: no file matches)
- [ ] `dva restart --var=FOO=bar` offers `--var` as a suggestion | verify: human — run it and read the "Did you mean?" block
- [ ] Nothing that was rejected is now accepted | verify: `go test ./internal/cli/ -count=1`
- [ ] `make test` passes | verify: `make test`

## References

- `internal/cli/selectors.go` — `rejectUnknownFlags`, the single message shape, and `similarTo`
- `internal/cli/flagtoken_test.go`, `internal/cli/manifest_static_commands_test.go` — where the helper is exercised today; there is no `selectors_test.go`, so pick one of these or add it
- `internal/cli/compose.go:151` — `up`'s `--var=` split, the form `restart` lacks
- `internal/cli/compose.go` — `restartCmd`'s Long help, where TASK-198 added the `(plan only)` markers this message contradicts
- `tasks/todo/198-restart-reports-success-on-a-typo-d-flag-while-doing-nothing.md` — the card whose review found this

## Technical Notes

`similarTo` uses a Levenshtein threshold of 2 against the advertised list only.
Any fix that widens the advertised list to make the suggestion work would also
widen what the message claims is accepted, which is the trade the helper's
contract note is warning about. The suggestion pool and the accepted list are
the same slice today; separating them is part of option (1) and (2) above and
should be stated explicitly if chosen.
