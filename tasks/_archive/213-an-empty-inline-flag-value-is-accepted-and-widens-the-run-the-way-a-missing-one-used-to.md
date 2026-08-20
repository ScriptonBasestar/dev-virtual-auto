---
id: TASK-213
title: "An empty inline flag value is accepted and widens the run the way a missing one used to"
type: bug
priority: P2
effort: S
created-at: 2026-08-20T16:55:00+09:00
source: "found by the adversarial review of TASK-211 — the fix closed the absent-value spelling and the review measured that the empty-value spelling still produces the identical harm"
scope: "internal/cli/flagtoken.go flagValue's hasValue branch and the four takeValue cases in internal/cli/compose.go. Decide the empty-value policy for all four flags at once; --var already has one and it disagrees with these."
status: done
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

- [x] `--mode=`, `-M=`, `--env=`, `--tag=` and `--exclude-tag=` each produce a non-zero exit and a message naming the flag as the user spelled it | verify: `grep -c '"--mode="' internal/cli/flagvalue_missing_test.go` returns ≥ 1 — **today 0**. Bound on the `=`-spelled table row, not on `wantFlag`, which already returns 4 and so could not fail; the first draft of this criterion made that mistake — **returns 1 at `138f030`**
- [x] A test asserts nothing ran for the empty-value spelling, not only that rc≠0 | verify: the new rows go through `restartCmd.RunE` and assert `ranMarkers` is empty, the same shape as `TestParseDvaFlagsRejectsAMissingValue` — **`grep -c 'nothing should have run' internal/cli/flagvalue_missing_test.go` returns 2, one per table**
- [x] The `--env=` reading is settled explicitly, not by default | verify: human — record in this card whether `--env=` means "no environment" or is an error, and why — **settled as an error; see "The `--env=` reading" below, with the measurement**
- [x] `--var`'s answer and these four agree, or the disagreement is documented at both sites | verify: `grep -c 'TASK-213' internal/cli/plan_lifecycle.go internal/cli/compose.go` — both non-zero, or a recorded decision here that they should differ — **both return 1; the answer is a documented disagreement, and it is documented because agreement was measured to be wrong**
- [x] `make test`, `make lint`, `make doc-check` pass | verify: run them and record the denominators, not just OK — **`make test` 9 packages ok, `internal/cli` 74.8%; `make lint` 289 files checked, 0 unformatted, 0 issues; `make doc-check` broken_links 0, oversized_docs 0, test_funcs 1128 from 172 files, run_patterns 128, unmatched_run 0, archive_cards 204, archive_missing 0, plus cilabels 5/5 and flowcheck 10 flows / 103 shell fields**

## Outcome

Fixed in `ec64bf9`, comments corrected in `138f030`.

The refusal went into `takeValue` (`compose.go`), the single funnel all four
value-taking flags already pass through — so it is one check, not four, and
`flagValue` gained no new responsibility, which is what the card asked for.
Wording is deliberately distinct: `--mode requires a non-empty value`, not
TASK-211's `requires a value`, because the user did supply a value and the
TASK-211 message would describe a different mistake than the one they made.

**Scope widened by one spelling, deliberately.** The card is written about the
`hasValue` branch, but `dva restart --mode ""` reaches the *other* flagValue
branch and produces the identical harm. Fixing only the `=` spelling would have
repeated TASK-211's own near-miss, where `--mode --` had to be taken along with
`--mode`. Both are refused by the one check, and both are in the table.

### The `--env=` reading

Settled as **an error**, and the measurement is what settles it rather than
taste. `applyEnv` opens with `if envName == "" { return nil }` — at
`compose.go:915-916` as of `cd93ed7`, and at `886-887` on `origin/master` before
this change. Both numbers are right for their own tree, which is exactly why the
commit is named beside them: a bare line number would have gone stale inside this
card, and TASK-208 exists because five comments did that.

So an empty env is not a spelling of "no environment"; it is *identical to not
passing the flag at all*, which is already spelled by omitting the flag.
Rejecting removes no capability.

Corpus check, with a positive control so the zero is not vacuous:
`git grep -n -- '--env='` finds 6 files (the control fires), while
`git grep -nE -- '--env=($|[[:space:]"\x27])'` returns **0** outside the new
test. An independent sweep run in parallel by a second session reached the same
verdict against a larger denominator — 89 `--env` occurrences repo-wide, 8 hits
for the empty-inline pattern, every one of them this card, the new test row, or
a comment quoting *`kubectl run --env=[]`*, which is another tool's help text.
It also checked the reliance vector I had not thought to check, and it is the one
that would matter in real use: no script anywhere passes a selector flag a shell
variable (`--env "$VAR"`), which is how an empty value actually reaches a CLI —
0 hits.

One detail sharpens the verdict. `--mode=` is not merely *equivalent* to absent:
`applyDefaultMode` (`compose.go:947-949` at `cd93ed7`) reads `if mode != "" ||
c.DefaultMode == "" { return mode, false }`, so an empty mode gets `default_mode`
substituted. A "clear the mode" reading of `--mode=` would produce the opposite
of clearing.

### Why `--var` does not follow, and must not

The card asked whether the four flags and `--var` should agree. They cannot, and
the reason is measured, not argued. Probing `setPlanVar` directly:

| call | result |
|---|---|
| `setPlanVar(m, "")` | `invalid --var format "", expected KEY=VAL` |
| `setPlanVar(m, "x")` | `invalid --var format "x", expected KEY=VAL` |
| `setPlanVar(m, "=v")` | `invalid --var format "=v", expected KEY=VAL` |
| `setPlanVar(m, "K=")` | accepted, sets `K` to `""` |
| `setPlanVar(m, "K=v")` | accepted, sets `K` to `"v"` |

The middle row is the one that decides it, and it is not the row I first
measured: `"x"` is **non-empty and still rejected, with the same message**. So
the branch is not testing emptiness at all. My first draft rested on the weaker
pair (`""` rejected, `"K="` accepted), which is consistent with an empty-value
policy that simply looks one level deeper; the `"x"` row rules that reading out.
It came from the parallel session's sweep, not from me.

So `--var=`'s rejection is a **KEY=VAL format check, not an empty-value policy**,
and `--var` deliberately accepts an empty *value* — setting a variable to the
empty string is a real thing to want. Adopting "an empty value is an error" there
would break `--var=K=`. The units differ: a scalar that names nothing versus a
pair whose halves are judged separately. Recorded at both sites
(`plan_lifecycle.go` `setPlanVar`, `compose.go` `takeValue`) so neither is later
"fixed" into the other.

## How this was verified

**The defect was measured before the fix, which is the only way to know the test
could see it.** Pre-fix run: 7 rejection rows failed with `want an error, got
nil`, the control already passed. A run where everything fails is
indistinguishable from a broken harness.

The first control was wrong and the run caught it: `--mode=dev` failed on a
*correct* build, because this fixture declares no per-entry modes, so mode
filtering drops s1 and s2 (`no lifecycle entries matched filters`). That control
was reporting on the fixture, not on the fix. Replaced with `--env=dev` — one
spelling away from the missing table's control, so the only difference between a
passing control and a passing rejection row is the `=`.

Five sabotages, each aimed at a different mechanism:

| # | sabotage | rows failed | signature |
|---|---|---|---|
| S1 | remove the empty check | 7 | `want an error, got nil` |
| S2 | hardcode `--mode` in the message | 5 | `does not name -M` / `--env` / `--tag` / `--exclude-tag` |
| S3 | reuse TASK-211's wording | 7 | `does not say the value must be non-empty` |
| S4 | check only when `hasValue` | 2 | exactly the two `--mode ""` / `--tag ""` rows |
| S5 | refuse every inline value | 1 + 8 pre-existing | the control, plus `compose_flags_test.go` |

S1 and S3 fail the same count, so the count alone does not separate them; the
*reason* does (`flagvalue_missing_test.go:115` vs `:122`), which is why the table
records signatures and not just numbers. S4 is the one that pays for the widened
scope — it fails exactly the two next-token rows and nothing else, so those rows
are demonstrably load-bearing rather than decorative.

**S5 disproved a claim this task had written.** The control's comment said it was
"the only thing standing between the fix and" a build that refuses every inline
value. S5 built that exact build and failed the control *plus eight pre-existing
tests* (`EqualsSyntax`, `ShortEqualsSyntax`, `TagEqualsFormat`, `TagsEqualsFormat`,
`ExcludeTagEquals`, `ExcludeTagsEquals`, `IncludeTagsCommaSeparated`,
`ExcludeTagsCommaSeparated`, all eight located by name in
`compose_flags_test.go`). The claim had been written before it was measured. It
was replaced in `138f030` with what the row actually adds — those eight stop at
`parseDvaFlags`, this one goes through `restartCmd.RunE` and asserts the stack
bounced — because a "this is the only test that catches X" comment is precisely
what a later refactor cites when deleting the row.

## An adjacent axis, measured and closed here

`--tag=a,,b` yields `includeTags = ["a","","b"]`, which *looks* like the same
family and is not a defect: `filterByTags` (`lifecycle/orchestrator.go`) builds a
set and matches with `hasAnyTag`, so the empty element can only match an entry
that declares an empty tag. It behaves as `--tag=a,b`. No follow-up card — this
paragraph exists so the next reader does not re-open it.

## What this card does not close

`takeValue` refuses an empty value; it does not refuse a *whitespace-only* one.
That was left open on purpose, and the reason is measured rather than assumed —
probed on the same fixture, `restart --mode=" "` returns `mode ' ' not found. No
modes defined in dva.yml under 'modes:'` and ran nothing. So it is already a
loud failure of a different kind (an unknown value, not an absent one), not the
silent widening this card is about, and trimming in `takeValue` would move the
report away from the code that owns mode names. Not carded; recorded here so
the omission is deliberate rather than overlooked.

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
