---
id: TASK-213
title: "An empty inline flag value is accepted and widens the run the way a missing one used to"
type: bug
priority: P2
effort: S
created-at: 2026-08-20T16:55:00+09:00
source: "found by the adversarial review of TASK-211 — the fix closed the absent-value spelling and the review measured that the empty-value spelling still produces the identical harm"
scope: "internal/cli/flagtoken.go flagValue's hasValue branch and the four takeValue cases in internal/cli/compose.go. Decide the empty-value policy for all four flags at once; --var already has one and it disagrees with these."
status: todo
---

# Task 213: An empty inline flag value is accepted and widens the run the way a missing one used to

## Summary

TASK-211 made `dva restart --mode` refuse instead of silently running the whole
stack. `dva restart --mode=` still runs the whole stack and exits 0.

`splitFlagToken` returns `("--mode", "", true)` for `--mode=`
(`internal/cli/flagtoken.go:49-51`), and `hasValue` short-circuits `flagValue`
before it can report anything:

```go
func flagValue(args []string, i, end int, value string, hasValue bool) (v string, consumed int, ok bool) {
	if hasValue {
		return value, 0, true   // <- "" is a value as far as this branch is concerned
	}
```

So `takeValue` never fires, `mode` is set to `""`, and `""` is what `mode` holds
when no `--mode` was typed at all. Measured on the TASK-211 fixture (`s1`/`s2`,
no plans), branch `celee__fix__211-flag-missing-value`:

| invocation | result |
|---|---|
| `restart --mode=` | rc=0, s1 and s2 both bounced — the whole stack |
| `restart --env=` | rc=0, the whole stack |
| `restart --tag=` | rc=0, `includeTags=[""]`, nothing ran |
| `restart --exclude-tag=` | rc=0, `excludeTags=[""]` |
| `restart -M=` | rc=0, `mode=""`, no error |

Row 1 is verbatim the harm TASK-211's summary describes: *"A caller who wrote
`--mode` meant to narrow the run. They got the widest possible run, reported as
success."* Row 3 is its mirror — an unmatchable empty tag, so nothing runs, also
reported as success.

## Why this is a separate card and not TASK-211 reopened

Different axis. TASK-211 is *no value to take*; this is *a value that is there
and is empty*. They meet at the same outcome but not in the same code: TASK-211
lives in the `i+1 < end` branch, this lives in the `hasValue` branch above it,
and no fix to one reaches the other. TASK-211's tests pass unchanged with this
open, which is correct — they pin what they measured.

What TASK-211 should not be read as claiming is that the flag-missing-a-value
class is closed. It closed two spellings (`--mode`, `--mode --`) of three.

## What to change

Decide the policy once, for all four value-taking flags, and note that this repo
already contains two answers that disagree:

- `--var=` **rejects**. `plan_lifecycle.go:198` routes it to `setPlanVar`, which
  errors at `:231-233` with `invalid --var format "", expected KEY=VAL`.
- `--mode=` / `--env=` / `--tag=` / `--exclude-tag=` **accept**.

TASK-211's own "What to change" nominates `--var` as *"the shape the four cases
here should copy"*. It copied `--var`'s bare-flag branch and not its
inline-value branch, which is how the disagreement survived the fix.

Rejecting looks right for all four — an empty mode, env or tag names nothing —
but check first whether anything relies on `--env=` as a way to spell "no
environment", because that is the one where an empty value has a plausible
reading. If it does, the answer is an explicit spelling for it, not silence.

Where to put it matters as much as what to do. `flagValue`'s `hasValue` branch
is shared by all four; `takeValue` is where the flag's name is known and where
TASK-211 argued the reporting belongs. Same argument applies here — put it in
`takeValue`, and give `flagValue` no new responsibility.

## Completion Criteria

- [ ] `--mode=`, `-M=`, `--env=`, `--tag=` and `--exclude-tag=` each produce a non-zero exit and a message naming the flag as the user spelled it | verify: `grep -c '"--mode="' internal/cli/flagvalue_missing_test.go` returns ≥ 1 — **today 0**. Bound on the `=`-spelled table row, not on `wantFlag`, which already returns 4 and so could not fail; the first draft of this criterion made that mistake
- [ ] A test asserts nothing ran for the empty-value spelling, not only that rc≠0 | verify: the new rows go through `restartCmd.RunE` and assert `ranMarkers` is empty, the same shape as `TestParseDvaFlagsRejectsAMissingValue`
- [ ] The `--env=` reading is settled explicitly, not by default | verify: human — record in this card whether `--env=` means "no environment" or is an error, and why
- [ ] `--var`'s answer and these four agree, or the disagreement is documented at both sites | verify: `grep -c 'TASK-213' internal/cli/plan_lifecycle.go internal/cli/compose.go` — both non-zero, or a recorded decision here that they should differ
- [ ] `make test`, `make lint`, `make doc-check` pass | verify: run them and record the denominators, not just OK

## References

- `internal/cli/flagtoken.go:121-129` — `flagValue`; the `hasValue` branch is the one this card is about
- `internal/cli/compose.go:782-810` — `takeValue` and TASK-211's argument for reporting there
- `internal/cli/plan_lifecycle.go:231-233` — `--var`'s empty-value rejection, the model TASK-211 nominated
- `internal/cli/flagtoken_test.go:22` — pins `--mode=` at the `splitFlagToken` grammar level only, which says nothing about whether an empty value is acceptable; do not mistake it for coverage
- `tasks/todo/211-a-stack-flag-missing-its-value-is-dropped-and-the-command-runs-as-if-unwritten.md` — the card whose review measured this

## Technical Notes

`--mode=` and a missing `--mode` are indistinguishable downstream: both leave
`mode == ""`, and `applyDefaultMode` then supplies the default. That is why the
empty spelling produces the *widest* run rather than an error — it does not look
like a mistake to any code below `parseDvaFlags`. Any fix therefore has to act
in `parseDvaFlags` or above; there is no later point where the information still
exists.
