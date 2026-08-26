---
id: TASK-215
title: "A flag typed where a value belongs is swallowed as that value"
type: bug
priority: P2
effort: M
created-at: 2026-08-20T20:05:00+09:00
source: "found by the adversarial review of TASK-213, probing what the new rules still admit. Neither TASK-213 nor TASK-214 names it, and both claimed to enumerate what was left"
scope: "`flagValue` in internal/cli/flagtoken.go, reached from `parseDvaFlags`' four value-taking arms in internal/cli/compose.go. The `--` terminator spelling is already refused by TASK-211 and is not in scope"
status: done
completed-at: 2026-08-26T11:33:00+09:00
completion-summary: "Reject recognized DVA selector flags used as values while preserving legitimate unrecognized leading-dash values."
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "dva test"
    result: "passed; internal/cli 75.4% coverage"
  - kind: automated
    command-or-step: "dva lint"
    result: "passed; 292 Go files formatted, 0 issues"
  - kind: automated
    command-or-step: "make doc-check"
    result: "passed; 279 Markdown files, 552 links, 1,147 test functions"
quality-review: pass
quality-reviewed-at: 2026-08-26T11:34:19+09:00
quality-review-evidence:
  - "recognized selector flag value-slot tests and leading-dash control passed"
  - "full dva test, dva lint, and make doc-check gates passed"
quality-review-receipt: tmp/task-management/direct/queue-run/task-215-review-receipt.json
archived-at: 2026-08-26T11:34:54+09:00
verified-at: 2026-08-26T11:34:54+09:00
verification-summary: "Recognized DVA flags are rejected as missing values before execution while unrecognized leading-dash values remain supported."
---

# Task 215: A flag typed where a value belongs is swallowed as that value

## Summary

`flagValue` takes `args[i+1]` without looking at it. A user who types the flag
and forgets the value gets the *next flag* stored as the value, silently.
Measured through `parseDvaFlags` on the current build:

| invocation | parsed as | error | what runs |
|---|---|---|---|
| `dva restart --exclude-tag --tag=x` | `excludeTags = ["--tag=x"]` | **none, rc=0** | **the whole stack** |
| `dva restart --tag --exclude-tag=x` | `includeTags = ["--exclude-tag=x"]` | **none, rc=0** | nothing |
| `dva restart --mode --env=dev` | `mode = "--env=dev"` | none | fails later, in mode lookup |
| `dva restart --tag --` | — | `--tag requires a value` | nothing — TASK-211 closed this |

Row 1 is verbatim TASK-213's title — a narrowing flag producing the widest
possible run at rc=0 — reached one token differently from the spelling TASK-213
refuses. Row 4 is the same mistake with a different next token, and it is loud,
which is what makes rows 1-2 worth carding: the terminator was closed and the
much likelier typo beside it was not.

Neither `--tag=x` nor `--exclude-tag=x` is ill-formed (TASK-213's class) or
undeclared (TASK-214's class). It is well-formed and declared — as a *flag*, in
a position where a value belongs.

## The argument against fixing it, and why the card exists anyway

`pflag` behaves identically, measured rather than assumed:

```
PFLAG ["--exclude-tag" "--tag=x"]     -> exclude="--tag=x" tag="" err=<nil>
PFLAG ["--exclude-tag" "--tag" "x"]   -> exclude="--tag"   tag="" err=<nil>
PFLAG ["--exclude-tag"]               -> err=flag needs an argument: --exclude-tag
```

So DVA is not deviating from what a Go CLI user has been trained to expect, and
"a value may begin with `-`" is a real capability in the general case. That is a
genuine argument for `wontfix`, and this card does not prejudge it.

What is not defensible is the **enumeration**: TASK-213 said what remained open
was "nothing is trimmed off values that survive", and TASK-214 said its class
"is what is left after they are gone". This spelling is in neither, so both
sentences were false when written. They are corrected now, and this card is
where the third class lives whatever is decided about it.

DVA also already diverges from pflag deliberately in this exact area — pflag
accepts `--exclude-tag=`, DVA refuses it (TASK-213) — so parity is a
consideration here, not a rule.

## What to decide

Tags, modes and env names never begin with `-` in any DVA config. If that is
accepted as an invariant, `flagValue` can refuse a next token that
`splitFlagToken` recognises as a known flag name, and the message can say so:
`--exclude-tag requires a value, got the flag --tag=x`.

Two things to settle with a measurement rather than by taste:

- **Known flags only, or anything starting with `-`?** Refusing only recognised
  names keeps `--mode -weird-but-real` working; refusing everything leading with
  `-` is simpler and catches `--tag -T`. Check whether any example in
  `examples/` or any doc uses a leading-`-` value before choosing the narrow one.
- **Does this belong in `flagValue` or in the four arms?** `flagValue` is shared
  with `takeBool`, which must keep accepting the next token unexamined.

## Completion Criteria

- [x] A flag in a value position is refused, naming both flags | verify: `grep -rn 'got the flag' internal/cli/*.go | grep -v _test` returns at least one line — **today 0, measured** — or, if the decision is `wontfix`, this card records the measurement that decided it and moves to `_archive/` with that verdict
- [x] The `--exclude-tag` direction specifically is pinned, not just the include side | verify: `grep -rc '"--exclude-tag", "--tag=' internal/cli/*_test.go | awk -F: '{s+=$2} END{print s+0}'` ≥ 1 — **today 0, measured.** The include side runs nothing and the exclude side runs everything; a test on `--tag` alone would pass on a build that still widens. Bound on the Go slice literal, not on `exclude-tag.*--tag=`, which was this criterion's first binding and already returned 1 — matching a *comment* at `flagvalue_missing_test.go:238`. A criterion satisfied by prose about the defect is satisfied by the defect
- [x] The refusal happens before anything runs | verify: the new rows go through `restartCmd.RunE` against `writeRestartProbeConfig` and assert `ranMarkers` is empty, the shape `flagvalue_missing_test.go` already uses
- [x] A value legitimately beginning with `-` still works, or the decision to forbid it is recorded | verify: human — whichever branch is taken, the card names the invariant it relies on
- [x] `make test`, `make lint`, `make doc-check` pass | verify: run them and record the denominators, not just OK

## Resolution

Only a next token whose name is one of DVA's recognized selector flags is refused.
The error names both the flag missing its value and the flag found in that slot, so
`--exclude-tag --tag=web` is diagnosed before TASK-214's unknown-tag check or any
lifecycle action. Values beginning with `-` remain supported when DVA does not own
their name; `--mode -weird-but-real` is the pinned control.

## References

- `internal/cli/flagtoken.go` — `flagValue`, which takes `args[i+1]` unconditionally; `splitFlagToken`, which already knows what a flag token looks like
- `internal/cli/compose.go` — `takeValue` / `takeList`, the four arms that would carry the new refusal
- `tasks/_archive/211-a-stack-flag-missing-its-value-is-dropped-and-the-command-runs-as-if-unwritten.md` — closed the end-of-args and `--` spellings of "forgot the value"; this is the third spelling
- `tasks/_archive/213-an-empty-inline-flag-value-is-accepted-and-widens-the-run-the-way-a-missing-one-used-to.md` — the ill-formed-value class, and the card whose enumeration this one falsifies
- `tasks/todo/214-an-unknown-tag-narrows-the-run-to-nothing-and-exits-zero.md` — the undeclared-tag class, the other half of the same enumeration

## Technical Notes

If TASK-214 lands first this becomes much quieter but does not disappear:
`--tag=x` as a tag name matches nothing, so an unknown-tag check would refuse
row 1 — with a message blaming a tag named `--tag=x` rather than a missing
value. Correct outcome, misleading diagnosis. Whichever lands second should
check the other's message rather than assume it composes.
