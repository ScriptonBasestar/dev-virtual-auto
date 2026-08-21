---
id: TASK-218
title: "A lone dash escapes up's flag guard, so `dva up -` starts what a bare `dva up` refuses to start"
type: bug
priority: P2
effort: S
created-at: 2026-08-20T19:00:00+09:00
source: "found while writing TASK-210's restart table — the `-` row needed a second expected message depending on config shape, and asking why turned up an escalation behind it"
scope: "internal/cli/flagtoken.go isFlagToken (new), adopted at the three plan_lifecycle.go guards that read a plan-name slot and — after review — at the two entry-name slots one position later, build.go and logs.go, whose own comments already claimed to read a token the same way the plan-name slot does. selectors.go was not touched: rejectUnknownFlags already applied this rule and its message already stated it. Not the terminator — that is TASK-216/217. Not root.go isFlag — TASK-223."
status: done
completed-at: 2026-08-21T18:20:00+09:00
---

# Task 218: a lone dash escapes up's flag guard

## Summary

Ten places in `internal/cli` decide whether a token is a flag by testing
`strings.HasPrefix(a, "-")`. Exactly **one** of the ten also tests the token's
length:

```go
// selectors.go:60, rejectUnknownFlags
if len(a) < 2 || !strings.HasPrefix(a, "-") {
    continue
}
```

So `-` is a flag to nine guards and not-a-flag to the tenth, and the tenth is
the only one standing between `dva up` and the whole stack. Where several plans
are declared and none is the default, a bare `dva up` refuses to guess; adding
a `-` makes it stop refusing and start everything.

```
$ dva up
ERROR: multiple plans configured; specify one: dva up <alpha|beta>

$ dva up -
[lifecycle] s1 (compose)
ERROR: entry "s1" up failed: compose up: Docker daemon is not reachable ...
```

That is TASK-087's defect exactly — an unrecognised token silently loses its
effect and the command runs anyway — surviving for one-character tokens because
the guard TASK-087 added excludes them.

## Measured

Six fixtures, run with `DOCKER_HOST=unix:///nonexistent-dva-review.sock` so
docker fails at once and the evidence is what was selected before it failed.

The six fixtures, defined here so the letters are rebuildable rather than a
path into someone's scratch directory:

| fixture | `stack:` entries | `plans:` | `default_plan:` | also carries |
|---|---|---|---|---|
| A | 2 | 2 | absent | — |
| B | 2 | 0 | n/a — no plans to name | — |
| C | 2 | 2 | `alpha`, top level | — |
| D | 2 | 0 | n/a — no plans to name | entry `tags:`, a `modes:` section |
| E | 3 | 0 | n/a — no plans to name | — |
| F2 | 2 | 1 | absent — the lone plan is promoted | — |

B, D and E share one shape, "no `plans:` at all", and differ only in fields no
guard on this path reads. They are kept as three variants precisely to show the
answer does not depend on those fields; a reader rebuilding only B loses
nothing but that control.

Two binaries: `dc762ca`, the current tip of `origin/master`, and `b293242`,
the head of the TASK-210 branch.

The baseline was named wrong twice before it was named as a commit, which is
the reason it is written this way now. The first pass measured `9bf3ee0` and
called it "master"; it was an ancestor. The second measured `36adfd4`, which
was the tip at the time, and `origin/master` then advanced ten commits to
`dc762ca` — `compose.go` 1042 → 1140 lines, `plan_lifecycle.go` 501 → 508 —
while this card sat open. Those are net line counts. `git diff --stat` prints
138 for the same file and it is not the growth: it sums 118 insertions and 20
deletions, so a file that churns without growing scores high there and a
shrinking file still scores positive.

Both re-runs reproduced the table. The 24 rows at `dc762ca` are byte-identical
to the 24 at `36adfd4`, so none of those ten commits touched this behaviour,
and the table below is reproducible at either. The conclusion never moved;
only the provenance line did, and a baseline named wrong is a table nobody can
reproduce.

| fixture | shape | `dva up -` | `dva down -` / `stop -` | `dva restart -` |
|---|---|---|---|---|
| A | 2 plans, no `default_plan` | **`[lifecycle] s1`** — whole stack | `unknown flag "-"` | `unknown stack entry "-"` |
| B, D, E | no `plans:` at all | `[lifecycle] <first entry>` — whole stack | `unknown flag "-"` | `unknown stack entry "-"` |
| C, F2 | a default plan resolves | `flags suppress the default plan` | same | same |

**All 24 rows are byte-identical between the two binaries** (`diff` of the two
sweeps: no output), so none of this is a TASK-210 regression; TASK-210 is only
where it became visible. Stated with its denominator because an empty sweep
would also diff clean: 24 rows each side, 6 fixtures x 4 verbs, every row
carrying an rc and a first line.

Two readings of the escalation, and they differ:

- **A is the escalation.** A bare `dva up` refuses there. `dva up -` does not.
- **B, D, E are the silent drop.** A bare `dva up` starts everything there too,
  so the outcome matches — but the `-` the user typed was discarded without a
  word, which is the half of TASK-087 that has no visible symptom.

`down` and `stop` are safe for a different reason than `up`: `teardownCommon`
(`compose.go:261`) runs its own `strings.HasPrefix(remaining[0], "-")` with no
length test, so it catches what `rejectUnknownFlags` skips. `restart` is safe
because `rejectUnknownEntryNames` reports the token as a name. `build` has
neither, so where no default plan resolves — A and B — it passes `-` to docker,
which answers `no such service: -`. Where one does resolve, C and F2,
`rejectSuppressedDefaultPlan` refuses first and docker is never reached; the
guard `build` is missing is the same `requirePlanSelection` line TASK-217 owns.

## Cause

Four classifiers see the same token and answer differently. The line cited
beside each guard is where it *tests*, not where it is declared, so the
citation and the `test` column name the same line:

| # | guard | call site | test | verdict on `-` |
|---|---|---|---|---|
| 1 | `requirePlanSelection` (`plan_lifecycle.go:73`) | `compose.go:110` | `len(args) > 0` | a selection was made |
| 2 | `rejectSuppressedDefaultPlan` (`plan_lifecycle.go:153`) | `compose.go:113` | `HasPrefix(head[0], "-")` | flag |
| 3 | `rejectUpPositionalArg` (`plan_lifecycle.go:205`) | `compose.go:116` | `HasPrefix(name, "-")` → return nil | flag (someone else's problem) |
| 4 | `rejectUnknownFlags` (`selectors.go:60`) | `compose.go:169` | `len(a) < 2 \|\| !HasPrefix(a, "-")` | **not a flag** |

The order matters and is the opposite of what reading the guards by name
suggests: `requirePlanSelection` runs **first**, on the raw `args`, and
`rejectUnknownFlags` runs **last**, on the `leftover` that survives
`parseDvaFlags` (`compose.go:120`). `rejectUpPositionalArg` runs twice, at
`:116` and again at `:172`.

So guard 1 decides the plan question before any classifier has looked at the
token, and it decides it by counting: one token present, therefore the user
named something, therefore do not ask which plan. Whether anything then refuses
comes down to guard 2, which fires only where a default plan resolves — that is
the whole difference between fixtures C/F2 and fixture A. In A guard 2 returns
nil for want of a default plan, guard 3 hands the token off on its dash test,
and guard 4 skips it on length. Nothing is left to refuse.

The disagreement also ships as two contradictory sentences. `selectors.go:140-141`
already tells the user the rule:

```
→ read as a stack entry name: a lone "-" is too short to be a flag
```

while `plan_lifecycle.go:156-159` tells the same user, about the same token:

```
ERROR: flags suppress the default plan "alpha"; name it explicitly: dva up alpha -
```

Both are shipped. Only one can be the rule.

## What to change

Rule first, code second: **is a lone `-` a flag or a name?** Both answers are
defensible — `-` conventionally means stdin, and DVA has no argument that could
be stdin — but the codebase has to pick one and say it in one voice.

Whichever way it goes, the fix is not "make the two guards agree" by copying one
test into the other. Copying `len < 2` into `rejectSuppressedDefaultPlan` makes
`dva up -` in fixtures C and F2 stop refusing and start the whole stack, trading
a wrong message for a wrong action. The refusal has to survive the alignment.

The narrow shape that keeps it has to reckon with the order above.
`requirePlanSelection` runs first, so it cannot defer to a classifier that has
not run yet: making it stop counting a lone `-` means giving it its own test, or
moving the plan question after the guards that classify. The alternative is to
give `up` the name-shaped check `down`/`stop` already have at `compose.go:261`,
which fires early enough to matter. Decide against the measured table, not from
the guard you happen to be editing.

## Completion Criteria

- [x] `dva up -` and a bare `dva up` agree on whether anything starts, in the two-plans-no-`default_plan` shape | verify: human — paste rc and the first non-warning line of both, run in a config with two plans, no `default_plan`, and two stack entries
- [x] The agreement is pinned by a differential test comparing the two invocations, not by an expected string | verify: `grep -c 'func TestUpLoneDashAgreesWithABareUp' internal/cli/plan_lifecycle_test.go` returns 1 (today: 0). Bound on the test's source because `go test -run` exits 0 when it matches nothing, and on a name that does not exist yet: `grep -c 'func Test' internal/cli/plan_lifecycle_test.go` is 18 today, so any criterion phrased as a count of existing test functions passes before the work starts
- [x] No two shipped messages give contradictory accounts of what `-` is | verify: human — paste `dva up -` and `dva restart -` from the same config and confirm both call `-` the same thing
- [x] The ruling — flag or name — is written on this card before the code changes | verify: human — see ## Resolution, "The ruling"
- [x] `dva up --force` and `dva up --no-wait` still run in a plan-less config, so the fix did not widen into real flags | verify: human — paste both; each must reach `[lifecycle] <entry>`. Not `-v`: `dva up` has no such flag and already answers `unknown flag "-v"`, so it would pass whatever the fix does
- [x] `make test` passes | verify: `make test`

## Resolution

**The ruling: a lone `-` is a NAME.** It matches nothing, so every guard that
reads a name slot must report it, and none may step aside for it.

That was already DVA's rule in two places. `rejectUnknownFlags`
(`selectors.go:60`) has skipped it via `len(a) < 2` since TASK-172, and
`rejectUnknownEntryNames` (`selectors.go:155`) prints `read as a <noun> name: a
lone "-" is too short to be a flag` to the user's face. What was missing was a
predicate the other guards could share, so the rule was restated as a bare
`strings.HasPrefix` at each site and came out backwards.

`internal/cli/flagtoken.go` now holds it:

```go
func isFlagToken(a string) bool { return len(a) > 1 && strings.HasPrefix(a, "-") }
```

adopted at `plan_lifecycle.go:175` (`rejectSuppressedDefaultPlan`), `:206`
(`rejectUnknownPlanArg`) and `:228` (`rejectUpPositionalArg`).

**`detectPlanRoute` is why "name" is the right reading and not a coin toss.** It
has never had a dash test at all — it looks `args[0]` up in `c.Plans` and finds
nothing (`plan_lifecycle.go:102`). The router reading the plan-name slot already
treated `-` as an unmatched name; the guards reading that same slot were the ones
disagreeing with it. This aligns them, rather than choosing between two opinions.

### Measured

Final binary, `DOCKER_HOST` at a dead socket, first non-empty line:

```
fixture A (2 plans, no default_plan)
  dva up        ERROR: multiple plans configured; specify one: dva up <alpha|beta>
  dva up -      ERROR: plan '-' not found. Available: alpha, beta      (was: started the whole stack)
  dva up -- -   ERROR: plan '-' not found. Available: alpha, beta      (TASK-216's second spelling, same answer)

fixture B (no plans)
  dva up        [lifecycle] s1 (compose)
  dva up -      ERROR: unexpected argument '-': 'dva up' takes no positional arguments ...
  dva up -- -   ERROR: unexpected argument '-': ... (same)

fixtures C, F2 (a default plan resolves)
  dva up -      ERROR: plan '-' not found. Available: alpha, beta      (was: flags suppress the default plan)
```

The C/F2 row is the contradiction this card names, gone: `-` is now described
the same way in every config shape, so no two shipped messages disagree about
what it is.

Across the full 264-row differential 11 of the 18 verdict changes belong to this
card, and **0 rows newly reached a runner while 8 stopped reaching one**. Two scope
notes, both owed after the fact:

- The shape written here first — "six fixtures x seven verbs x six spellings" —
  multiplies to 252, not 264. The total is the load-bearing number and card 217's
  table corroborates it (225 unchanged + 18 verdict + 21 message = 264); the product
  was a description of the matrix, and a wrong one.
- The zero is scoped, narrowly. **No fixture here declares a stack entry named `-`**
  — A, B, C, D and F2 declare `s1`/`s2`, E declares `a`/`b`/`web` — so this matrix
  cannot observe the entry-name slots this card later adopted in `build.go` and
  `logs.go`. It is a zero that could not have moved. On a fixture that does declare
  one, two rows newly reach a runner; see the correction below.

### TASK-087's hole did not re-open

`restart_names_test.go` carried a tripwire predicting that aligning these guards
would restore TASK-087, "where an unrecognised token loses its effect and the
command runs anyway". The prediction was conditionally right and worth heeding:
it assumed the fix would transplant `rejectUnknownFlags`' **length test** into
`rejectSuppressedDefaultPlan`, which would have left `-` caught by nobody.

Unifying the predicate while leaving the name guards live means the token is
still caught — as a name. Measured, not argued: 0 rows newly reached a runner —
over a matrix in which no fixture declares an entry named `-`. Read that zero as
"the guards let nothing slip", not as "nothing anywhere runs now": on the DASH
fixture `dva build multi -` moves rc 1 → 0 and reaches compose, because `-` is a
declared entry there and the user named it. A token reaching a runner *because it
names something declared* is this ruling working, not TASK-087 returning.

### What was left

Two sites still call a lone `-` a flag, both message-only, both filed rather than
fixed here because each needs a new message branch rather than a predicate swap:

- `compose.go:302` — `down`/`stop` answer `unknown flag "-"`. Adopting the
  predicate here makes the message *worse*, measured: the replacement hint never
  names the token the user typed. That is a wording ruling, not a swap.
- `plan_lifecycle.go:268` — `parsePlanFlags` answers `unsupported plan flag: -`.
  It says the same of the terminator, and that is the sharper half: measured on
  fixture A after this change,

  ```
  dva up alpha -          unsupported plan flag: -
  dva up alpha --         unsupported plan flag: --
  dva up alpha -- -       unsupported plan flag: --
  dva up -- -             plan '-' not found. Available: alpha, beta
  ```

  The last two lines are the same two tokens answered opposite ways, decided by
  whether a plan was named. `requirePlanSelection` drops a leading terminator
  (TASK-217); `parsePlanFlags`, one branch later, rejects it as a flag. Neither
  is wrong on its own and together they cannot both be right. This card widened
  the gap by fixing only the first, and the fix belongs with whoever rules on
  the wording — `--` is not a "plan flag" under any reading.

`root.go:247` (`isFlag`) also answers `-` as a flag. It is **not** a settled
counterweight — see the correction below. TASK-223 owns it.

### The sweep this card owed: 13 decision sites, and where the other axis sits

"What was left" named three sites and never said how many there were. Naming a class is
not sweeping for it, so here is the sweep with its axis and its denominator. From the
worktree root:

    { grep -rn 'strings\.HasPrefix' --include='*.go' internal/ | grep '"-"'
      grep -rn 'isFlagToken(\|isFlag(' --include='*.go' internal/
      grep -rn "\[0\] == '-'" --include='*.go' internal/
    } | grep -v '_test.go' | grep -vE ':[0-9]+:[[:space:]]*//' | sort -u

**17** lines on 2026-08-21. Four are the two predicate *definitions*, leaving **13
decision sites**. The population is Go under `internal/`, so this card's prose cannot
enter its own count.

    5   adopted isFlagToken here   build.go:171, logs.go:132,
                                   plan_lifecycle.go:175, :206, :228
    2   left, named above          compose.go:302, plan_lifecycle.go:268
    2   the isFlag defect          root.go:190, :210          → TASK-223
    4   never counted before       selectors.go:60, :158, flagtoken.go:46,
                                   runner/interaction_tree.go:269

This is a different axis from the **12** in "Correction to the census" below, and the two
must not be read as one number disagreeing with itself. Seven lines are in both. The wide
axis sees five this one does not — the `== "--"` terminator tests and the `--help`/`-h`
literals, which decide a specific token rather than the flag/name question. This one sees
ten the wide axis does not, because the wide axis predates the helper and matches only
spellings, so every `isFlagToken(...)` and `isFlag(...)` **call** is invisible to it,
along with anything outside `internal/cli`.

The four never counted:

- `selectors.go:60` — `len(a) < 2 || !strings.HasPrefix(a, "-")` **is** `!isFlagToken(a)`,
  exactly. Compared exhaustively over 39 distinct tokens: 0 disagreements, while a control
  that drops the length test disagrees on 3 — so the comparison was able to fail. The
  References line below already calls this "the rule the fix generalises"; what no one
  measured is that generalising it left this site behind, holding a third spelling of the
  reading `isFlagToken`'s own comment says the helper exists to prevent. The drift is live,
  not hypothetical: sabotaging `isFlagToken` to `len(a) > 0` fails 8 test functions and
  **none** is a `rejectUnknownFlags` test, because this site computes the predicate itself.

  Left unchanged deliberately. The swap is a no-op today — a lone `-` is skipped by both
  spellings — so on its own it is a refactor no test can fail. It is worth doing only with
  a test that drives `-` through `rejectUnknownFlags` and asserts it is read as a name:
  that test fails under the same sabotage *only after* the swap, which is the pin missing
  now.
- `selectors.go:158` — its sibling case `n == "-"` already answers the token the way this
  card argued for, in a user-facing message, and predates the card. Measured on fixture
  `207fix/F2`:

        $ dva restart -
        ERROR: unknown stack entry "-" for "dva restart"
               → read as a stack entry name: a lone "-" is too short to be a flag
               → declared here: s1, s2

  References records this; the argument never used it. It is the strongest support the
  change had — the `-`-is-a-name reading was already dva's stated position *to users*, so
  the guards were disagreeing with dva's own message, not only with `detectPlanRoute`.
- `flagtoken.go:46` — `splitFlagToken`'s prefix test is inert for `-`: the token holds no
  `=`, so both readings return `("-", "", false)`. Nothing is decided.
- `runner/interaction_tree.go:269` — `normalizeRunOptions` prefixes option **names declared
  in config**, not argv tokens. Different domain; not a slot this card governs, and the
  first site here found outside `internal/cli`.

### Correction: two sites this card should have changed, and a claim it got wrong

An independent audit of the six remaining sites, run against a snapshot of this
branch, refuted two things written above.

**1. The entry-name slot was left behind, and its comment said so.**
`build.go` and `logs.go` each read the first extra argument as an entry name, one
position after the plan-name slot, and each carried a comment claiming *"the same
reading detectPlanRoute gives the plan-name slot one position earlier"*. Changing
only the plan-name slot made that comment false. The visible cost, measured on a
plan with an entry literally named `-`:

```
before   dva build multi -   ERROR: plan "multi" builds 2 entries, so - cannot be
                                    routed to one of them; name it:
                                    dva build multi <-|s2>
```

The suggestion offers `-` and the same sentence refuses it. Both sites now use
`isFlagToken`; `dva build multi -` and `dva logs multi -` route to the entry, and
`dva build multi --no-cache` / `dva logs multi -f` still get the ambiguity error.
Pinned separately, per site, by `TestPlanBuildRoutesToAnEntryNamedWithALoneDash`
and `TestPlanLogsRoutesToAnEntryNamedWithALoneDash` — reverting either site fails
exactly its own test and no other.

**2. "The command slot only withholds a shorthand" was false.**
That sentence justified leaving `isFlag` alone, and it was generalised from two
one-token measurements (`dva -`, `dva run -`). `isFlag` has *two* call sites, and
the second one was never measured. `Execute:210` partitions every argument
flags-first before rewriting `os.Args`, so with an interaction named `-` declared:

```
dva greet -        rc=0   RAN_DASH_with=[] greet     ← asked for greet, ran "-"
dva run greet -    rc=0   RAN_GREET_with=[] -        ← the explicit form disagrees
```

A wrong answer in that slot *acts*, at rc=0, and runs a different interaction than
the one named. So the two predicates are not separated by differing cost; they are
separated only by which one has been fixed. `TestDashPredicatesDisagreeOnPurpose`
and the `flagtoken.go` doc comment now say that, and TASK-223 carries the defect.

The lesson is narrower than "measure more": both measurements this card ran were
real, and both were of the *one-token* forms. The defect lives in the two-token
form, which the census axis (`HasPrefix`, then `== "-"`) could not suggest either,
because it enumerates predicate *definitions* and this is a property of a *call
site*. Counting definitions never asks how many callers each has.

### Correction to the census in ## Technical Notes

The census command was `grep -rn 'HasPrefix(.*"-")' internal/cli/*.go | grep -v
_test`, and its **10** was correct for that pattern. The pattern cannot see
byte-indexing, and that is where `isFlag` lives:

```
baseline c51dd95, pattern HasPrefix(...,"-")                          10 sites
baseline c51dd95, widened to  == "-" | == "--" | s[0] == '-'          16 sites
  of which invisible to the narrower pattern                           6 sites
  including root.go:247 — the one site that answers "-" the other way on purpose
after this change, wide axis                                          12 sites
  the 14 first written here, minus build.go and logs.go                2 sites
```

Six invisible sites, and the single one that mattered was among them. The card
could not have anticipated the `isFlag` conflict, because the command it shipped
as reproducible evidence was blind to it. A census is only as wide as its axis,
and the axis has to be stated with the count.

The `14` was itself stale for one revision: it was measured before this card adopted
`isFlagToken` at `build.go` and `logs.go`, and those two sites had spelled the rule
with `HasPrefix`, so adopting the helper removed them from the pattern. Re-running the
same wide command on the finished tree answers **12**. The correction below rewrote the
prose about those two sites and left this number describing the tree before them —
which is the same class of defect one paragraph up, in the paragraph that names it.


## References

- `internal/cli/selectors.go:58` — `rejectUnknownFlags`; its `len(a) < 2` at :60 is the rule the fix generalises
- `internal/cli/selectors.go:142` — `rejectUnknownEntryNames`; :154 already answers `n == "-"` and :155 prints `a lone "-" is too short to be a flag`. The message was right and the guards disagreed with it
- `internal/cli/plan_lifecycle.go:68` — `requirePlanSelection`; TASK-217 fixed its terminator handling at :87
- `internal/cli/plan_lifecycle.go:150` — `rejectSuppressedDefaultPlan`; the dash test is :175, now `isFlagToken`
- `internal/cli/compose.go:302` — `teardownCommon`'s dash test, still with no length exception. Deliberately out of scope; see ## Resolution, "What was left"
- `internal/cli/restart_names_test.go` — `hintUnderDefaultPlan` pinned the divergent message so this would fail loudly when the ruling landed. It did. The column is gone and `TestRestartUnknownNameRuling` now asserts the opposite property: no case's outcome may differ across the four plan shapes
- `tasks/_archive/087-unrecognized-stack-args-become-entry-names.md` — the defect this is the one-character remainder of
- `tasks/todo/217-a-lone-terminator-disarms-build-s-plan-selection-guard-and-builds-the-whole-stack.md` — same `requirePlanSelection` line, different token
- `tasks/_archive/216-the-bare-and-terminator-forms-diverge-for-up-down-and-stop.md` — the `--` half, and still not this card, but no longer for the reason written here first. TASK-216 **overturned** the restart-local ruling: `up`/`down`/`stop` now consume a leading `--`. That widened this bug rather than leaving it alone — `dva up -- -` went rc=1 to rc=0 there, inheriting `dva up -`, so the dash now has two spellings and this card owns both

## Technical Notes

The count is the part worth keeping: **10 dash tests in non-test `internal/cli`
code, 1 with a length exception.** That is the census, and it is why the defect
reads as an oversight rather than a design — nine sites did not opt into an
exception, one did, and only that one guards `up`.

`grep -rn 'HasPrefix(.*"-")' internal/cli/*.go | grep -v _test` reproduces it.
Two of the ten (`build.go:167`, `logs.go:131`) are negated — they test for the
*absence* of a dash to decide something is a name — so they cannot leak a flag;
they are counted here because they still encode the same rule and would have to
move if the ruling changes.

Piping that same grep through `grep -c 'len('` answers **3**, not 1, and the two
extra hits are why the discriminator has to be stated: `build.go:167` and
`logs.go:131` test `len(extraArgs) > 0` — the length of the *list*, not of the
token. One site tests the length of the token itself. A census whose command
does not distinguish those two would have reported this defect as three-way
disagreement and sent the fix to the wrong files.

### Why this card is 218 and not 215

Filed as TASK-215; a concurrent session filed a different TASK-215 —
`tasks/todo/215-a-flag-typed-where-a-value-belongs-is-swallowed-as-that-value.md`
— and integrated it first. Same silent collision as TASK-217's, resolved the
same way: the integrated number keeps it, this card moves, and every inbound
link moves with it. References to "TASK-215" elsewhere in the tree now mean the
flag-as-value card.
