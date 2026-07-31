---
id: TASK-110
title: "23 archive links resolve only on this machine because they point into gitignored tmp/, and a filesystem link check can never notice"
type: chore
priority: P4
effort: S
status: done
decision: "A — delink, keep the artifact name and run ID in prose"
created-at: 2026-07-31T12:20:00+09:00
closed-at: 2026-07-31T13:10:00+09:00
scope: "tasks/_archive/ — 23 links across 13 files targeting ../../tmp/, which .gitignore:34 excludes and which holds 0 tracked files"
---

# Task 110: the link check validates against the disk, not against the repository

## Problem

Found while closing [TASK-109](109-the-task-link-check-has-been-red-for-22-links-since-the-repo-moved.md).
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

- [x] No link promises an untracked target | verify: for every link under `tasks/`, resolve it and run `git ls-files --error-unmatch <target>`; print the count that fail — must be 0
- [x] Provenance is not lost | verify: print the diff; each removed link must leave the artifact name and run ID readable in the prose
- [x] The blind spot is closed or recorded | verify: under C, inject a link to a fresh ignored file and confirm the checker flags it; otherwise print where the limitation is written down
- [x] Full suite passes | verify: `make test`

## Resolution

Option **A**. All 23 links replaced with plain text carrying the artifact name, the run ID, and the
original section reference. B was not taken because it would have committed generated analysis output
that `.gitignore:34` excludes on purpose and that `file-locations` assigns to `tmp/` by rule — the
repository's own convention says that evidence does not belong here.

The shape of every replacement:

```diff
-- [unified.md](../../tmp/gap-analysis-runs/20260716T091912Z-73dc094/unified.md) — G8
+- `unified.md` (gap-analysis run `20260716T091912Z-73dc094`, untracked) — G8
```

Nothing is lost that the link carried. A reader could not have opened the target anyway — that was
the defect — and can now see the run ID, which is the only thing that would let them regenerate it.
`untracked` is stated so the next reader does not go looking.

### Before and after

| | links under `tasks/` | broken | targets untracked |
| --- | --- | --- | --- |
| before | 317 | 0 | **23** |
| after | 294 | 0 | **0** |

`broken: 0` on both rows is the whole point of this task: the count that moved is the one a
filesystem check never looked at.

### The blind spot is recorded, not closed

Option A fixes the 23 instances and does not stop a 24th. The check that would is one command, and
it is written here because there is no committed link checker to teach — the sweeps in TASK-109 and
this task were both ad hoc:

````sh
# Every relative link under tasks/, tested for tracking rather than existence.
# Fenced blocks and inline code spans are stripped first — a link quoted as an
# example is not a link, and this file is itself proof: the diff above quotes one
# of the 23 removed links, and an unstripped checker calls it a live target.
find tasks -name '*.md' | while IFS= read -r f; do
  awk '/^```/{c=!c; next} !c' "$f" | sed 's/`[^`]*`//g' |
    grep -oE '\]\([^)]+\)' | sed -E 's/^\]\(//; s/\)$//' |
    while IFS= read -r link; do
      case "$link" in \#*|http*) continue;; esac
      p="$(dirname "$f")/${link%%#*}"
      git ls-files --error-unmatch "$p" >/dev/null 2>&1 || echo "UNTRACKED: $f -> $link"
    done
done
````

Two things this task's own tooling proved, both worth keeping:

- **A green link check is evidence about the checker.** The first run of the sweep above reported
  *every* link broken: `grep -ro` prefixes each match with `path:`, and the separator that split the
  two was parsed wrong, leaving a stray colon on every target. A checker that answers "all broken" is
  as uninformative as one that answers "none broken" — the useful output is the one that
  distinguishes them, which is why the counts are printed above rather than a verdict. The sweep then
  failed a *second* way, on this very file: it read the example diff above as a live link, because it
  did not know that quoted code is not a reference.
- **`zsh` does not word-split unquoted variables.** The first delink pass ran
  `for f in $files` over a newline-separated list and passed all 13 paths as one filename. `sed`
  errored, nothing was edited, and the pass would have been reported as done had the residual
  `grep` not been printed immediately after. Same failure mode as the vacuous-pass hazard already
  recorded for `dva` corpus counts.

## Related

- [TASK-109](109-the-task-link-check-has-been-red-for-22-links-since-the-repo-moved.md) — the
  same class one level up; this was found by its non-vacuity probe, not by its main check.
- [TASK-111](111-make-lint-reports-zero-issues-while-an-available-analyzer-has-50.md) — the
  same lesson in the lint gate, closed the same day.
