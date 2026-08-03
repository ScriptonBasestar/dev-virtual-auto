---
id: TASK-106
title: "USAGE.md is 730 lines against a 500-line standard and 27KB against a 10KB one, and nothing measures it"
type: chore
priority: P4
effort: M
status: superseded
superseded-by: TASK-090
created-at: 2026-07-31T11:35:00+09:00
closed-at: 2026-07-31T12:55:00+09:00
scope: "USAGE.md — 730 lines / 27KB vs the 500-line / 10KB per-document standard in skill:docs:doc-standards"
---

# Task 106: the manual is the only document over the limit, and the limit is unenforced

## Problem

`skill:docs:doc-standards` sets 500 lines / 10KB per document. Measured 2026-07-31:

| document | lines | size | within standard |
| --- | --- | --- | --- |
| **USAGE.md** | **730** | **27KB** | **no — 46% over lines, 170% over bytes** |
| README.md | 203 | 6KB | yes |
| AGENTS.md | 172 | 8KB | yes |
| ARCHITECTURE.md | 171 | 7KB | yes |
| PRODUCT.md | 71 | 3KB | yes |
| SOUL.md | 68 | 3KB | yes |

Every other root document is comfortably inside. USAGE.md is the single outlier, and it grows by a
line or two with each task that corrects something in it — it grew from 729 to 730 during
[TASK-099](../done/099-usage-md-says-conflicts-are-silently-ignored.md) alone.

Nothing measures this. There is no test, no make target, and no lint step that reads a document's
size, so the standard is advisory in practice and the drift is invisible until someone counts by
hand.

## Why it matters here specifically

The stated audience for these documents includes LLM agents, and the same reasoning that made
[TASK-096](../_archive/096-manifest-static-commands-undercounts.md) a defect applies: an agent that
reads a truncated or partially-attended document concludes the missing part does not exist. A
730-line reference read under a budget is a document whose tail is unreliable — and the tail of
USAGE.md is where the reserved-name rules, the hook table, and the plan `vars` precedence live.

It is also where TASK-099's contradiction survived: two answers to one question, 30 lines apart,
because no reader routinely holds the whole file at once.

## Options

- **A — split by audience.** `USAGE.md` keeps the getting-started path; the reference sections
  (`interaction`, `stack`, `plans`, `vars` precedence, reserved names) move to `docs/` under the
  `doc-naming` priority prefixes, with USAGE.md linking out. Costs: every inbound link, and the
  `AGENTS.md` Documentation Ownership table, need updating in the same commit.
- **B — accept USAGE.md as a recorded exception.** Write the exception down where the standard is
  stated, so the next reader knows the overflow is a decision rather than drift. Costs nothing,
  fixes nothing.
- **C — split, and add the guard that would have caught it.** A test over the root documents that
  fails when one exceeds the standard, with a declared exception list — so an accepted exception is
  a line of code rather than a memory. The guard is the part that stops this recurring.

## Decision needed

~~Which of A / B / C.~~ — see Resolution.

## Acceptance criteria

- [x] Superseded before any criterion applied | verify: `grep -c 'USAGE.md' tasks/done/090-seven-documents-exceed-the-doc-standard-nothing-enforces-it.md` — non-zero, i.e. 090 already covers this file

## Resolution — withdrawn as a duplicate of TASK-090

Filed 2026-07-31, withdrawn the same day. **Not a defect report; a duplicate one.**

[TASK-090](../_archive/090-seven-documents-exceed-the-doc-standard-nothing-enforces-it.md) was
already open in `decision/` asking the same question, and asking it better:

| | this task (106) | TASK-090 |
| --- | --- | --- |
| files measured | 6 root `*.md` | **147 tracked `.md`** |
| documents found over the limit | 1 | **7** |
| headline claim | "USAGE.md is the single outlier" | USAGE.md is one of seven |
| notes the standard is unstated in-repo | no | yes — 0 hits for `500`/`10240`/`doc-standards` |
| costs a split would incur | not measured | 33 inbound refs; `skillgen` link rewriting |
| recommendation | none | B, with reasoning |

**The narrow measurement produced a wrong headline.** "USAGE.md is the single outlier" is true of the
six root documents and false of the repository — `skills/config/references/schema-reference.md` is
larger, at 794 lines / 34708 bytes. Measuring the convenient corpus instead of the real one is how a
subset masquerades as a survey.

Two open tasks asking one question is the defect this repo keeps closing elsewhere — `SOUL.md` 신념 3,
하나의 동작에는 하나의 소유자만 둔다. The owner is 090.

**Carried across to 090** (the one thing here that was not already there): the root-document
comparison, and that `USAGE.md` is still growing — 729/27633 when 090 measured it, **730/27792**
today, having gained a line while [TASK-099](../done/099-usage-md-says-conflicts-are-silently-ignored.md)
corrected a sentence in it.

Nothing else from this file was worth moving.

## Related

- [TASK-090](../_archive/090-seven-documents-exceed-the-doc-standard-nothing-enforces-it.md) — the
  owner of this question. Decide there.
- [TASK-099](../done/099-usage-md-says-conflicts-are-silently-ignored.md) — measured this while closing it;
  the contradiction 099 fixed is the kind of thing document length produces.
- [TASK-096](../_archive/096-manifest-static-commands-undercounts.md) — the same audience argument, applied
  to the command manifest instead of the manual.
