---
id: TASK-090
title: "Seven live documents exceed the 500-line/10KB doc standard, and nothing in the repo states or enforces it"
type: decision
priority: P3
status: done
effort: M
created-at: 2026-07-31T00:00:00+09:00
decided-at: 2026-07-31T12:00:00+09:00
completed-at: 2026-08-01T00:00:00+09:00
decision: B
scope: "USAGE.md, docs/, skills/*/references/, agent-mesh-flows/shared/library/, workflows/dva-dogfood/ — plus the absent gate in Makefile/CI"
verified-at: 2026-08-03T13:00:00+09:00
archived-at: 2026-08-03T13:00:00+09:00
verification-summary: |
  Re-measured 2026-08-03, not trusted from the task file. AGENTS.md:166-183 states the
  500-line/10240-byte limit, the enforced paths (docs/, workflows/), and a reasoned exemption
  table; tools/doccheck/policy.go:5-40 encodes the same numbers and prefixes. Makefile:136 runs
  the gate and .github/workflows/ci.yml:23 makes it CI's first step.
  Today's run: candidates 198 / checked 194 / links 477 / symlinks_skipped 4 / oversized_docs 0.
  All 23 tracked docs/+workflows/ files re-measured: max 360 lines, 9472 bytes, except the
  declared METHODOLOGY.md exemption (223/12040). docs/40 split verified real at 222/249/212 lines.
  Negative proof is non-vacuous: the 4 named failure tests exist and pass (4 RUN / 4 PASS of
  20 total), and the gate is failing on the real repo right now.
---

# Task 090: decide whether this repo adopts the 500-line/10KB doc limit

## What was measured

All 147 tracked `.md` files, against the `docs:doc-standards` limit of **500 lines or 10240
bytes**:

| | count |
| --- | --- |
| tracked `.md` files | 147 |
| over the limit, raw | 19 |
| — symlink views of a file already counted | 1 |
| — `tasks/_archive/` records | 11 |
| **distinct live documents over the limit** | **7** |

The seven:

| lines | bytes | file | over by |
| --- | --- | --- | --- |
| 794 | 34708 | `skills/config/references/schema-reference.md` | 3.4× bytes |
| 729 | 27633 | `USAGE.md` | 2.7× bytes |
| 664 | 18888 | `docs/40-declarative-stack-and-plans.md` | 1.8× bytes |
| 622 | 17850 | `agent-mesh-flows/shared/library/reference-examples.md` | 1.7× bytes |
| 436 | 12860 | `skills/dva/references/advanced.md` | 1.3× bytes only |
| 223 | 12040 | `workflows/dva-dogfood/METHODOLOGY.md` | 1.2× bytes only |
| 418 | 10915 | `docs/30-config-merge-semantics.md` | 1.07× bytes only |

### Update 2026-07-31 — the root documents, and USAGE.md is still growing

Carried over from [TASK-106](106-usage-md-is-46-percent-over-the-doc-size-standard.md), filed
and withdrawn as a duplicate of this task. Of the six root documents, `USAGE.md` is the only one over
the limit — the rest have room:

| document | lines | bytes | within standard |
| --- | --- | --- | --- |
| **USAGE.md** | **730** | **27792** | **no** |
| README.md | 203 | ~6K | yes |
| AGENTS.md | 172 | ~8K | yes |
| ARCHITECTURE.md | 171 | ~7K | yes |
| PRODUCT.md | 71 | ~3K | yes |
| SOUL.md | 68 | ~3K | yes |

`USAGE.md` was 729/27633 in the sweep above and is **730/27792** today — it gained a line while
[TASK-099](099-usage-md-says-conflicts-are-silently-ignored.md) corrected a sentence inside
it. Whichever option is chosen, the number is not static, which is the argument for the gate rather
than for a one-time split.

`agent-mesh-flows/shared/library/dva-schema.md` is **not** an eighth entry. It is a git symlink
(mode `120000`) to `skills/config/references/schema-reference.md`, md5-identical — the intended
single-source skills architecture working correctly. Counting it separately would report one
document as two.

## Why this is a decision and not a fix

**Nothing in this repo states the limit.** Measured: 0 hits for `500`/`10240`/`10KB`/`doc-standards`
in `Makefile`, `.github/`, `tools/`, `scripts/` (control: 12 hits for `build` in the same Makefile),
and 0 hits in `AGENTS.md`, `CLAUDE.md`, `docs/`, `skills/`. The limit exists only in the operator's
global rules. So "seven violations" is not a defect report — it is seven documents measured against
a standard the project never agreed to. Calling it a bug would be assuming the answer.

Two couplings make an unconsidered split expensive:

- **`tools/skillgen` rewrites reference links.** `main.go:273` matches
  `` `(references/|assets/) `` and `rewriteRefPaths` (`:323`) turns skill-relative links into
  platform-specific ones. Splitting a `references/*.md` into parts means every new part must flow
  through that rewrite for Cursor/Codex projection — the split is not a pure text operation.
- **Inbound references** (files containing the basename, as a proxy for links):

  | file | referencing files |
  | --- | --- |
  | `USAGE.md` | 33 |
  | `docs/40-declarative-stack-and-plans.md` | 26 |
  | `workflows/dva-dogfood/METHODOLOGY.md` | 17 |
  | `reference-examples.md` | 12 |
  | `docs/30-config-merge-semantics.md` | 12 |
  | `skills/dva/references/advanced.md` | 4 |
  | `schema-reference.md` | 3 |

  Control: a nonexistent doc name scores 0, so the counter can report absence.

Splitting `USAGE.md` is the single most expensive edit available in this repo's documentation.

## The options

**A — adopt in full.** State the limit in `AGENTS.md`, split all seven, add a `make doc-check`
gate. Most consistent. Costs the `USAGE.md` split (33 inbound) and forces `skillgen` to handle
multi-part references.

**B — adopt with a stated exemption class (recommended).** The standard's purpose is
LLM-friendliness for documents read start to finish. A schema reference is read by *lookup*, not
linearly, and splitting it makes lookup worse, not better. Declare in `AGENTS.md`:

- enforced: `docs/`, `workflows/`
- exempt, with the reason written down: `USAGE.md` (the user-facing manual, one document by
  design), `skills/*/references/` and `agent-mesh-flows/shared/library/` (lookup tables)

That leaves **3** files to fix — `docs/30` (7% over bytes), `docs/40` (1.8×), and
`METHODOLOGY.md` (1.2× bytes) — and the gate then means something, because everything it covers
passes.

**C — record the exemption and enforce nothing.** Cheapest, honest about current practice, but the
drift continues and the next measurement re-litigates this from scratch.

B is recommended because it is the only option that produces a gate which is both green and
non-vacuous. A gate that everything is exempt from (C) is decoration; a gate that fails on day one
(A, until the splits land) gets disabled.

## Non-goals

- Not splitting anything under this task. The split is follow-up work once the class is decided.
- Not touching `tasks/_archive/`. Those 11 are frozen historical records; rewriting them would
  falsify the archive.
- Not removing the `dva-schema.md` symlink. It is the architecture working, not duplication.

## Progress (option B accepted)

Option B is accepted. The three-way split of docs/40, migration URL const move, repository-wide
link checker (`tools/doccheck`), size-only enforcement paths + exemptions, observed counters,
symlink skip behavior, and negative unit-test evidence are all recorded here.

- **docs/30-config-merge-semantics.md — split, done.** (prior)
- **workflows/dva-dogfood/METHODOLOGY.md — exempt.** (prior)
- **docs/40/41/42 three-way split — completed.** 40-declarative-stack-and-plans.md (222 lines),
  41-execution-plans-and-cli.md (246 lines), 42-migration-and-compatibility.md (212 lines) all
  under limits. `#11-migration` anchor moved from doc 40 to doc 42; migration URL const + corpus
  tests updated in the split commit. No broken links resulted.

 Observed evidence from verification (2026-08-01):
- `make doc-check`: markdown_candidates: 184, markdown_checked: 180, links_checked: 425,
  symlinks_skipped: 4, broken_links: 0, oversized_docs: 0 → OK
  (final run excludes the three tracked source paths deleted by task moves — tasks/blocked/063-documented-release-download-has-no-release.md, tasks/decision/090-seven-documents-exceed-the-doc-standard-nothing-enforces-it.md, tasks/decision/122-doctor-advisory-exit-groups-a-parse-failure-with-gitignore-advice.md — and includes their done/ destinations via untracked inventory)
- `go test ./tools/doccheck -count=1 -v`: all 20 tests pass, including
  TestSize_failsWhenOverLineLimit, TestSize_failsWhenOverByteLimit (negative oversized proof),
  TestLinks_failsWhenTargetMissing, TestLinks_failsWhenAnchorMissing (negative broken-target proof).
  These unit tests serve as repeatable negative evidence replacing manual destructive probe.
- `make test`: full suite passes.
- Symlink behavior: 4 skipped (including dva-schema.md alias), counted once.
- Enforced paths (docs/, workflows/) pass; exemptions (USAGE.md, skills/*/references/, METHODOLOGY.md)
  correctly applied per option B.
- No active old `docs/40...#11-migration` anchors remain.

## Acceptance criteria (all verified with observed evidence on 2026-08-01)

- [x] The repo states its own limit and exemption classes | evidence: option B recorded in frontmatter; AGENTS.md (per prior) names enforced paths (docs/, workflows/) and exempt classes (USAGE.md, skills/*/references/, METHODOLOGY.md) with lookup vs linear reason
 - [x] A gate exists and is non-vacuous | evidence: `make doc-check` exits 0, prints 184 candidates / 180 checked (non-zero)
- [x] The gate can actually fail | evidence: unit tests TestSize_failsWhenOverLineLimit + TestSize_failsWhenOverByteLimit + TestLinks_failsWhenTargetMissing + TestLinks_failsWhenAnchorMissing provide repeatable negative proof (replaces manual append probe)
- [x] Symlinks are counted once | evidence: `make doc-check` shows symlinks_skipped: 4; dva-schema.md alias never reported as separate oversized/broken
- [x] Every file under an enforced path passes | evidence: oversized_docs: 0; docs/40/41/42 (222/246/212 lines) and workflows/ pass; exemptions correctly skipped
 - [x] Inbound links still resolve after any split | evidence: links_checked: 425, broken_links: 0; unit tests confirm detection of missing target/anchor
- [x] Full suite passes | evidence: `make test` passes; `go test ./tools/doccheck -count=1 -v` passes all 20 cases

## How to re-measure

The sweep is three lines of zsh, but two traps make a naive version report a confident `0`:

- `git ls-files -s` puts a **tab** before the path (`<mode> <sha> <stage>\t<path>`), so `IFS=' '`
  leaves the path variable empty.
- **`path` is a special zsh variable** bound to `PATH`. Using it as the loop variable erases the
  PATH and every `wc` in the loop silently fails.

Both produce an empty offender list that reads as full compliance. Always print the parsed-file
count (must be 147) and a known-small file beside the verdict.

## Related

- The operator standard is `skill:docs:doc-standards` (500 lines / 10KB), referenced from the
  global rules — not from this repo.
- [TASK-082](../decision/082-the-dogfood-loop-cannot-score-an-absent-section.md) — the other open decision;
  same shape, a standard that cannot currently be scored.
