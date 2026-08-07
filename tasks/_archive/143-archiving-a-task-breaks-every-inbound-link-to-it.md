---
id: TASK-143
title: "Moving a task to _archive breaks every link pointing at it, and CI goes red"
type: bug
priority: P2
effort: S
created-at: 2026-08-03T13:00:00+09:00
source: "TASK-090 finalize verification — the gate 090 built, caught the archiving workflow itself"
depends-on: [TASK-090]
scope: "dva repo — tools/doccheck, tasks/ workflow"
status: done
quality-review: pass
quality-reviewed-at: 2026-08-07T18:05:08+09:00
verified-at: 2026-08-07T18:05:08+09:00
archived-at: 2026-08-07T18:05:08+09:00
quality-review-evidence: |
  - kind: test
    command-or-step: make test && make doc-check (mise go 1.26.4)
    result: exit 0; shared suite green
  - kind: recheck
    command-or-step: acceptance criteria re-observed
    result: doccheck task-link resolve; make doc-check OK
verification-summary: |
  quality-review pass; re-checked deliverables. doccheck task-link resolve; make doc-check OK. Shared: make test OK; make doc-check OK (mise go 1.26.4).
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

- [x] **Direction: A** (teach `tools/doccheck` to resolve). Chosen because it matches how the ID
      is used — a task's identity is its number, its directory is its (changeable) state — and it
      keeps doccheck honest about literal paths while surviving transitions by design. B re-buys
      the problem every time a new state directory appears.
- [x] Under A: a `../todo/NNN-….md` link to a file now in `done/` resolves, and a genuinely
      missing `NNN` still fails — pinned by `TestLinks_resolvesMovedTaskLink` and
      `TestLinks_taskLinkGenuinelyMissingStaysBroken`. An ambiguous basename (two files) fails too
      (`TestLinks_ambiguousTaskLinkIsAnError`) — the checker refuses to guess.
- [x] (B not chosen.)
- [x] `make doc-check` exits 0 on a clean tree, `links_checked: 513` (non-zero).
- [x] Task paths inside inline code — `verify:` bindings — are covered: `checkCodeTaskPaths`
      scans inline-code spans for `tasks/<state>/NNN-….md` and resolves each. On the current repo
      the scan reports **0 stale of 513 links / 0 inline-code misses** — the 5 stale inline-code
      paths from 2026-08-03 were already repointed, and the scan now keeps them that way. Pinned by
      `TestLinks_resolvesMovedInlineCodeTaskPath` and
      `TestLinks_inlineCodeTaskPathGenuinelyMissingFails`.
- [x] The 5 stale paths from 2026-08-03 resolve (they were repointed AND would now resolve by
      basename regardless); a genuinely missing `NNN` inside a binding is still reported.
- [x] `AGENTS.md`'s Documentation-gate section names the mechanism ("Task links survive state
      transitions (TASK-143)").
- [x] `make test` exits 0.

## Resolution

`tools/doccheck/check.go` gained `resolveTaskLink`: before a `tasks/<state>/NNN-…` markdown link is
declared broken, it searches the inventory for any `tasks/*/NNN-…` sharing the basename — one hit
resolves, zero stays broken, more than one is an ambiguity the gate fails on. `checkCodeTaskPaths`
applies the same resolution to `tasks/…/NNN-….md` strings inside inline-code spans (where
`verify:` bindings live, invisible to the link scan because code is blanked before link extraction).
This unblocked a live CI breakage: moving 146/162/153 to `done/` had left 5 inbound markdown links
pointing at their old `todo/` paths; `make doc-check` went 5 broken → 0 with no manual repoint.

## Notes

Option A is the smaller surface and the one that matches how the ID is actually used — a
task's identity is its number, and its directory is its *state*, which is expected to
change. Option B keeps doccheck honest about literal paths but re-buys the problem every
time a new state directory appears.

## Review

Independent `core:code-reviewer` pass: **No Critical/High; implementation correct.** Three
Medium findings applied before finalizing:

- **M1 (latent correctness, fixed)** — `checkCodeTaskPaths` scanned the raw body, so a fenced code
  block leaked into the inline-code scan (the backtick regex matched from an opening fence's third
  backtick to a closing fence's first), turning a stale task path in a ``` example into a false
  broken link. Now strips fenced regions first, the way `extractLinks` already does; inline code
  (where `verify:` bindings live) is preserved. Pinned by the existing fenced-suppression test.
- **M2 (test gap, fixed)** — the inline-code ambiguity branch was untested; added
  `TestLinks_ambiguousInlineCodeTaskPathIsAnError`.
- **M3 (test gap, fixed)** — anchor validation against the *resolved* file was untested; added
  `TestLinks_movedTaskLinkAnchorCheckedAgainstResolved` (present-anchor passes, absent fails).

Deferred (Low): the ambiguity error lists no file paths (`resolveTaskLink` discards its hits) —
listing them would make the error actionable without a re-grep; left for a follow-up. The
`BrokenLinks` counter now also counts inline-code misses (semantics slightly looser, gate
behaviour unchanged). Duplicate basenames across state dirs exist in the data (090, 092, …) but no
link currently points at the wrong one, so no ambiguity fires today.
