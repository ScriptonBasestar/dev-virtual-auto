---
id: TASK-110
title: "23 archive links resolve only on this machine because they point into gitignored tmp/, and a filesystem link check can never notice"
type: chore
priority: P4
effort: S
status: todo
created-at: 2026-07-31T12:20:00+09:00
scope: "tasks/_archive/ — 23 links across 13 files targeting ../../tmp/, which .gitignore:34 excludes and which holds 0 tracked files"
---

# Task 110: the link check validates against the disk, not against the repository

## Problem

Found while closing [TASK-109](../done/109-the-task-link-check-has-been-red-for-22-links-since-the-repo-moved.md).
The link check reports 0 broken across 291 links — and it is right about the filesystem and wrong
about the repository.

23 links in 13 archived task files point at `../../tmp/…`. Measured 2026-07-31:

```
.gitignore:34:tmp/
git ls-files tmp/ | wc -l   →   0
```

`tmp/` is ignored and holds **zero tracked files**. The seven distinct targets exist in this working
directory and in no clone:

| target | tracked |
| --- | --- |
| `tmp/gap-analysis-runs/20260716T091912Z-73dc094/unified.md` | no |
| `tmp/gap-analysis-runs/20260716T091912Z-73dc094/code-to-doc.md` | no |
| `tmp/gap-analysis-runs/20260716T091912Z-73dc094/doc-to-code.md` | no |
| `tmp/gap-analysis-runs/20260716T091912Z-73dc094/evidence-cli.md` | no |
| `tmp/gap-analysis-runs/20260716T091912Z-73dc094/evidence-contradictions.md` | no |
| `tmp/gap-analysis/convergence.md` | no |
| `tmp/gap-analysis/evidence-flags.md` | no |

All 23 are inside `tasks/_archive/`; none are in an active task.

## Why this is the same defect as 109, one level down

TASK-109 fixed 14 links written as `/Users/archmagece/myopen/…`. Those passed review because they
resolved on the machine that wrote them. These 23 are the identical mistake in a form the checker
cannot catch at all: a link that escapes the checked subtree is validated against whatever happens to
be on the local disk, so an ignored target passes forever and fails for every other reader silently.

That is worth stating plainly — **a green link check is not evidence the links work.** It is evidence
they work here.

## Options

- **A — delink.** Replace each link with plain text naming the artifact and its run ID, so the
  provenance survives without a promise the repo cannot keep. The evidence these tasks cite is a
  2026-07-16 analysis run; the prose already summarises what it found.
- **B — commit the artifacts.** Move the seven files somewhere tracked and repoint. Makes the links
  real, at the cost of committing generated analysis output that `.gitignore` deliberately excludes —
  and `file-locations` names `tmp/` as the place such output belongs.
- **C — teach the checker about tracking, then decide per link.** Have it flag any link whose target
  is untracked or ignored, not merely absent. This is the part that stops a third instance; it
  composes with either A or B.

## Decision needed

Whether the archive should keep pointing at evidence the repository does not carry (A), start
carrying it (B), or both fix and guard (C). B contradicts `.gitignore` on purpose and so is not a
call to make silently.

## Acceptance criteria

- [ ] No link promises an untracked target | verify: for every link under `tasks/`, resolve it and run `git ls-files --error-unmatch <target>`; print the count that fail — must be 0
- [ ] Provenance is not lost | verify: print the diff; each removed link must leave the artifact name and run ID readable in the prose
- [ ] The blind spot is closed or recorded | verify: under C, inject a link to a fresh ignored file and confirm the checker flags it; otherwise print where the limitation is written down
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-109](../done/109-the-task-link-check-has-been-red-for-22-links-since-the-repo-moved.md) — the
  same class one level up; this was found by its non-vacuity probe, not by its main check.
