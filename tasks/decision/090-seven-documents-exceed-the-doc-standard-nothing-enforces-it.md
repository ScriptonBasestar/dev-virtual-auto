---
id: TASK-090
title: "Seven live documents exceed the 500-line/10KB doc standard, and nothing in the repo states or enforces it"
type: decision
priority: P3
status: decision
effort: M
created-at: 2026-07-31T00:00:00+09:00
scope: "USAGE.md, docs/, skills/*/references/, agent-mesh-flows/shared/library/, workflows/dva-dogfood/ — plus the absent gate in Makefile/CI"
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

Carried over from [TASK-106](../done/106-usage-md-is-46-percent-over-the-doc-size-standard.md), filed
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
[TASK-099](../done/099-usage-md-says-conflicts-are-silently-ignored.md) corrected a sentence inside
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

Option B is accepted. Status of the work it implies:

- **docs/30-config-merge-semantics.md — split, done.** The worked example and migration note
  (§8–9) moved to `docs/30-config-merge-examples.md`. Semantics is now 345 lines / 9640 bytes;
  examples is 83 lines / 1615 bytes. No inbound link targeted §8–9, so nothing broke. Committed.
- **workflows/dva-dogfood/METHODOLOGY.md — exempt.** A split plan exists (move §"Session and
  resume" to `ref-resume.md`, 2716 bytes), but the file is a load-bearing spec that every one of
  the 9 numbered stages loads whole via `METHODOLOGY = ./METHODOLOGY.md`, and `ref-session.md`
  caches references by path + Git revision. Splitting silently drops the resume protocol from
  the load unless the `METHODOLOGY =` lines and the reuse registry are updated in the same commit
  — a larger change than the limit is worth. Exempting it matches the exemption class's spirit
  (splitting harms the contract), so it joins USAGE.md / skills-references / library.
- **docs/40-declarative-stack-and-plans.md — split pending.** A 3-way plan exists (40 stays as
  entry, 41 = execution-plans-and-cli, 42 = migration-and-compatibility), but `#11-migration` is
  referenced by code, not just docs: `internal/config/validate_warnings.go:17` (URL const),
  `corpus_urls_test.go:221,226`, `validate_warnings_test.go:199`. Moving it requires updating the
  const + the corpus test in the same commit. Filed as the next step before the gate can turn on.

The gate (`make doc-check`), AGENTS.md statement, and `.github/workflows/ci.yml` line are
mechanical once docs/40 is split — they are the last step, not the first, so the gate is green
the moment it lands.

## Acceptance criteria

- [ ] The repo states its own limit and exemption classes | verify: `human — AGENTS.md names the limit, the enforced paths, and the reason each exempt class is exempt`
- [ ] A gate exists and is non-vacuous | verify: `make doc-check` exits 0 AND prints the number of files checked; the count must be non-zero
- [ ] The gate can actually fail | verify: `human — append 11KB to a file under an enforced path, confirm make doc-check exits 1, revert`
- [ ] Symlinks are counted once | verify: `make doc-check` output must list `dva-schema.md` either as skipped or not at all — never as a separate failure from its target
- [ ] Every file under an enforced path passes | verify: `make doc-check` — print the offender count, expect 0
- [ ] Inbound links still resolve after any split | verify: the repo link checker over all `.md` — print links checked AND broken; broken must be 0 with a deliberately bad path proving it can detect one
- [ ] Full suite passes | verify: `make test`

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
- [TASK-082](082-the-dogfood-loop-cannot-score-an-absent-section.md) — the other open decision;
  same shape, a standard that cannot currently be scored.
