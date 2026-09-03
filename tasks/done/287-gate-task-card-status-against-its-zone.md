---
id: TASK-287
title: "Gate task-card status against the directory zone it sits in"
type: bug
priority: P2
effort: S
exec-tier: standard
created-at: 2026-09-03T21:30:00+09:00
source: "Queue audit 2026-09-03: eight cards under tasks/done/ carried status: todo and no gate noticed"
scope: "doccheck zone/status consistency check for tasks/{todo,done,issue,_archive}, its vacuous-run guard, and the Makefile doc-check path"
status: done
depends-on: []
---

# Task 287: gate card status against its zone

## Summary

A task card records its state twice — once as the directory it lives in, once as
`status:` in its frontmatter — and nothing checks that the two agree. On 2026-09-03 an audit
found **eight** cards sitting in `tasks/done/` while still declaring `status: todo`
(TASK-245, 254, 259, 265, 275, 279, 284, 285). They were repaired in the same batch that
found them; this card closes the hole that let eight of them accumulate.

## Problem

The drift is invisible to every gate the repository actually runs.

- **`doccheck` never reads `status`.** Confirmed by grep across `tools/doccheck/*.go`: the
  string appears only in `archive_test.go` fixtures. `main.go`'s own header enumerates what
  it enforces — links, size, verify bindings, binding tools, archive frontmatter presence —
  and card state is not among them.
- **`ce task validate` does not assert it either**, and `--all` excludes `tasks/_archive/`
  during its tree walk. `tools/doccheck/archive.go` already documents this class of blind
  spot at length for a neighbouring property, and the reasoning transfers verbatim: a
  property enforced by nothing is enforced by nothing regardless of which tool *looks* like
  it should own it.
- **Reading the zone from the path is itself error-prone**, which is why this belongs in a
  tested tool rather than in each session's ad-hoc shell loop. `tasks/_archive/done/` exists,
  so splitting a path on `/` and taking segment two classifies an archived done-card as
  unfinished. A peer session made exactly that mistake on 2026-09-03 and reported TASK-262
  and TASK-264 as dangling when both were closed. The zone rule must be
  longest-prefix, not nth-segment.

The consequence is not cosmetic. `status:` is what a plan counter, an audit sweep, and any
`ce`-driven tooling read to decide whether work remains. Eight cards reading `todo` from
inside `done/` is eight false "open" signals in every count taken from frontmatter, and the
counters in PLAN-002 and PLAN-004 were both stale at the moment the drift was found.

## Zone rule to enforce

| Path prefix (longest match wins) | Permitted `status:` |
| --- | --- |
| `tasks/_archive/` | `done`, `superseded` |
| `tasks/done/` | `done` |
| `tasks/todo/` | `todo` |
| `tasks/issue/` | `todo` |
| `tasks/plan/` | not a card — skip (plans carry `type: plan`, no `status:`) |

`superseded` is permitted only under `_archive/` because the three existing superseded cards
all live there and a superseded card is closed work; `done/` is for cards that were actually
completed. A card whose `status:` is absent entirely is an error under every zone except
`tasks/plan/`.

## Completion Criteria

- [x] `doccheck` resolves each task card's zone by longest matching path prefix — never by
      splitting on `/` and indexing a segment — so `tasks/_archive/done/` classifies as archive
      rather than as done | verify: `/usr/bin/grep -Eq '^func TestZoneResolvesArchiveDoneAsArchive\(' tools/doccheck/cardstatus_test.go && go test ./tools/doccheck -count=1`
- [x] `doccheck` reports a card whose `status:` is not permitted in its zone, naming the file,
      the found status, and the permitted set, and exits 1 | verify: `/usr/bin/grep -Eq '^func TestCardStatusMismatchFailsWithNamedFile\(' tools/doccheck/cardstatus_test.go && go test ./tools/doccheck -count=1`
- [x] A card missing `status:` in a card zone is an error; a file under `tasks/plan/` is skipped
      without being counted as a card | verify: `/usr/bin/grep -Eq '^func TestMissingStatusIsErrorAndPlansAreSkipped\(' tools/doccheck/cardstatus_test.go && go test ./tools/doccheck -count=1`
- [x] The check fails on a vacuous run — zero cards swept means the walk broke, not that the
      repository is clean — matching the exit-1-on-vacuous rule the rest of `doccheck` already
      holds | verify: `/usr/bin/grep -Eq '^func TestZeroCardsSweptIsFailure\(' tools/doccheck/cardstatus_test.go && go test ./tools/doccheck -count=1`
- [x] `doccheck`'s printed report gains a `cards_checked` / `status_mismatches` pair so a passing
      run states how much it swept, and `main.go`'s header comment lists this check alongside the
      others it enumerates | verify: `go run ./tools/doccheck | /usr/bin/grep -Eq '^cards_checked:'`
- [x] The whole repository passes the new check with no exemption list | verify: `make doc-check`
- [x] Repository gates pass | verify: `make test && make doc-check && make commit-check`

## Non-goals

- No YAML parser. `doccheck` is stdlib-only by declaration (`main.go`), and this check needs a
  single frontmatter key read as a line prefix — the same technique `archive.go` already uses.
  Introducing a YAML dependency for one key would be a larger change than the defect.
- No new `status:` values. The vocabulary in use is exactly `todo`, `done`, `superseded`
  (measured across all cards); this card enforces the existing set rather than extending it.
- No moving of cards. If the check fires, the fix is a human deciding which of the two
  statements — directory or frontmatter — is the true one. A tool that auto-moves cards would
  guess, and guessing wrong silently closes open work.
- No check of `closed-at`, `decision-status`, `depends-on`, or any other field. Scope is the
  one property that was measurably drifting.

## Parent plan

None, deliberately. PLAN-004 covers documentation truth in the shipped surfaces users and
agents read; this is task-queue metadata hygiene, which is not in that plan's declared scope.
TASK-279 sets the precedent for an unparented card that a plan references without adopting.
