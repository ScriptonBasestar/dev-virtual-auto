---
id: TASK-066
title: "Ten live configs warn on section order, and they disagree with canonical order in the same two ways"
type: chore
priority: P4
normalized-by: "TASK-194 — was type: decision"
status: done
effort: S
scope: "Cross-repo: 10 ~/mydevbox/*/dva.yml; possibly internal/config/validate_warnings.go (canonicalSectionOrder). Needs the user's call — their tree"
created-at: 2026-07-30T00:00:00+09:00
verified-at: 2026-08-03T11:52:20+09:00
archived-at: 2026-08-03T11:52:20+09:00
verification-summary: |
  All 7 criteria MET, with the corpus count re-measured rather than trusted.
  `dva validate` (v0.1.44) was run against all 31 live ~/mydevbox/*/dva.yml configs
  matching the literal warning string "section order: found" emitted at
  internal/config/validate_warnings.go:536 — TOTAL_CONFIGS=31, WARNED=0, with the
  non-zero swept count printed beside the verdict so an empty result cannot read as a
  vacuous pass. This confirms the claimed 10 → 0 reduction.
  Option A was taken and Option B genuinely was not: canonicalSectionOrder
  (validate_warnings.go:20-27) is unchanged — checks still at index 9, endpoints still
  near-last — and shared-guardrails.md:54 still teaches the identical order.
  All 10 recorded reorder commits exist in their respective repos with matching
  messages and no uncommitted dva.yml. A spot-checked diff (9b58876) shows the
  endpoints block relocated verbatim, a pure move with no line-level edits.
  TASK-069's "18 warn" figure is a distinct migration check (missing plans: key), so
  the two counts do not contradict.
---

# Task 066: Decide whether the configs or the canonical order is wrong

## Problem

10 of 31 live configs emit a real section-order warning:

```
[warn] semantic: section order: found [...] but canonical order is [...]; consider reordering
```

Cosmetic — `dva validate` still exits 0 and nothing misbehaves. What makes it worth a task
is *how* the 10 disagree: not randomly, but in the **same two ways**, both the reverse of
what `canonicalSectionOrder` prescribes.

| pattern | count | configs |
| --- | --- | --- |
| `endpoints` placed early (right after `stack`) instead of near-last | 5 | scripton-nd-stack, scripton-signalhub, scripton-zai-batch, scripton-zai-review, sigdock-idp |
| `checks` placed last instead of index 9 (before `modes`) | 5 | flow-agent-mesh, hek, matdosa, primeno1, sigdock-idp |
| `environments` placed after `modes` instead of before `checks` | 3 | flow-agent-mesh, matdosa, sadawiki |

(sigdock-idp shows both of the first two, so the rows overlap; 10 distinct configs total.)

Both majority patterns are defensible as written. `endpoints` describes the services `stack`
declares, so putting it adjacent to `stack` reads well; `checks` is a verification concern,
which is naturally written last. Canonical order says the opposite for both.

## Root cause — the constant may be arbitrary where it matters

`internal/config/validate_warnings.go:20-27`. `checks` sits at index 9 inside a block
commented *"Legacy sections retain a deterministic position during migration"* — placed to be
**deterministic**, not because it reads better. `endpoints` sits at index 20 with no stated
rationale either. So this is not obviously "10 configs are untidy"; it is equally readable as
a prescription that never justified two of its positions, contradicted by a third of the corpus.

## The decision

**A. Normalize the 10 configs** to canonical order. Mechanical, key reordering only, no
semantic change. Makes the tool's advice and the corpus agree. Cost: 10 files in the user's
tree, and it is churn in service of a warning nobody is blocked by.

**B. Reorder `canonicalSectionOrder`** to match how configs are written. A trap — see the
Resolution; it converts silent-and-correct configs into warning ones.

**C. Accept and close.** A `consider reordering` suggestion on a passing validate.

**Recommendation: C, or A if the noise is bothersome.**

## Non-goals

- Do not change `canonicalSectionOrder` without deciding B on its merits — `CanonicalSectionOrder()`
  is injected into `agent-mesh-flows/shared/library/shared-guardrails.md` by `tools/libgen`,
  so reordering it changes what the AI generator teaches, not just what validate says.
- Do not reorder keys and edit values in the same pass; a reordering diff must be reviewable
  as a pure move.
- Do not raise the severity. Nothing here breaks a run.

## Acceptance criteria

- [x] Option chosen and recorded here | verify: `human — A, chosen by the user against the C recommendation`
- [x] No live config warns on section order | verify: `n=$(/usr/bin/find ~/mydevbox -maxdepth 2 -name dva.yml -not -path '*/node_modules/*' | wc -l | tr -d ' '); [ "$n" -gt 0 ] || { echo "no dva.yml under ~/mydevbox — nothing was measured"; exit 2; }; out=$(/usr/bin/find ~/mydevbox -maxdepth 2 -name dva.yml -not -path '*/node_modules/*' | while read -r f; do (cd "$(dirname "$f")" && dva validate 2>&1 | /usr/bin/grep -F 'section order: found'); done); echo "configs=$n warnings=$(printf '%s' "$out" | /usr/bin/grep -c .)"; [ -z "$out" ]` — **`configs=25 warnings=0`, exit 0 (TASK-199).** The binding was `… | while read f; do (cd … && dva validate 2>&1 | grep -F 'section order: found'); done` — expect no output over 31 configs. The loop's status is the last `grep`'s and `grep` exits 1 when it finds nothing, so as published it reported **failure on exactly the healthy state** the criterion describes, and would have reported success only once a config started warning. Both directions are demonstrated now: against a two-config fixture whose second file orders `stack:` before `vars:` it prints `configs=2 warnings=1` and exits 1. The third exit code is the one the original could not have: `~/mydevbox` is a machine-local corpus, so a reader elsewhere gets `nothing was measured` and **exit 2** rather than a silent pass over zero files. The denominator is printed rather than asserted because it drifts — the recorded `31` measures `25` on 2026-08-20, with nothing about section order having changed
- [x] Every touched config still validates | verify: `human — dva validate rc=0 in each of the 10; measured below`
- [x] Each reorder is a pure move, not an edit | verify: `human — per-file line multiset and yaml.safe_load equality against git HEAD; measured below`
- [x] No comment travels with the wrong section | verify: `human — all 14 moved comment lines reviewed; see Resolution`

## Evidence

Measured 2026-07-30 across 31 configs with the sweep in criterion 2, matching DVA's literal
warning text: `section order: found`, the exact string emitted at
`validate_warnings.go:536`. An earlier
pass grepped for `'order|section'` instead and reported 18 configs — those hits were the
`modes` / `stack.*.order` / `applications` **migration** warnings, an entirely different
check. The pattern matched the word, not the finding.

### Also checked in the same pass, and clean

Recorded so they are not re-investigated:

- **dripter nginx publishing `16206:443`** — refuted. No nginx port rows exist in any of the
  5 compose files. The first search found 1 of 5 files because Python's `glob('**/compose*')`
  skips hidden directories; `/usr/bin/find` found all 5.
- **dripter jaeger `6831`/`6832`/`14250`/`14268` undeclared in `endpoints:`** — absent but not
  a defect: ingest ports (two UDP), not browsable URLs. Jaeger's UI port is declared.
- **`.sb/dva` gitignore hygiene** — a real defect, in DVA not in the configs; fixed in
  [TASK-065](065-gitignore-check-misses-ancestor-rules.md).

## Resolution

**Option A**, chosen by the user against this task's own C recommendation. Executed as one
mechanical pass on 2026-07-30. Section-order warnings across the corpus: **10 → 0 of 31.**

### Correction to the Problem section: it is 8 repos, not 10

`scripton-zai-batch` and `scripton-zai-review` are **git worktrees of
`scripton-nd-stack-devbox`** — same remote, `gitdir` under
`scripton-nd-stack-devbox/.git/worktrees/`, checked out on branches `zai/batch` and
`zai/review`. Their `dva.yml` files are byte-identical to nd-stack's (same sha), which the
table above read as three independent configs making the same mistake. It is one file seen
from three branches.

The identical content is why all three were still reordered rather than only `develop`:
a 3-way merge of the *same* change on both sides resolves without conflict, whereas fixing
only `develop` would leave the two feature branches warning and conflicting on a 56-line
block move later.

### What was committed

The 10 working trees each already carried an uncommitted `dva.yml` from earlier steps in the
same session (the `$schema` URL fix and the `stack.compose` → `runners.compose` migration).
Committing the reorder on top would have buried a pure move inside a content change, so the
earlier work landed first, in its own commit per repo.

| repo | pre-existing work | reorder |
| --- | --- | --- |
| flow-agent-mesh-devbox | `4da893b` | `9415860` |
| hek-devbox | `f0f50d5` | `0db2783` |
| matdosa-devbox | `f252d95` | `ebbb696` |
| primeno1-devbox | `1301e80` | `1b3530c` |
| sadawiki-devbox | `622b2dc` | `04eb66f` |
| scripton-nd-stack-devbox | `7f8a07c` | `9b58876` |
| scripton-signalhub-devbox | `ae228b0` | `aebf7b9` |
| scripton-zai-batch | `f16b914` | `e38f5eb` |
| scripton-zai-review | `055eb63` | `5838d37` |
| sigdock-idp-devbox | — (was clean) | `1416ea9` |

Every commit used an explicit `-- dva.yml` pathspec. 9 of the 10 repos carry unrelated
uncommitted work of the user's — `CLAUDE.md`, `README.md`, a staged `compose.yml →
compose.yaml` rename, `package.json` — all with mtimes of 2026-07-11 or earlier against
`dva.yml`'s 12:51 today, which is how ownership was established. None of it was swept in;
the post-commit dirty counts are unchanged.

One consequence: in `matdosa-devbox` and `sadawiki-devbox` the config now references
`compose.yaml` while the rename creating that name is still only *staged* by the user. Their
working trees are fine; the intermediate commit is not self-consistent. Left alone —
committing someone else's staged rename is out of scope.

### How "pure move" was verified

Not by eyeballing the diff. Per file, against `git show HEAD:dva.yml`:

1. the multiset of non-blank lines is identical (175/175, 88/88, 166/166, 254/254, 201/201,
   462/462 ×3, 303/303, 261/261),
2. `yaml.safe_load(before) == yaml.safe_load(after)`,
3. the top-level key order did change — guarding against a no-op that would have "passed"
   both checks above.

(2) is what makes the other warning categories safe to ignore: identical parsed config means
every content-derived warning is unchanged by construction, so the 6-69 remaining warnings
per config cannot have shifted. `dva validate` has no file argument, so there was no way to
run the validator against the old bytes in place; deep equality replaces that measurement
rather than approximating it.

**The invariants above cannot catch comment misattribution.** A comment run directly above a
key was assumed to document that key, which is wrong if it was really a trailing note for the
section before it, or a banner spanning a group. 14 comment lines moved in total; none
contains a positional word (`above`, `below`, `following`, `next`, `previous`, `first`,
`last`, and the Korean equivalents), and each is a header naming the section it precedes —
`# Modes — HOW to run infrastructure` over `modes:`, `# --- Endpoints ---` over `endpoints:`,
the symmetric `# ----- / # Environments / # -----` triples over `environments:`. 4 of the 10
files moved no comment at all.

### One change beyond the reorder, stated rather than hidden

The three nd-stack-family files diff as `+55/-57`. The two-line gap is **EOF whitespace**:
they ended with three blank lines and now end with one. No key, value, or comment is
affected, and the non-blank multiset is identical — but it is not literally a move, so it is
recorded here instead of passing as one.

### Measurement note

`for d in $REPOS` silently did nothing: zsh does not word-split unquoted parameter expansions,
so the whole list arrived as one directory name. **Third** firing of that trap this session
(see TASK-062); the tell was `git` reporting one absurd path rather than ten.

### B is still untaken

Normalizing the corpus removes the evidence that made B tempting, but not the argument
against it: `endpoints` has no position that satisfies every config, because sadawiki and
primeno1 place it last on purpose. `canonicalSectionOrder` remains unchanged, so
`shared-guardrails.md` still teaches the same order it did.
