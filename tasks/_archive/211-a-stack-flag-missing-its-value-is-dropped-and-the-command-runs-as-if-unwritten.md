---
id: TASK-211
title: "A stack flag missing its value is dropped and the command runs as if unwritten"
type: bug
priority: P2
effort: S
created-at: 2026-08-20T15:35:00+09:00
source: "found by the adversarial review of TASK-207 — a second spelling reached this pre-existing hole, which is how the hole was noticed"
scope: "internal/cli/compose.go parseDvaFlags value-taking cases and internal/cli/flagtoken.go flagValue. Every parseDvaFlags caller is affected, not restart alone; count the call sites when the work starts rather than quoting a figure — TASK-208 exists because the comment's count is wrong."
status: done
---

# Task 211: A stack flag missing its value is dropped and the command runs as if unwritten

## Summary

`flagValue` (`internal/cli/flagtoken.go:114-122`) returns `ok=false` when a
value-taking flag is the last token, and every caller in `parseDvaFlags`
(`internal/cli/compose.go:764-783`) is written as

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
run, reported as success. The same holds for `--env`, `--tag` and
`--exclude-tag` — the four value-taking cases in `parseDvaFlags` — and for every
command that parses its flags this way.

Not `--var`, though the first draft of this card said so. `--var` is not a
`parseDvaFlags` case at all; it is handled in `parsePlanFlags`
(`internal/cli/plan_lifecycle.go:186-192`) and already errors:

```
dva restart p1 --var      rc=1   ERROR: --var requires KEY=VAL      (master and branch)
```

That is the shape the four cases here should copy, and it is in this repo
already — which also means a fix aimed at `parseDvaFlags` will not touch `--var`
and does not need to.

The terminator swallows a value the same way, and TASK-207's ruling turns that
into an escalation. Measured, two plans and no `default_plan`, final TASK-207
binary `f0abf9449eeb` against master `2d8bc83e46a9`:

```
                     master           TASK-207 branch
restart --mode --    rc=0 nothing     rc=0 whole stack
restart --env --     rc=0 nothing     rc=0 whole stack
restart --mode       rc=0 whole stack rc=0 whole stack
restart              rc=1 refused     rc=1 refused
```

The last two rows are why this is filed as the root cause rather than a TASK-207
regression: master already restarts the whole stack for a bare `--mode` in a
config whose bare `restart` it refuses. The branch only makes the `--mode --`
spelling agree with the `--mode` spelling. Closing this card closes both.

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

Make a missing value an error, in `parseDvaFlags` rather than in `flagValue`.

**The reason this card first gave was wrong.** It said the helper "is also used
where consuming the next token is optional". It is not:

```
$ git grep -c 'flagValue(' 9bf3ee0 -- internal/ tools/     # before the fix
compose.go:4   flagtoken.go:1        → four callers + the definition
```

Four callers, and they were the four cases being fixed. `flagValue`'s own doc
comment carried the same claim — *"which every caller has always treated as 'no
value given' rather than as an error"* — so the false rationale sat in two
places and would have been inherited by whoever read either one.

**Stated against `9bf3ee0` on purpose, because the fix changed the answer.** The
four cases now call `takeValue`, so `flagValue` has exactly one caller:

```
$ git grep -c 'flagValue(' 74af3a1 -- internal/ tools/     # and 30b9a69, unchanged
compose.go:1   flagtoken.go:1        → one caller (takeValue) + the definition
```

The first draft of this card printed the four-caller grep with no commit and
four line numbers — `compose.go:787/792/797/802` — none of which contains
`flagValue(` any more. Both new code comments said "exactly four callers" in the
present tense, in the very commit that made it one. That is TASK-208's defect
class reproduced inside the card that cites TASK-208 as required reading before
quoting a count, which is the strongest available argument that a count needs
the commit that produced it attached. Fixed in `compose.go` and `flagtoken.go`;
found by review, not by me.

The conclusion survives on a different argument: `flagValue` is not told the
flag's name and has no `err` to return, so it cannot say `--mode requires a
value`. `parseDvaFlags` holds both. Its neutrality is prospective, not a current
constraint, and both comments now say that instead.

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
tool. Counted rather than quoted, as this card's `scope:` asks:

```
$ grep -rn 'parseDvaFlags(' internal/ tools/     # 34 lines
compose.go:120  compose.go:245  compose.go:339
compose.go:397  compose.go:476  compose.go:648   # 6 production call sites
```

Six, which is TASK-208's corrected figure and not the in-code comment's "12" or
its "7 of the 12". The remaining hits are the definition and tests. The
dependence question is settled by measurement below, not by this count.

TASK-208 lists the same six as `120,245,339,397,455,569`. The count agrees and
the last two line numbers do not, because TASK-207's gate and this card's
`takeValue` both landed above them — which is exactly the drift TASK-208's own
Technical Notes predicted. Re-run the grep rather than reconciling the two
lists.

All six take the error and return it before doing anything:

```
120, 339, 397, 476, 648   if err != nil { return err }
245                       if err != nil { return nil, nil, "", nil, nil, err }
```

Five return it as a cobra `RunE` error; `245` is a helper and hands it to its
own caller.

So `err` is not merely set; no call site continues on partial state. That
mattered to check, because `parseDvaFlags` sets a *named* return and keeps
looping — the loop finishing is not the same as the command proceeding.

## Measured after the fix

`takeValue`, a closure symmetric with `takeBool`, sets `err` when `flagValue`
reports `ok=false`. Same no-plans fixture, master `ee40f4f334cd` (which already
contains TASK-207) against branch `7faecc18b2d0`, built at `74af3a1`. The later
commit `30b9a69` touches `_test.go` files only, so that fingerprint still
describes the code these rows are about — stated because a binary fingerprint
quoted without the commit that produced it stops being checkable the moment the
branch moves.

| invocation | master | branch |
|---|---|---|
| `restart --mode` | rc=0, s1+s2 bounced | rc=1 `--mode requires a value`, nothing ran |
| `restart --mode --` | rc=0, s1+s2 bounced | rc=1, same message, nothing ran |
| `restart --tag` | rc=0, s1+s2 bounced | rc=1 `--tag requires a value` |
| `restart --env` | rc=0, s1+s2 bounced | rc=1 `--env requires a value` |
| `restart --env dev` | rc=1 `env 'dev' not found` | rc=1, identical |

The last row is the control and it is deliberately identical on both binaries:
without it, a build that had simply stopped accepting `--env` at all — or one
whose `restart` no longer ran anything — would produce the same four rows above.

Row 2 closes the divergence recorded higher up this card. `--mode --` and
`--mode` now agree by both erroring, rather than by both restarting everything.

### What this card does not close

A third spelling produces the identical harm and is untouched: `--mode=`, with
an empty inline value, still runs the whole stack and exits 0. `hasValue` is
true, so `flagValue` returns `("", 0, true)` before reaching the branch this fix
is about, and `mode == ""` is indistinguishable from no `--mode` at all.

That is a different axis — a value that is there and is empty, rather than no
value to take — in a different branch of the same helper, so no fix to one
reaches the other. Filed as TASK-213 rather than folded in here, but it means
this card's title should be read as closing two spellings of three, not the
class. Found by the same review.

### It is not restart-only, and no passthrough caller loses anything

Same fixture, other verbs, same two binaries:

| invocation | master | branch |
|---|---|---|
| `up --mode` | rc=0, s1+s2 started | rc=1 `--mode requires a value` |
| `stop --env` | rc=0, s1+s2 stopped | rc=1 `--env requires a value` |
| `down --tag` | rc=0 | rc=1 `--tag requires a value` |
| `build --mode` | rc=1 `no configuration file provided: not found` | rc=1 `--mode requires a value` |
| `logs --tag` | rc=1 `unknown flag: --tag` | rc=1, identical |

`build` is the caller that hands leftovers to `docker compose`, and its master row
settles the passthrough question by measurement rather than by reading: the error
it fails with is docker's own complaint about a missing compose file, which means
docker was invoked and never saw `--mode`. A recognised flag is not appended to
`filtered`, so the token was **dropped, not forwarded** — there was no forwarding
behaviour for the fix to take away. `logs` never reaches this code for `--tag` at
all; cobra rejects it as an unknown flag on both binaries.

### The tests fail for the right reason

The suite is 12 cases: `TestParseDvaFlagsRejectsAMissingValue`'s 7 refusal
subtests and 1 control, plus the 4 relocated unit tests. Green baseline, then
four sabotages, each aimed at a different mechanism and each reverted with
`git checkout --` before the next:

| sabotage | what fails |
|---|---|
| restore the silent drop (`if false && !ok …`) | all 11 refusal cases; control passes |
| read the end of argv instead of `end` (`i+1 < len(args)`) | exactly the 2 terminator subtests; the 4 unit tests all pass |
| wire only 3 of the 4 cases — leave `--exclude-tag` on bare `flagValue` | exactly the 2 `--exclude-tag` cases (subtest + unit test) |
| keep the error, change its wording (`"%s is bad"`) | all 7 subtests; the 4 unit tests all pass |
| **hardcode the flag name** (`fmt.Errorf("--mode requires a value")`) | **nothing — `go test ./internal/cli/` was fully green** |

Each has a distinct signature — 11 / 2 / 2 / 7 — which is what makes the set
discriminating. Four sabotages that all failed everything would show only that
the tests run.

### The fifth sabotage survived, and closing it took a new assertion

The adversarial review found it; my four had not. With the message hardcoded to
`--mode requires a value`, `dva restart --tag` answers by naming a flag the user
never typed, and **every test in the package passed**. The refusal rows asserted
only `strings.Contains(err.Error(), "requires a value")`, and the four unit
tests asserted only `err != nil`. Nothing checked that `name` reached the
message.

That is worse than an ordinary gap, because passing the flag's name through is
the *only* reason this card, `takeValue`'s comment and `flagValue`'s comment all
give for reporting in `parseDvaFlags` instead of in `flagValue`. The suite was
not testing the property the design argument rests on.

Closed by adding a `wantFlag` column and asserting the message names the flag as
the user spelled it. Re-measured with the identical sabotage reapplied:

```
--- FAIL: …/the_short_form_-M          --- FAIL: …/a_repeatable_--tag
--- FAIL: …/--tag_before_the_terminator --- FAIL: …/--env
--- FAIL: …/--exclude-tag              FAIL  github.com/…/internal/cli
```

Five of seven, and the two `--mode` rows still pass — correctly, since that is
the one flag the hardcoded string happens to name. `-M` is the row that matters
most: the message has to say `-M`, not silently promote it to `--mode`.

Two of these say something the prose could not:

**Sabotage 3 measures criterion 1's weakness instead of asserting it.** With
`--exclude-tag` left unfixed, `grep -c 'requires a value' internal/cli/compose.go`
**still returns 1** — the criterion passes over a partially-wired fix. What
catches it is the per-flag rows, and both of them fire.

**Sabotage 2 shows the two levels are not redundant.** A fix that handles only a
trailing flag passes all 4 unit tests and 5 of the 7 subtests. The unit tests
name `parseDvaFlags` as the source of the refusal but are blind to the
terminator half of the defect; only the end-to-end rows reach `dvaFlagEnd`.
Sabotage 4 is the mirror image — the unit tests do not pin the message either.
Neither level would be sufficient alone.

### Four existing tests were endorsing the defect

`git grep 'parseDvaFlags(' 9bf3ee0 -- internal/ tools/` — 34 hits at that commit,
6 of them production call sites, 38 in the tree as it now stands because
`30b9a69` moved four callers into a file that already had some — turned up `TestParseDvaFlags_MissingValue`, `_MissingEnvValue`,
`_MissingTagValue` and `_MissingExcludeTagValue` in `compose_flags_test.go`. Each
called `parseDvaFlags` with a flag that had nothing to take and discarded `err`
with `_`, asserting only that the result was empty — which is true whether the
flag was rejected or silently dropped.

So all four passed before the fix and after it, and `make test` staying green
across this change said nothing about the change. What they were actually
documenting was the drop, as intended behaviour; anyone reading them would have
taken the defect for the specification.

They now require the error, and they moved to `flagvalue_missing_test.go`
alongside the end-to-end test. Both levels are kept on purpose: the end-to-end
subtests drive `restartCmd.RunE`, so a refusal raised anywhere else on that path
would satisfy them, while these four name `parseDvaFlags` as the source.

**Correction to the reason given in `30b9a69`'s commit message.** It says the
move put `compose_flags_test.go` "back under the 600-line limit (601 → 553)".
The file measured 581 lines at `9bf3ee0` and at `74af3a1`, and 553 at `30b9a69`
— so the move removed 28 lines and no committed state was ever over 600. The
601 was a transient working-tree state during editing, and quoting it as if it
were a commit is exactly the mistake the paragraph above is about.

The limit itself is real but is not this repository's: it is `error_lines: 600`
for the `test` kind in ce-agent-kit's `file-size.yaml`, applied by a
workstation edit hook. Review checked `Makefile`, `.golangci.yml`, `AGENTS.md`
and `CLAUDE.md`, found nothing, and concluded no such limit exists — the grep
was right and the conclusion was not, because the rule is not repo-declared.
Worth separating: a workstation policy is a real constraint on how work gets
done here and not a fact about this codebase, so a commit message should not
cite it as "the limit" without saying whose.

The commit message cannot be corrected in place — the branch is pushed and
amending it needs a force-push, which is not available to me — so the record is
here.

## Completion Criteria

- [x] A missing value is an error for every value-taking case in `parseDvaFlags` | verify: `/usr/bin/grep -c 'requires a value' internal/cli/compose.go` returns ≥ 1 (was 0, now 1) — **this binding is weaker than it reads, and that was measured rather than guessed**: one shared `takeValue` closure serves all four cases, so the count stays 1 however few cases call it. Sabotage 3 below leaves `--exclude-tag` on the bare `flagValue` and the grep still returns 1 while two test cases fail. What actually carries "every case" is the per-flag rows, which name `--mode`, `-M`, `--tag`, `--env` and `--exclude-tag` separately
- [x] A test pins it for at least `--mode` and one repeatable flag such as `--tag` | verify: `/usr/bin/grep -rc 'func TestParseDvaFlagsRejectsAMissingValue' internal/cli/ | /usr/bin/grep -v ':0'` names `internal/cli/flagvalue_missing_test.go:1` (was: no file matched)
- [x] The test asserts nothing ran, not only that rc≠0 | verify: `/usr/bin/grep -c 'nothing should have run' internal/cli/flagvalue_missing_test.go` returns 1 (delete the assertion and it is 0). **Rebound after review.** The original binding was `grep -A30 'func TestParseDvaFlagsRejectsAMissingValue' internal/cli/*_test.go | grep -c 'ranMarkers'`; review called the 4-line margin brittle, and adding the `wantFlag` rows pushed `ranMarkers` from line 45 to 58, outside the `-A30` window — the binding returned 0 with the assertion fully intact. A criterion tied to a line-distance window measures the file's shape, not the property
- [x] The error names the flag the user typed, not a fixed one | verify: `/usr/bin/grep -c 'wantFlag' internal/cli/flagvalue_missing_test.go` returns 4 (was 0). Added because the hardcoded-name sabotage passed the entire package — see "The fifth sabotage survived"
- [x] Passthrough callers are checked for a dependence on the drop, and the finding recorded here | verify: human — settled by measurement in "It is not restart-only" above. `build` is the forwarding caller; on master it fails with docker's own "no configuration file provided", proving docker was invoked without ever seeing the dropped `--mode`. A recognised flag is never appended to `filtered`, so no caller could have been depending on the token reaching an external tool. `logs` does not reach this code for `--tag` — cobra rejects it first, identically on both binaries
- [x] `make test` passes | verify: `make test` — 9 packages ok, `internal/cli` coverage 74.7% → 74.8%. Note what this criterion is worth on its own: it was already satisfied by the *unfixed* tree, because the four tests that touched this behaviour discarded `err`. A green suite only became evidence once those four were changed to require the error — see "Four existing tests were endorsing the defect"
- [x] `make lint` and `make doc-check` pass on the final tree | verify: `make lint` → `gofmt -s: 289 files checked, 0 unformatted` / `0 issues.`; `make doc-check` → 267 markdown checked, 542 links, 0 broken, 1127 test funcs across 172 `_test.go` files, 128 `run_patterns`, 0 unmatched, `doc-check: OK` (denominators printed because an empty corpus would also report zero failures)

## References

- `internal/cli/flagtoken.go:114-122` — `flagValue`, whose `ok=false` is correct and ignored
- `internal/cli/compose.go:764-783` — the four value-taking cases with no `else`
- `internal/cli/compose.go:748-756` — `takeBool`, the pattern to follow, and TASK-172's argument for reporting here
- `tasks/_archive/207-restart-exits-0-on-an-unknown-service-name-and-the-test-pinning-it-cites-a-deleted-command.md` — the card whose review measured this
- `tasks/todo/208-five-comments-size-the-flag-fallthrough-class-at-twelve-call-sites-the-real-count-is-six.md` — before quoting a call-site count, read this

## Technical Notes

`dvaFlagEnd` sets `end` at the first `--`, so a flag immediately before the
terminator is "last" for `flagValue`'s purposes even though tokens follow it.
That is why `--mode --` reaches the same drop as a trailing `--mode`, and any
fix has to treat both, not only the end-of-argv case.
