---
id: TASK-143
title: "Moving a task to _archive breaks every link pointing at it, and CI goes red"
type: bug
priority: P2
status: todo
effort: S
created-at: 2026-08-03T13:00:00+09:00
source: "TASK-090 finalize verification — the gate 090 built, caught the archiving workflow itself"
depends-on: [TASK-090]
scope: "dva repo — tools/doccheck, tasks/ workflow"
---

# Task 143: Make archiving repoint its own inbound links

## Problem

Task files link to each other by relative path — `../done/074-….md`, or a bare
`083-….md` between siblings. Archiving moves the *target* and leaves every *referrer*
pointing at a path that no longer exists.

`make doc-check` is CI's first step (`.github/workflows/ci.yml:22-23`), so each archiving
round turns master red. Measured 2026-08-03 on a clean tree, mid-round:

```
$ make doc-check
  ERROR    68 broken link(s)
doc-check: FAIL
```

All 68 pointed at `tasks/done/NNN-….md` files that had moved to `tasks/_archive/`.
Referrers spanned `tasks/_archive`, `tasks/done`, `tasks/decision` and `tasks/todo` — the
rot is not confined to the files being moved, so the person doing the move cannot see it by
looking at their own diff.

This has happened before and been cleaned up by hand: commit `2b81af5` ("repoint 22 stale
archive links so the check can go red again") and `2c734a8`. Nothing prevents the next
round.

## Repaired, not fixed

The 68 links were repointed on 2026-08-03 (`make doc-check` → `broken_links: 0`,
`links_checked: 477`) using `tmp/scripts/repoint-task-links.py`, which parses doccheck's
own `BROKEN` output, re-finds each target by basename across `tasks/`, and rewrites the
link relative to the referrer. That script lives in `tmp/` and is untracked — it is a
one-off, not a fix.

The defect is that the archiving move and the link update are two separate acts, and only
one of them is anybody's job.

## The half the gate cannot see

`doccheck` checks markdown *link targets*. A task path written inside inline code — which
is where `verify:` bindings live — is invisible to it. So `make doc-check` exits 0 while
bindings point at files that moved.

Measured 2026-08-03 by scanning `tasks/` for every literal
`tasks/<state>/NNN-….md` string and testing each for existence: **5 stale of 8 distinct**.
Two of the five are inside `verify:` bindings, which is the damaging kind — a binding whose
path no longer resolves fails for a reason that has nothing to do with the criterion:

```
tasks/_archive/106-…md:67   verify: grep -c 'USAGE.md' tasks/done/090-…   → exit 2
                                             (actual: tasks/_archive/090-…)
tasks/_archive/063-…md:158  → tasks/blocked/063-…  (actual: tasks/_archive/063-…)
```

The remaining three are prose. Both halves are the same defect: a task's *state directory*
is being used as part of its *address*.

## Acceptance criteria

- [ ] Pick a direction and record why:
      (A) teach `tools/doccheck` to resolve a `tasks/<state>/NNN-…md` link to whichever
      state directory actually holds `NNN-…md`, so links survive state transitions by
      design; or
      (B) make repointing part of the move — a checked-in script or Makefile target that
      archiving must run, with `make doc-check` proving it ran.
- [ ] Under A: a link written as `../done/NNN-….md` to a file now in `_archive/` resolves,
      and a link to a genuinely missing `NNN` still fails — prove both, with counts.
- [ ] Under B: the script is tracked (not in `tmp/`), has the standard bash/python header,
      and refuses to guess when a basename resolves to more than one file.
- [ ] `make doc-check` exits 0 on a clean tree, printing a non-zero `links_checked`.
- [ ] Task paths inside inline code — `verify:` bindings above all — are covered too. Print
      the stale-of-total count the way the scan above does; a bare "none found" is not a
      result when the denominator is unstated.
- [ ] The 5 stale paths found on 2026-08-03 resolve, and a genuinely missing `NNN` inside a
      binding is still reported.
- [ ] The archiving procedure in `AGENTS.md` names whichever mechanism was chosen, so the
      next round does not rediscover this.
- [ ] `make test` exits 0.

## Notes

Option A is the smaller surface and the one that matches how the ID is actually used — a
task's identity is its number, and its directory is its *state*, which is expected to
change. Option B keeps doccheck honest about literal paths but re-buys the problem every
time a new state directory appears.
