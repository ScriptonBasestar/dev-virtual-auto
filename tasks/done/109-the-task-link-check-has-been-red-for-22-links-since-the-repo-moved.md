---
id: TASK-109
title: "22 broken links in tasks/_archive/ keep the link check permanently red, so a genuinely new break is invisible"
type: chore
priority: P4
effort: S
status: done
created-at: 2026-07-31T11:58:00+09:00
completed-at: 2026-07-31T12:25:00+09:00
scope: "tasks/_archive/ — 13 files, 22 links. Every target is determinable; none require a judgment call about intent."
---

# Task 109: a check that is always red is not a check

## Problem

The link check over `tasks/` reports **22 broken links, all in `tasks/_archive/`**. Outside the
archive it reports **0 of 171**. The archive breaks have been there long enough that every recent
task has had to say "22 pre-existing" when reporting its own link result — including
[TASK-104](../done/104-a-literal-key-that-spells-a-composite-key-deletes-one-command.md),
[TASK-099](../done/099-usage-md-says-conflicts-are-silently-ignored.md) and
[TASK-107](../done/107-command-suggestions-come-out-in-a-different-order-every-run.md).

That is the actual cost. A number everyone has learned to subtract is a number nobody reads, and the
23rd break — a real one, in an active task — would arrive looking exactly like the noise.

## Measured 2026-07-31

All 22 fall into three groups, and **every target exists**:

**Group 1 — 14 links in 8 files (`001`–`008`): absolute paths from the repo's old location.** They point at
`/Users/archmagece/myopen/scripton/dev-virtual-auto/...`; the repo now lives under `mywork/`. All
five distinct targets exist here today:

| target | exists |
| --- | --- |
| `docs/30-config-merge-semantics.md` | yes |
| `docs/31-execution-plan-resolution.md` | yes |
| `docs/40-declarative-stack-and-plans.md` | yes |
| `USAGE.md` | yes |
| `examples/README.md` | yes |

**Group 2 — 4 links: right file, wrong directory.** The task moved to `_archive/` after the link was
written:

| link as written | actual location |
| --- | --- |
| `../todo/027-up-silently-ignores-unknown-args.md` | `_archive/` |
| `../todo/033-restart-discards-service-names.md` | `_archive/` |
| `../decision/041-status-exit-code-on-unrunnable-entry.md` | `_archive/` |
| `../decision/045-doctor-check-fix-implemented-but-schema-forbids-it.md` | `_archive/` |

**Group 3 — 4 links: same task ID, renamed slug.**

| link as written | actual file |
| --- | --- |
| `010-schema-runner-allowlist.md` | `010-schema-validate-all-runners.md` |
| `012-fix-env-precedence-claim.md` | `012-fix-env-precedence-docs.md` |
| `018-fix-env-file-precedence.md` | `018-fix-claude-md-env-precedence.md` |
| `023-repo-wide-precedence-sweep.md` | `023-stale-os-chain-docs31-docs40.md` |

No link is ambiguous. Nothing here needs someone to reconstruct what an author meant.

## Options

- **A — repair all 22 (recommended).** Mechanical: strip the absolute prefix to a relative path,
  repoint the four directory moves, match the four renamed slugs by ID. Repairing a path so it
  reaches the same content is not rewriting the historical record; the record is the prose.
- **B — exclude `_archive/` from the link check.** One line, and the check goes green. But it also
  stops noticing when an active task links into the archive and the target moves again, which is
  precisely how group 2 happened.
- **C — leave it, and write the 22 down as accepted.** Cheapest. Keeps the subtraction everyone is
  already doing, and keeps the 23rd break invisible.

## Acceptance criteria

- [x] The check is green | verify: run the link check over all of `tasks/`; print links checked and broken — broken must be 0
- [x] Nothing was repaired by deletion | verify: print the diff line count of removed links (should be 0 — every fix repoints, none drops)
- [x] The targets are right, not merely present | verify: for each of the 3 groups, open one repaired link and confirm the destination is the document the sentence is talking about
- [x] A new break is now visible | verify: introduce one bogus link in a scratch copy, confirm the check reports exactly 1, then discard the scratch copy

## Resolution

Option **A** — all 22 repointed, none dropped. Measured 2026-07-31:

| | before | after |
| --- | --- | --- |
| links checked under `tasks/` | 273 | 291 |
| broken | 22 | **0** |
| markdown links inside `_archive` | 114 | **114** (nothing deleted) |

The `after` count is higher because this task file and
[TASK-110](../todo/110-23-archive-links-point-into-gitignored-tmp-and-the-checker-cannot-tell.md) added links
of their own.

**The three groups, as repaired:**

| group | fix | count |
| --- | --- | --- |
| 1 | `/Users/archmagece/myopen/scripton/dev-virtual-auto/` → `../../` | 14 |
| 2 | `../todo/` and `../decision/` → `../_archive/` | 4 |
| 3 | slug matched by task ID | 4 |

**Targets checked for meaning, not just existence** — one per group:

| citing sentence | lands on |
| --- | --- |
| 001 `## References` → `30-config-merge-semantics.md` | doc titled "Config Merge Semantics" |
| 046 "same exit-code family for `dva stack status`" | 041, titled "should 'dva stack status' exit non-zero when an entry can never run?" |
| 025 "why `env_file` > `environment:`" | 018, titled "Fix inverted env_file/environment precedence in CLAUDE.md" |

**A fix the criteria did not ask for.** In group 3 the repointed links still *displayed* the old
slug — `[023-repo-wide-precedence-sweep.md](../_archive/023-stale-os-chain-docs31-docs40.md)` — so a
reader saw a filename that is not in the repo. Four such display texts (three in 025, one in 031)
were aligned with their targets. A link check reads the parenthesis, not the brackets, so this
class is invisible to it entirely.

**Non-vacuity.** A green check proves nothing unless it can go red. Copied `tasks/` to a scratch tree
and injected one bogus link:

| | links checked | broken |
| --- | --- | --- |
| scratch copy, untouched | 291 | 41 |
| same copy, one bogus link added | 292 | **42** |

Exactly +1, and the injected link appeared by name in the output. The check still sees a break.

## Left open

The scratch copy reporting **41** rather than 0 is not a defect in the repair — the copy contains
only `tasks/`, so every `../../docs/…`, `../../USAGE.md` and `../../internal/…` link escapes it. But
reading that list surfaced a real finding, now filed as
[TASK-110](../todo/110-23-archive-links-point-into-gitignored-tmp-and-the-checker-cannot-tell.md): 23 archive
links point into `tmp/`, which `.gitignore:34` excludes and which holds 0 tracked files. They resolve
here and in no clone, and a filesystem-based check can never say so.

So the headline number from this task deserves its caveat: the check is green against **this disk**.

## Related

- [TASK-110](../todo/110-23-archive-links-point-into-gitignored-tmp-and-the-checker-cannot-tell.md) — found by
  this task's non-vacuity probe rather than by its main check.
- [TASK-106](../done/106-usage-md-is-46-percent-over-the-doc-size-standard.md) — the other unenforced
  standard found this week; same shape, in that nothing measures it until someone counts by hand.
