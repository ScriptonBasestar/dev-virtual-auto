---
id: TASK-202
title: "A disposition kept two AC bindings that no longer measure what they claim"
type: docs
priority: P2
effort: S
created-at: 2026-08-19T17:36:34+09:00
source: "measured 2026-08-19 — TASK-194's AC1 exits 0 having scanned 0 of 197 cards, its AC4 exits 1 with `no matches found`; TASK-063 discloses the identical residue in its own record and TASK-194 does not"
scope: "The record text of tasks/_archive/194 and the two sibling cards that bind a tasks/done/ path (106, 127). No verdict, acceptance criterion or timestamp changes."
status: done
completed-at: 2026-08-26T10:46:21+0900
verified-at: 2026-08-26T10:46:21+0900
archived-at: 2026-08-26T10:46:21+0900
verification-summary: |
  Re-read the three affected archive records and disclosed their moved `tasks/done/`
  bindings without changing their historical verdicts or timestamps. The current
  `_archive/` equivalents resolve, the sweep excludes this follow-up record, and
  `make doc-check` passes.
---

# Task 202: A disposition kept two AC bindings that no longer measure what they claim

## Summary

`tasks/_archive/194-three-cards-fail-validation-and-the-failure-has-become-the-baseline.md`
was archived with `quality-review: pass`. Two of its four acceptance criteria no longer
support that verdict when re-run today, in opposite directions.

**AC1 (`194:174`) passes vacuously.** The binding is
`ce task validate --all; echo "exit: $?"`. Re-run 2026-08-19:

```
📋 Validating all tasks...

No task files found
exit: 0
```

All 197 cards live in `tasks/_archive/`, and `--all` never opens them:
`task_storage.go:218` returns `filepath.SkipDir` for any directory named `archive` or
`_archive` while enumerating candidates (ce-agent-kit at `c99d1921`; the `ce` on PATH is
built from `9b11760`, so line numbers may drift). Every other state directory (`todo`,
`doing`, `done`, `blocked`, `plan`, `decision`) is empty. The criterion reads as "the
corpus validates" and measures "nothing was looked at".

That is not the only `_archive` skip, and the difference matters to anyone re-running this
by hand. Validating an archived card *by path* takes a second, independent route:
`canonicalFrozenZone` (`canonical_validator.go:277-284`, same repo) tests the same two
directory names, but only after frontmatter parsing and after canonical detection have
both succeeded. Measured 2026-08-19, that left 9 of 197 cards outside every skip and
judged as if they were unfinished current work — `046`, whose unquoted YAML scalar errors
before the check is reached, and `001`–`008`, which carry neither `id:` nor `type:`, are
therefore not detected as canonical, and fall through to a legacy validator that has no
archive concept at all. All nine exited 1. Both causes are fixed in the two commits
preceding this one on this branch; the archive now measures 197 skipped, 0 non-zero.
`--all` still scans 0 of 197 either way, which is what AC1 was written against.

**AC4 (`194:177`) now fails.** The binding is
`ls tasks/done/082-* tasks/done/123-* tasks/done/164-*`, whose stated purpose is "the
three filenames are unchanged". Re-run 2026-08-19: `no matches found`, exit 1. The
filenames are in fact unchanged; the three cards moved to `tasks/_archive/` after the
disposition, so the binding's *path* went stale while its *claim* stayed true. A reader
re-running it today gets a failure that says nothing about the thing being asserted.

The repository already has a house answer for this shape. `TASK-063` hit the same residue
and disclosed it in its own record (`063:29-31`): "One cosmetic residue: criterion 1's
`verify:` binding still names the pre-move path … the stale path is resolved by this
archive move." `194` records nothing.

This is the same defect class as `6dccd36` and `faf7a17` — a record that reads stronger
than the measurement under it — reached by a different route. There the count was wrong;
here the command is right and no longer points at what it was written to point at.

## Completion Criteria

- [x] `194`'s AC1 line states that the binding scans zero cards under the current layout, or is replaced by one that scans a non-zero denominator | verify: `grep -cE 'scans 0|0 of 197|_archive' tasks/_archive/194-*.md` returns ≥ 1 (today: 1 after the disclosure)
- [x] `194`'s AC4 binding resolves against the current tree | verify: `ls tasks/_archive/082-* tasks/_archive/123-* tasks/_archive/164-*` exits 0 and prints 3 paths
- [x] The record says which of the two routes it took, in `063`'s self-disclosure form rather than by silent rewrite | verify: human — read `194`'s verification-summary and confirm a later reader learns the binding changed and why, without re-running it
- [x] The other two archived cards that bind a `tasks/done/` path are examined and either corrected or recorded as still-valid | verify: `grep -rlE '^- \[[ x~]\].*verify:.*tasks/done/' tasks/_archive/*.md | grep -v '/202-'` returns 3 files (106, 127, 194) — excluding this record, each is dispositioned in its verification summary
- [x] The sweep's axis and denominator are stated in the commit message | verify: human — the commit message names "3 cards bind a tasks/done/ path"
- [x] `make doc-check` passes | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && make doc-check`

## Resolution (2026-08-26)

The three archived records were re-read without changing their historical verdicts or
timestamps. The result is disclosure, not a silent rewrite:

| card | currentization | status of the original claim |
| --- | --- | --- |
| `194` | Its verification summary now says AC4's `tasks/done/082-*`, `123-*`, `164-*` binding was valid before the move and gives the current `_archive/` equivalents. | filenames and disposition result remain valid; the original path is historical |
| `106` | Its verification summary already recorded the stale `tasks/done/090-*` shell path; it now names `_archive/090-*` as the current target. | supersession result remains valid |
| `127` | Its verification summary now records that `tasks/done/126-*` was the path at verification time and that TASK-126 is now archived. | the referenced card remains the same; only the path moved |

AC1's zero-card denominator is also now stated in the `194` verification record, so a reader can
distinguish the successful command from the archive-skipping layout it actually measured. The
current-tree path check for `194` resolves all three cards, and the three-card sweep explicitly
excludes this follow-up record so it cannot certify itself. `make doc-check` passes.

## References

- `tasks/_archive/194-…:174` — AC1, the vacuous binding; `:177` — AC4, the stale path
- `tasks/_archive/063-…:29-31` — the precedent: the same residue, disclosed in-record
- `internal/adapter/filesystem/task_storage.go:218` (ce-agent-kit `c99d1921`) — `--all`
  skips the directory while enumerating, so archived files are never opened
- `internal/usecase/task/canonical_validator.go:277-284` (same repo, `canonicalFrozenZone`)
  — per-file validation skips the same names, but only after parsing and detection succeed.
  Neither path exists in this repository; `ce` is installed here as a binary only
- `faf7a17`, `6dccd36` — the same defect class through the count route

## Open Questions

- Rewrite the binding, or leave it and add a residue note? `063` chose disclosure, and
  disclosure is the safer default here: rewriting a landed binding erases the evidence that
  the disposition was taken against a different tree. Recommend disclosure for AC4 (the claim
  still holds) and replacement for AC1 (the claim never held).
- AC1's vacuity is a property of the validator's layout assumption, not of this card. Whether
  `ce task validate --all` should scan `_archive` at all is a separate question and belongs to
  ce-agent-kit, not here. This card must not be closed by changing that tool.

## Technical Notes

- Do not change `quality-review`, `verified-at` or `archived-at` on any card touched here.
  The defect is in what the record claims, not in when it was judged.
- `ce task validate --all` exiting 0 must never again be cited as a corpus gate in this
  repository without printing its denominator alongside.
- The two `_archive` skips are independent, and "the archive is skipped" is not a single
  fact. Saying it without naming which route was measured is exactly how nine red cards sat
  unnoticed behind an `--all` that exits 0 without opening a single one of them.
