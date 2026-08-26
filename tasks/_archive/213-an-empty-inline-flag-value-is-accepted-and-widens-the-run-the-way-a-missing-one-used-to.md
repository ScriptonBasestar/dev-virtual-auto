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

- [x] `--mode=`, `-M=`, `--env=`, `--tag=` and `--exclude-tag=` each produce a non-zero exit and a message naming the flag as the user spelled it | verify: `/usr/bin/grep -c '"--mode="' internal/cli/flagvalue_missing_test.go` returns ≥ 1 — **today 0**. Bound on the `=`-spelled table row, not on `wantFlag`, which already returns 4 and so could not fail; the first draft of this criterion made that mistake — **returns 1 at `138f030`**
- [x] A test asserts nothing ran for the empty-value spelling, not only that rc≠0 | verify: the new rows go through `restartCmd.RunE` and assert `ranMarkers` is empty, the same shape as `TestParseDvaFlagsRejectsAMissingValue` — **`grep -c 'nothing should have run' internal/cli/flagvalue_missing_test.go` ≥ 2, one per table — 3 at `f38ad5c`.** Written as "returns 2" until the review round added a third table, which is the same trap criterion 4 fell into two lines below; an exact count in a criterion is a claim about a file's future
- [x] The `--env=` reading is settled explicitly, not by default | verify: human — record in this card whether `--env=` means "no environment" or is an error, and why — **settled as an error; see "The `--env=` reading" below, with the measurement**
- [x] `--var`'s answer and these four agree, or the disagreement is documented at both sites | verify: `/usr/bin/grep -c 'TASK-213' internal/cli/plan_lifecycle.go internal/cli/compose.go` — both non-zero, or a recorded decision here that they should differ — **both non-zero; the answer is a documented disagreement, and it is documented because agreement was measured to be wrong.** The first version of this line said "both return 1" and was already false when written down: `02ce928` had made `plan_lifecycle.go` return 2 on the same branch. Counts here are ≥-bindings for that reason — `plan_lifecycle.go` 2, `compose.go` 3, in the argument order the command uses. Written as "3 and 2" for one commit, transposed against that order and against this line's own preceding sentence, which says `02ce928` made `plan_lifecycle.go` return 2
- [x] A value that is empty *after* the split is refused too, not only one that is empty as typed | verify: `/usr/bin/grep -c 'requires non-empty tags' internal/cli/compose.go` ≥ 1 — **1 at `f38ad5c`, 0 at `1e7d476`, measured on both**. Bound on the message rather than on `takeList`'s name, so renaming the helper cannot satisfy it
- [x] Every spelling `parseDvaFlags` recognises has a rejection row, not a representative sample | verify: `for f in --mode -M --env -E --tag --tags -T --exclude-tag --exclude-tags; do /usr/bin/grep -q "\"$f=\"" internal/cli/flagvalue_missing_test.go || echo MISSING $f; done` prints nothing — **prints nothing at `f38ad5c`; at `1e7d476` it printed `-E`, `--tags`, `-T`, `--exclude-tags`**
- [x] `make test`, `make lint`, `make doc-check` pass | verify: run them and record the denominators, not just OK — **re-run after the second review round: `make test` 9 packages ok, `internal/cli` 75.0%; `make lint` 289 files checked, 0 unformatted, 0 issues; `make doc-check` broken_links 0, oversized_docs 0, test_funcs 1131 from 172 files (1130 before `TestParseDvaFlags_FirstBadFlagIsReported`), run_patterns 128, unmatched_run 0, archive_cards 205, archive_missing 0, plus cilabels 5/5 and flowcheck 10 flows / 103 shell fields.** `make test-integration` also passes, and is recorded here as *not evidence for this change*: `go list -deps -tags=integration ./internal/integration/... | grep -c 'dva/internal/cli'` returns 0, so it cannot exercise this code in either direction

## Outcome

Fixed in `ec64bf9`, comments corrected in `138f030`. **Reopened by an adversarial review and finished in `a25b38c` and `f38ad5c`** — the first fix closed part of the class it claimed to close. See "What the review found" below; the short version is that `dva restart --exclude-tag=,` still bounced the entire stack at rc=0, one character past the spelling `ec64bf9` refuses.

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

Corpus check, with a positive control so the zero is not vacuous. Every number
below is stated with the command that produces it, because the first version of
this paragraph quoted a parallel session's denominators — "89 `--env` occurrences
repo-wide, 8 hits" — which no command in the paragraph reproduces, and a review
that tried got different figures again. A number no one can re-derive is not
evidence, however green the verdict it sits beside.

Every command below is scoped `-- ':!tasks/'`, and that scoping is the finding.
The first version of this table was stamped "at `f38ad5c`" and swept the whole
repository including this file — so writing the table changed three of its own
numbers, and a review measuring at `4b73299` got 106 / 120 / 3 where the table
said 99 / 110 / 3. One row was wrong at the commit it was stamped with, because
the third `--env=` occurrence was the line this card added. Re-measuring again
while fixing that produced 109 / 124: a repo-wide count of a string that appears
in prose is not a measurement of the code, it is a measurement of how much has
been written about the code, and it moves every time anyone writes about it.

| measurement | command (all scoped `-- ':!tasks/'`) | at `4b73299` |
|---|---|---|
| files containing `--env=` (the positive control — it must fire) | `git grep -l -- '--env=' -- ':!tasks/' \| wc -l` | 4 |
| the empty-inline spelling, in code and docs | `git grep -nE -- '--env=($\|[[:space:]"\x27])' -- ':!tasks/' \| wc -l` | 2 |
| a selector flag fed a shell variable | `git grep -nE -- '--(env\|mode\|tag\|exclude-tag)[= ]"?\$' -- ':!tasks/' \| wc -l` | 0 |

The last row is the reliance vector that would matter in real use, since `--env
"$VAR"` with an unset `VAR` is how an empty value actually reaches a CLI; a
parallel session checked it and I had not thought to. It reads 0 outside
`tasks/`, and it read 1 inside `tasks/` for exactly as long as this card
contained a sentence saying there were none — a corpus sweep whose pattern
matches the prose describing the sweep. The unscoped "lines" and "occurrences"
rows are gone rather than corrected: they never bore on the verdict, and their
only stable property was drifting.

The 4-file control was previously written up as "`git grep -n -- '--env='` finds
6 files", which is a `-l` count attributed to a `-n` command, at a scope that
counted this card among the callers.

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

Five more after the review, against the second fix:

| # | sabotage | rows failed | signature |
|---|---|---|---|
| S6 | the review's own: `&& name != "--tags" && name != "--exclude-tags" && name != "-E" && name != "-T"` | 4 | exactly the four alias rows, which did not exist before this round |
| S7 | delete `takeList`'s element rule | 8 | every `,` and hole row, `--exclude-tag` ones also reporting `ran [s1_stop s1_up s2_stop s2_up]` |
| S8 | element rule on `p == ""` instead of `TrimSpace(p) == ""` | 1 | only `a blank element` (`--tag=a, ,b`) |
| S9 | delete the blank-value rule | 5 | the four blank rows plus the blank next token |
| S10 | hardcode the flag name in both new messages | 8 | `does not name` — and the survivors are exactly the rows whose flag *is* the hardcoded name |

S10 is the S2 of this round and the reason it is worth repeating: the rows that
survive a hardcoded identity are the ones that agree with it by luck, so the
count is not the finding — *which* rows lived is.

A second review then sabotaged the *second* fix and found it unpinned. `f38ad5c`
consists of moving `i += n` out of `if ok` in four near-identical case arms;
only `--mode` had a test, because `--mode` was the spelling that had leaked.
Reverting the other three left the package green — `ok internal/cli 9.2s`, zero
failures — with `["--env","","s1"]` handing `["","s1"]` to passthrough callers
again. Deleting both `if err == nil` guards, so the *last* bad flag wins instead
of the first, was also green: no row used two bad flags. Both are pinned now,
and re-sabotaged to confirm the pins fire:

| # | sabotage | rows failed | signature |
|---|---|---|---|
| S11a-d | `i += n` back inside `if ok`, one arm at a time | 1 each | exactly the subtest for that flag — `--mode`, `--env`, `--tag`, `--exclude-tag` in turn, never another |
| S12 | both `if err == nil` guards dropped (last error wins) | 1 | `TestParseDvaFlags_FirstBadFlagIsReported`: `err` names `--tag`, the flag typed second |

S11's signature is the useful part: four sabotages, four different single
failures, no overlap. A blanket failure would have proved only that the tests
run. **A fix is pinned at the granularity it was written**, and this one was
written four times — a shape that is easy to review as one change and easy to
revert as four.

The degenerate rows also use `Errorf` rather than `Fatalf` on the missing-error
branch, deliberately. With `Fatalf` the subtest returns before `ranMarkers`, so
S7 would report `want an error, got nil` for every flag alike; with the check
reached, the log separates `--exclude-tag=,` (ran everything) from `--tag=,`
(ran nothing), which is precisely the distinction the deleted section got wrong.

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

## What the review found

The section that stood here was wrong, and wrong in the most expensive way a
section can be: it examined one member of a family, generalised the verdict to
the family, and told the next reader not to re-open it. It read —

> `--tag=a,,b` yields `includeTags = ["a","","b"]`, which *looks* like the same
> family and is not a defect […] No follow-up card — this paragraph exists so the
> next reader does not re-open it.

Every clause about `--tag` is true. `filterByTags` does build a set, the empty
element does match nothing, and for the *include* side matching nothing means
running nothing. What was never measured is the complement. For `--exclude-tag`,
matching nothing means **excluding** nothing:

| spelling | error | what ran |
|---|---|---|
| `--exclude-tag=,` | none, rc=0 | `s1_up s1_stop s2_up s2_stop` — the whole stack |
| `--exclude-tags=,` | none, rc=0 | the whole stack |
| `--exclude-tag=a,,b` | none, rc=0 | the whole stack |
| `--exclude-tag=" "` | none, rc=0 | the whole stack |
| `--tag=,` | none, rc=0 | nothing |
| `--tag=" "` | none, rc=0 | nothing |

The first four rows are verbatim the harm named in this card's own title,
reachable one character past the spelling `ec64bf9` refuses, under a heading
saying the axis was closed. The last two are the quieter half — a narrowing flag
that silently runs nothing and reports success is the same defect as TASK-211,
pointed the other way.

The cause is a guard on the wrong side of a transform. `if v == ""` ran before
`strings.Split(v, ",")`, so it validated the string the user typed rather than
the values the program uses; `,` is non-empty as a string and empty as a list.
`takeList` (`compose.go`) now applies the rule after the split, and `takeValue`
also refuses a blank value, which is what closes the `" "` rows.

**Three further findings from the same review, all acted on:**

- The "What this card does not close" section measured whitespace on `--mode`
  alone and asserted the conclusion for four flags. Per flag it was: `--mode` and
  `--env` loud, `--tag` silent-and-empty, `--exclude-tag` silent-and-everything.
  Rewritten below, and the behaviour it described is now fixed rather than
  documented.
- The rejection table listed `--mode, -M, --env, --tag, --exclude-tag` and had no
  row for `-E`, `-T`, `--tags` or `--exclude-tags`. Guarding `takeValue` with
  `&& name != "--tags" && name != "--exclude-tags" && name != "-E" && name != "-T"`
  left the package fully green while `dva restart -E=` ran the whole stack. Four
  rows added; that sabotage is S6 below.
- `parseDvaFlags(["--mode","","s1"])` returned `filtered = ["","s1"]` — a rejected
  flag's value reaching what passthrough callers hand to docker. `a25b38c`'s
  comment called it unreachable, which is a property of the six callers rather
  than of this function. Fixed in `f38ad5c` by advancing `i` whether or not the
  value was accepted, and pinned by
  `TestParseDvaFlags_RejectedValueIsStillConsumed`.

## What this card does not close

Whitespace-only values *are* closed now, and the way that changed is worth
recording. The section that stood here left them open on a measurement of
`--mode=" "` — which returns `mode ' ' not found` and runs nothing, a loud
failure of a different kind — and then stated the conclusion for all four flags.
Probed per flag afterwards:

| spelling | before | |
|---|---|---|
| `--mode=" "` | `mode ' ' not found`, ran nothing | loud |
| `--env=" "` | `env ' ' not found. Available: dev`, ran nothing | loud |
| `--tag=" "` | no error, rc=0, ran nothing | silent |
| `--exclude-tag=" "` | no error, rc=0, **ran the whole stack** | silent widening |

Two of four, not four of four. The generalisation is the defect; the single
measurement was correct. All four are refused in `takeValue` now, with a message
distinct from the empty one (`requires a non-blank value, got " "`) so a fix for
either cannot satisfy the other's rows. "Whitespace" here means
`unicode.IsSpace`, about 25 runes; `--tag=<U+200B>` is a zero-width space, is not
one of them, and passes as a well-formed tag that nothing declares — TASK-214's
class, noted there.

Two gaps remain, both carded rather than waved off:

- **Nothing is trimmed off values that survive.** `--tag=" a"` is still passed
  through as the tag `" a"`, which will not match an entry declaring `a`, and the
  run is empty at rc=0. That is an unknown-tag complaint rather than an
  empty-value one — the same shape as `--mode=xyz`, which already fails loudly
  because something owns mode names and checks them. Nothing owns tag names:
  `filterByTags` treats an unknown tag as a tag that matches nothing. Rewriting
  the user's input in `takeValue` would hide that rather than fix it. TASK-214.
- **A flag typed where a value belongs is swallowed as that value.**
  `dva restart --exclude-tag --tag=x` stores `"--tag=x"` as an excluded tag,
  matches nothing, excludes nothing, and **runs the whole stack at rc=0** — this
  card's own title, reached by forgetting a value rather than by typing an empty
  one. TASK-211 closed `--tag --` and `--tag` at end-of-args; this is the third
  spelling of the same slip and it is the silent one. TASK-215.

The second was found by a review probing what the new rules still admit, after
this section had already been rewritten once for over-generalising. The sentence
it replaces began "What remains open, and deliberately:" and named one thing.
**A list of what a fix leaves open is itself a claim that has to be probed**, and
neither time was it wrong about the item it named — only about being complete.

## References

References name symbols rather than line numbers. Every line number this card
carried moved at least once while the card was open — `applyEnv`'s guard was
cited as 915-917, then 915-916, and is at 953-955 after `f38ad5c` — and a review
spent time establishing that two sessions reporting different numbers were both
right about different trees. TASK-208 is five comments that lost this argument.

- `internal/cli/flagtoken.go` — `flagValue`; its `hasValue` branch is the one this card is about, and its `consumed=0` on the nothing-to-take branch is what makes advancing unconditionally safe
- `internal/cli/compose.go` — `takeValue` (the funnel, and TASK-211's argument for reporting there) and `takeList` (the same rule on the far side of `strings.Split`)
- `internal/cli/compose.go` — `applyEnv`'s opening `if envName == "" { return nil }`, and `applyDefaultMode`'s `if mode != "" || c.DefaultMode == ""`, the two guards that make an empty value identical to an absent one
- `internal/cli/plan_lifecycle.go` — `setPlanVar`'s rejection, the model TASK-211 nominated and this card measured to be a different rule
- `internal/lifecycle/orchestrator.go` — `filterByTags` / `hasAnyTag`; why "matches nothing" is safe for `--tag` and is the whole defect for `--exclude-tag`
- `internal/cli/flagtoken_test.go` — `TestSplitFlagToken`; pins `--mode=` at the grammar level only, which says nothing about whether an empty value is acceptable. Do not mistake it for coverage
- `tasks/_archive/211-a-stack-flag-missing-its-value-is-dropped-and-the-command-runs-as-if-unwritten.md` — the card whose review measured this. Cited as `tasks/todo/…` until `f38ad5c`; it was archived in the same session that filed this one
- `tasks/todo/214-an-unknown-tag-narrows-the-run-to-nothing-and-exits-zero.md` — a well-formed tag no entry declares
- `tasks/todo/215-a-flag-typed-where-a-value-belongs-is-swallowed-as-that-value.md` — `--exclude-tag --tag=x`, found by the review of this card's fix. Filed because this card twice wrote down what it left open and twice named a proper subset

## Technical Notes

`--mode=` and a missing `--mode` are indistinguishable downstream: both leave
`mode == ""`, and `applyDefaultMode` then supplies the default. That is why the
empty spelling produces the *widest* run rather than an error — it does not look
like a mistake to any code below `parseDvaFlags`. Any fix therefore has to act
in `parseDvaFlags` or above; there is no later point where the information still
exists.
