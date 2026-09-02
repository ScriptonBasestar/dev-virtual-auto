---
id: TASK-270
title: "Decide whether DVA ships man pages (cobra/doc GenManTree)"
type: chore
priority: P2
effort: S
exec-tier: strong
created-at: 2026-09-03T00:56:00+09:00
source: "CLI discoverability audit, 2026-09-03 session (docs vs help gap review)"
scope: "decision only: man page generation, packaging surface, and maintenance cost; implementation is a follow-up card if adopted"
status: todo
depends-on: []
---

# Task 270: decide man page distribution

## Summary

`man dva` does not work: no `cobra/doc` usage exists in the repo (`grep -rn "GenMan"` is
empty) and release archives carry only the binary and checksums. The user-facing promise is
"learn the tool from the CLI alone"; `--help` currently carries that alone. Decide whether
man pages add enough reach (offline packagers, `man`-first users, Homebrew/Linux distro
packaging conventions) to justify a generation step and a release-asset change — or record a
reasoned rejection so the gap stops resurfacing in audits.

## Problem

1. cobra makes generation nearly free (`doc.GenManTree` over rootCmd), but the real cost is
   distribution: a `man/` tree in release archives, install targets (`make install` currently
   copies one binary), and keeping generated pages in sync per release
   (`docs/52-manual-release-runbook.md` owns the release procedure).
2. Long help quality is being raised by TASK-268/269; generated man pages inherit that text,
   so sequencing matters — generating before those land would snapshot the thin help.
3. Precedent check needed: peer Go dev-tools (kubectl, helm, gh) each answered this
   differently (gh ships man via packaging; helm generates on demand).

## Completion Criteria

- [ ] A decision record states adopt/reject with rationale covering: expected consumers, generation point (build-time vs release-time), distribution channel (archive layout, `make install` behavior), and sync guarantee per release | verify: human — decision record reviewed
- [ ] If adopted: a follow-up implementation task exists specifying `doc.GenManTree` wiring, Makefile target, release-runbook (docs/52) delta, and ordering after TASK-268/269 | verify: human — follow-up card exists and is linked
- [ ] If rejected: the rationale names `--help` + USAGE.md + `dva manifest` as the supported discovery surfaces so future audits close this line of inquiry by reference | verify: human — rationale recorded
- [ ] This card's outcome is linked from TASK-268/269 if it changes their scope | verify: human — cross-links checked

## Non-goals

- No implementation in this card — generation, Makefile, and release packaging land only via the follow-up task if adopted.
- No shell-completion changes; `dva completion` already exists and is out of scope.
- No web-docs/static-site generation decision; this card is man(1) only.
