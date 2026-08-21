---
id: TASK-217
title: "A lone terminator disarms build's plan-selection guard and builds the whole stack"
type: bug
priority: P2
effort: S
created-at: 2026-08-20T16:56:47+09:00
source: "found by TASK-210's caller census — the card measured four verbs, the two functions it changed have seven callers, and build was the one whose terminator behaviour was already wrong"
scope: "internal/cli/plan_lifecycle.go requirePlanSelection. Filed against buildCmd's call site; the fix landed one level down, in the guard itself — see ## Resolution for why that is narrower rather than wider."
status: done
completed-at: 2026-08-21T18:20:00+09:00
---

# Task 217: A lone terminator disarms build's plan-selection guard and builds the whole stack

## Summary

In a config with several plans and no default, `dva build` refuses to guess.
Adding a `--` makes it stop refusing and build everything:

```
$ dva build
ERROR: multiple plans configured; specify one: dva build <alpha|beta>

$ dva build --
[+] Building ... Image p1-web  Building
ERROR: failed to connect to the docker API ...
```

`--` means "no names follow". It cannot mean *more* than the bare form — that is
the escalation TASK-198 named and TASK-207 ruled on. Here it converts a refusal
into a whole-stack build.

## Measured

The fixture is three files, written out here because the original lived in a
scratch directory that has since been overwritten — a card whose evidence is a
path is a card nobody can re-run:

```yaml
# dva.yml — two plans, NO default_plan (grep -c default_plan → 0)
version: "0.1.44"
stack:
  s1: {description: entry one, default_runner: compose,
       runners: {compose: {files: [docker-compose.yml], project_name: p1}}}
  s2: {description: entry two, default_runner: compose,
       runners: {compose: {files: [docker-compose.yml], project_name: p2}}}
plans:
  alpha: {description: first plan,  entries: [{name: s1, runner: compose, order: 10}]}
  beta:  {description: second plan, entries: [{name: s2, runner: compose, order: 10}]}
```

```yaml
# docker-compose.yml — a real build: context, so entry selection is visible
services:
  web:
    build: {context: ./ctx}
    command: sleep 1
```

```dockerfile
# ctx/Dockerfile
FROM alpine
RUN true
```

Run with `DOCKER_HOST=unix:///nonexistent-dva-review.sock`, so the docker call
fails instantly and the evidence is what was selected before it failed.

| binary | `dva build` | `dva build --` | `dva build -` |
|---|---|---|---|
| `dc762ca` (master) | rc=1, `multiple plans configured` | rc=1, **`Image p1-web Building`** then docker API failure | rc=1, `no such service: -` |
| `b293242` (TASK-210 branch) | rc=1, `multiple plans configured` | rc=1, **`Image p1-web Building`** then docker API failure | rc=1, `no such service: -` |

Measured three times now, against a fixture rebuilt from the definition above
each time, and every cell has held. The baseline moved twice while this card
was open and that is the reason to name commits here rather than a branch:
the first pass labelled `9bf3ee0` "master" when it was an ancestor, the second
labelled `36adfd4` "master" and `origin/master` advanced to `dc762ca` before
the card was integrated. Both re-runs reproduced the table exactly. The
conclusion never moved; only the provenance line did, and a baseline named
wrong is a table nobody can reproduce.

The `-` column is here as a control and belongs to TASK-218: it also gets past
`requirePlanSelection`, but docker rejects it, so `build` escalates only for the
token docker accepts. Same guard, same line, different downstream luck.

### What TASK-210 already changed for `build`, without measuring it

TASK-210's card says it measured four verbs. `build` is a fifth caller of the
same helpers, and two of that branch's commits each moved a different `build`
row. Bisected across every state where the answer can differ: `dc762ca`, plus
the only two commits on this branch that change behaviour. Four of the
branch's commits touch `.go` files, but `eec5b3f` and `b293242` touch comments
alone — their diffs are empty once comment lines are excluded — so neither can
move a row. Fixtures are the ones TASK-218 defines; `DOCKER_HOST` points at a
dead socket, so rc is the evidence:

| shape | `dc762ca` master | `d51dc98` fix 1 | `51bbf79` fix 2 |
|---|---|---|---|
| A: `build` | rc=1 `multiple plans configured` | = | = |
| **A: `build --`** | **rc=0, builds** | **=** | **=** |
| C, F2: `build` | rc=0, builds | = | = |
| **C, F2: `build --`** | **rc=1 `flags suppress the default plan`** | **rc=0, builds** | = |
| C, F2: `build -- --bogus` | rc=1 `flags suppress the default plan` | = | rc=1 `no such service: --bogus` |

Two separate movements, and they are not the same kind of change.

**Fix 1 turned a refusal into a build.** In the two fixtures where a default
plan resolves, `dva build --` was refused on master and now runs. That is the
ruling applied — a bare `dva build` is rc=0 there, so the terminator form now
matches the bare form, which is the identity TASK-207 and TASK-210 exist to
establish. It is still a refuse-to-accept transition that TASK-210 made without
measuring `build`, and it is written down here because nothing else records it.

**Fix 2 changed who answers.** `build -- --bogus` refuses on both sides; the
DVA-owned "flags suppress the default plan" became docker's "no such service".
After `--` every token is a name and `build` forwards names to docker
(TASK-172), so docker answering is the honest end of that path — but a specific
diagnostic was lost, and `dva build -- -- s1` moved the same way. Whoever rules
on the `build -- <service>` question below should rule on this row too: if
`build` should refuse a `--`-prefixed *name*, that is a new check, not a
restored one.

**Neither touched the row this card is about.** Fixture A — two plans, no
default — reads `build` rc=1, `build --` rc=0 at every one of the four commits.
That is this defect in the shared harness, invisible in the output only because
A's compose file uses `image:` and so has nothing to build; the card's own
fixture supplies a build context to make it visible. The reason the same change
fixed C and F2 but not A is the whole fix: in C and F2 the guard that fired was
`rejectSuppressedDefaultPlan`, which TASK-210 taught to step aside for a leading
terminator; in A it is `requirePlanSelection`, which still counts `--` as a
selection. The sibling guard has already been given the treatment this card
asks for.

This is pre-existing and not a TASK-210 regression. TASK-210 is
where it became visible: its census found seven callers of the routing helpers,
and measuring the three it had not covered (`build`, `logs`, `status`) turned
this up. `logs` and `status` were unchanged in all four fixtures — cobra strips
the terminator before their `RunE` ever sees it, because they do not set
`DisableFlagParsing`.

## Cause

`parseDvaFlags` keeps the terminator on purpose, and `requirePlanSelection`
returns nil as soon as any token is left:

```go
mode, _, _, _, remaining, err := parseDvaFlags(args)   // remaining == ["--"]
...
if err := requirePlanSelection(c, "build", remaining); err != nil {   // sees a token, returns nil
```

One token is enough to mean "do not ask for a plan", and `--` is a token. That
rule is right for real flags — `dva build --no-cache` must not be answered with
"name a plan" — and wrong for the separator.

**Why build alone.** The other six callers are covered by something build cannot
use:

- `up`, `down`, `stop` — `rejectUnknownFlags` refuses the surviving `--` (measured
  in fixtures A and B: `unknown flag "--"`).
- `restart` — TASK-207 added an explicit re-check that strips the terminator and
  re-runs `requirePlanSelection` when nothing else is left.
- `logs`, `status` — cobra consumes the terminator; it never reaches this code.
- `build` — cannot refuse unknown flags at all. `dva build --no-cache` has to
  reach docker verbatim (TASK-172) *wherever it reaches docker at all*, which
  measured is fixtures A and B; where a default plan resolves,
  `rejectSuppressedDefaultPlan` refuses `--no-cache` first. So the guard that
  backstops up/down/stop is deliberately absent here. That is the whole reason
  this one survived.

## What to change

The narrow form is restart's, applied to `remaining`: when every token was the
terminator, the invocation means "no names given", so re-run the selection guard
with an empty list. `dropLeadingTerminator` (TASK-210,
`internal/cli/plan_lifecycle.go`) already exists for the plan-name slot.

Decide first whether the guard should also cover the several-plans case for
`dva build -- <service>`, which today reaches docker as a service name. That
argument predates plans and TASK-172 protects it, so the likely ruling is "only
a lone terminator", but it must be ruled rather than assumed — the fix is one
line either way and the difference is which invocations start refusing.

## Completion Criteria

- [x] `dva build --` refuses in the several-plans-no-default shape exactly as a bare `dva build` does | verify: human — build the fixture from the three files in ## Measured, run both, and paste rc plus whether any image began building
- [x] The identity is pinned by a differential test comparing `build --` to a bare `build`, not by an expected string | verify: `grep -c 'func TestBuildLoneTerminatorMeansABareBuild' internal/cli/build_flag_leak_test.go` returns 1 (today: 0). Bound on the test's source rather than on `go test -run`, which exits 0 when it matches nothing, and on a name that does not exist yet rather than on a count of `buildCmd.RunE`, which is 3 in this file today (two calls and a comment) and would certify itself
- [x] `dva build -- <service>` and `dva build --no-cache` still reach docker unchanged wherever they reach it today, whichever way the ruling goes | verify: human — paste both invocations' first line against a config with **no** default plan (fixture A or B). Do not use a one-plan config: the lone plan is promoted to default, and `dva build --no-cache` is refused there today by `rejectSuppressedDefaultPlan` — that refusal is not this card's to remove, and a verifier sent to that shape would read it as a regression
- [x] The ruling for `build -- <service>` is recorded on this card, not left implicit | verify: human — see ## Resolution, "The ruling"
- [x] `make test` passes | verify: `make test`

## Resolution

Fixed in `requirePlanSelection` (`internal/cli/plan_lifecycle.go:87`), not in `buildCmd`.

```go
args = dropLeadingTerminator(planRoutingArgs(args))
```

That is the line `detectPlanRoute` already had eight lines below, reading the same
slot. `parseDvaFlags` keeps the terminator deliberately, so "the callers that
reject unknown flags have always rejected a stray `--`" — and this is the caller
where the token kept **for** rejection was the token that suppressed the
rejection.

**Why this is narrower than the plan, not wider.** The card proposed restart's
shape: a special case inside `buildCmd`. Moving it into the guard produces the
identical measured outcome and touches nothing else, because the guard's only
output is its verdict. The args handed to docker are the caller's own slice and
are never rewritten, so every invocation that reaches docker today reaches it
spelled exactly as before. A `buildCmd`-local fix would have been one more place
that knows about terminators; this is one fewer.

### The ruling

**`dva build -- <service>` keeps reaching docker as a service name.** The
terminator means "no names follow" only when nothing follows it. `build -- web`
follows it with a name, so it is a selection and TASK-172 protects it. Only the
lone terminator changes.

Measured after the fix, fixture A (two plans, `grep -c default_plan` → 0), first
non-empty line, dead `DOCKER_HOST`:

```
dva build              ERROR: multiple plans configured; specify one: dva build <alpha|beta>
dva build --           ERROR: multiple plans configured; specify one: dva build <alpha|beta>   <- the fix
dva build web          time="TS" level=warning msg="No services to build"                      <- docker answered
dva build -- web       time="TS" level=warning msg="No services to build"                      <- unchanged
dva build -- --no-cache  no such service: --no-cache                                           <- unchanged
```

`build -- web` and `build -- --no-cache` are "unchanged" by the differential, not
by inspection: neither appears among the 18 verdict changes or the 21
message-only changes below. The docker warning is what reaching docker looks like
in this fixture, whose compose file declares `image:` and so has nothing to build.

`build -- --no-cache` reaching docker as a *service name* — and docker replying
`no such service: --no-cache` — is left alone, and is not this card's to fix.
`internal/cli/flagtoken.go` already argues it should not leak; that is filed
separately.

### Measured

264-row differential, six fixtures x seven verbs x six argv spellings, base
`c51dd95` against the fix, `DOCKER_HOST` pointed at a dead socket. Discriminator:
the first non-empty output line, classified `GUARD` when it starts with `ERROR: `
and `PASS` otherwise. rc is **not** a discriminator here — a row that passed every
guard and then failed in docker also exits 1 — and a `[lifecycle]`/`[plan:` scan
is worse still: `build` and `logs` never print those lines, so it reports a false
zero for exactly the verb this card is about.

| | rows |
|---|---|
| unchanged | 225 |
| verdict changed (GUARD/PASS) | 18 |
| message changed only | 21 |

Of the 18: **1 is this card** (`A: build --`, PASS → GUARD). 11 are TASK-218.
6 are C/F2 rows moving GUARD → PASS, where the token now reaches docker as a name
and docker refuses it, which is how `logs nosuch` already behaved.

Positive control for the discriminator, so that "18 changed" is distinguishable
from a column that cannot move: rows reaching a runner fell 56 → 48 across the
same differential.


### Correction: the line moves four rows this card said it could not

An adversarial review of the fix commit, re-measured here from scratch, refuted the
sentence that justified the change.

**`Only build reaches here with the terminator intact` — false for four rows.** The
comment above the fix said up/down/stop drop the terminator at their call sites and that
logs never sees one. Both halves are true of a *single* terminator and neither is true of
a second one: the call sites drop the leading terminator, and whatever follows rides
through untouched. Measured at `c51dd95` -- the commit before `33b3e76` added that line --
against HEAD, both built from clean checkouts and run in one directory whose `dva.yml`
declares two plans, no `default_plan`, and two compose entries backed by a real
`docker-compose.yml`, with `restart -- --` as the control that does not move:

| argv | line absent (`c51dd95`) | line present (HEAD) |
|---|---|---|
| `dva up -- --` | `unknown flag "--" for "dva up"` | `multiple plans configured; specify one: dva up <p1\|p2>` |
| `dva down -- --` | `unknown flag "--" for "dva down"` | the same refusal, for `down` |
| `dva stop -- --` | `unknown flag "--" for "dva stop"` | the same refusal, for `stop` |
| `dva build --` | rc 0, `No services to build` | the refusal — the row this card is about |
| `dva logs -- --` | reaches the docker API | the refusal |

Five rows move and four are not build. Those four are an improvement, and the card
asserted they could not happen.

Those compose entries are load-bearing, and the earlier form of this paragraph named no
fixture at all. On a `script:`-runner fixture the `dva build --` cell reads rc 1, `no
configuration file provided` -- docker's answer when it finds no project -- instead of rc 0,
`No services to build`. The defect is the same and the message is not, so a reviewer who
picks the other shape reports the cell as not reproducing, which is what happened. A row
whose text depends on the fixture has to name the fixture.

**`dva build -- --` was listed here as a second control and is not one.** `restart -- --` is:
`unknown stack entry "--"` at both revisions, refused on either side. `build -- --` also reads
the same at both revisions -- but what it reads is `no such service: --`, which is docker
answering after it has loaded the project and looked the token up as a service name. The row
does not move because the guard is bypassed on **both** sides, not because it sits outside the
fix. `dropLeadingTerminator` drops exactly one `--`; up/down/stop drop another at their call
sites, so `up -- --` arrives empty and is refused, while build has only the guard's own drop
and arrives holding `["--"]`, which `len(args) > 0` reads as a selection made. A control
broken on both sides certifies nothing, and this one was offered as the boundary of the fix --
the same "a control that shares no code path with the disputed row is not a control for it"
this pair of cards spends two sections on. The row is open at HEAD and is filed as TASK-224.

**The count of tests pinning the line was measured on a run that never finished.** The
first answer was "one test, `TestBuildLoneTerminatorMeansABareBuild`". That test does not
merely fail under the revert — `dva build --` reaches `execComposePassthrough`, and
`ExecReplace` panics the test binary on purpose (TASK-144) rather than let `syscall.Exec`
swallow the run. The binary died after **30 of 809** tests, so "one test fails" was the
first failure before an abort, not a count. Re-run with that one test skipped, the suite
completes and the true set is visible.

Two trees are involved and their totals differ by this card's own new test, so read each
count against the unsabotaged baseline of the tree it came from. The aborted run was taken
while that test still carried a `logs` subtest: 809 with it, 808 once it was dropped, and
807 on the final tree with the one aborting test skipped. `go vet` passing proves only that
a sabotage compiles. The count needs `grep -c '^=== RUN'` matching that tree's own baseline
and `grep -c '^panic:'` at zero, both stated, or the number is a floor wearing a total's
clothes. An independent reviewer reached the same conclusion from a run that died at 30 of
804 on the parent commit -- a third tree, a third denominator, the same defect.

`TestSecondTerminatorMeetsThePlanGuardNotTheFlagGuard` now pins up/down/stop: each asserts
the `-- --` refusal is the plan guard's, word for word what a bare verb gets, rather than
the unknown-flag guard's one layer out. Reverting the line fails all three subtests on a
run that finishes. `logs -- --` is deliberately not a subtest — it would reach the same
passthrough tripwire and trade one recorded failure for every test after it.

Any later "reverting X fails N tests" in this card or 218 should be read off a run whose
`=== RUN` count matches the unsabotaged baseline. Where it does not, the number is a floor.

## References

- `internal/cli/compose.go:687-734` — `buildCmd`'s prologue: `DisableFlagParsing` :708, `parseDvaFlags` :719, `detectPlanRoute` :728, `requirePlanSelection` :731, `rejectSuppressedDefaultPlan` :734. The frontmatter's original `:662` and this line's original `:640-668` were both stale by the time the card was worked; re-derived at the fix commit
- `internal/cli/compose.go` — `restartCmd`'s terminator re-check, the shape this would copy
- `internal/cli/plan_lifecycle.go:68` `requirePlanSelection`, `:103` `detectPlanRoute`, `:145` `dropLeadingTerminator` — the fix is one line, at `:95` once the correction below rewrote the comment above it. The earlier form of this entry gave a span, `:87`–`:99`, and this card called it stale and miscounted. Both charges were wrong. At `4e88aa8`, the tree it was written on, `:87` is the fix line and `:99` is `detectPlanRoute`'s own `dropLeadingTerminator` call — two calls to the same helper, which is what the span was pointing at, not a distance from a function's first line. It went stale afterwards, when the correction below rewrote the comment above the fix and pushed everything down eight lines. That is this commit's edit, not an earlier author's error, and reading a moved citation as a wrong one gets the blame exactly backwards. The durable statement is the relative one — `detectPlanRoute` begins exactly eight lines below the fix line and does the same thing for the same slot, and that distance held across the rewrite
- `tasks/_archive/210-the-flag-terminator-is-refused-as-a-flag-that-suppresses-the-default-plan.md` — the census that found this, and the ruling it would extend
- `tasks/_archive/207-restart-exits-0-on-an-unknown-service-name-and-the-test-pinning-it-cites-a-deleted-command.md` — the terminator/bare identity
- `tasks/_archive/218-a-lone-dash-escapes-up-s-flag-guard-so-dva-up-dash-starts-what-a-bare-up-refuses.md` — the same `requirePlanSelection` line reached by `-` instead of `--`; whichever card lands first should check the other's table still holds
- `tasks/_archive/216-the-bare-and-terminator-forms-diverge-for-up-down-and-stop.md` — the terminator ruling for the verbs that take no names; `build` is the exception it does not cover

## Technical Notes

The harness exists. `build_flag_leak_test.go` already drives `buildCmd.RunE`
directly (5 call sites across that file, `native_build_delegation_test.go` and
`provision_note_test.go`), so a terminator case is an addition to a working
pattern rather than new scaffolding.

What is missing is narrower, and worth stating as its own number: of those 5
invocations, **0 pass a `--`**. The commands that forward unknown flags to a tool
are exactly the ones whose terminator handling nobody writes a test for, because
the interesting cases all look like flags. That is the same blind spot this card
describes in the code.

### Why this card is 217 and not 214

It was filed as TASK-214 on a branch cut from `36adfd4`. A concurrent session
filed a different TASK-214 — `tasks/todo/214-an-unknown-tag-narrows-the-run-to-
nothing-and-exits-zero.md` — and integrated it first, so `origin/master` owns
that number. Rebasing produced two cards with the same id, and git said nothing:
the filenames differ, so the merge is clean and the collision is silent.

The integrated number wins and the unintegrated one moves. What that costs is
every inbound link, which is why the renumber swept `USAGE.md`,
`internal/cli/restart_names_test.go`, the sibling cards and the archived
TASK-210 rather than only the two files being renamed. Anything that still says
"TASK-214" now means the unknown-tag card.
