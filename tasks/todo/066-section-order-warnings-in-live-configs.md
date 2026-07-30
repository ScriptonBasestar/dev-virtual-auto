---
id: TASK-066
title: "Ten live configs warn on section order, and they disagree with canonical order in the same two ways"
type: decision
priority: P4
status: todo
effort: S
scope: "Cross-repo: 10 ~/mydevbox/*/dva.yml; possibly internal/config/validate_warnings.go (canonicalSectionOrder). Needs the user's call — their tree"
created-at: 2026-07-30T00:00:00+09:00
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
commented *"Legacy sections retain a deterministic position during migration"* — that is, it
was placed to be **deterministic**, not because that position reads better. `endpoints` sits
at index 20, after `subprojects`, with no stated rationale either.

So this is not obviously "10 configs are untidy". It is equally readable as a prescription
that never justified two of its positions, now contradicted by a third of the corpus.

## The decision

**A. Normalize the 10 configs** to canonical order. Mechanical, key reordering only, no
semantic change. Makes the tool's advice and the corpus agree. Cost: 10 files in the user's
tree, and it is churn in service of a warning nobody is blocked by.

**B. Reorder `canonicalSectionOrder`** to match how configs are actually written. **This is
a trap and is not recommended.** Moving `endpoints` early would silence 5 configs and start
warning the 2 that correctly place it last (sadawiki, primeno1). No single position satisfies
the corpus — which is itself evidence the ordering is a style preference rather than a rule.

**C. Accept and close.** It is a `consider reordering` suggestion on a passing validate. The
warning already communicates everything a reader needs.

**Recommendation: C, or A if the noise is bothersome.** B is worse than doing nothing
because it converts silent-and-correct configs into warning ones. If A is chosen, do it as
one mechanical pass and re-run the sweep in Evidence to confirm 0 remaining.

## Non-goals

- Do not change `canonicalSectionOrder` without deciding B on its merits — `CanonicalSectionOrder()`
  is injected into `agent-mesh-flows/shared/library/shared-guardrails.md` by `tools/libgen`,
  so reordering it changes what the AI generator teaches, not just what validate says.
- Do not reorder keys and edit values in the same pass; a reordering diff must be reviewable
  as a pure move.
- Do not raise the severity. Nothing here breaks a run.

## Acceptance criteria

- [ ] Option chosen and recorded here | verify: `human — decision recorded`
- [ ] If A: no live config warns on section order | verify: `human — re-run the Evidence sweep; expect 0`
- [ ] If A: every touched config still validates | verify: `human — dva validate in each of the 10, expect rc=0`
- [ ] If C: closed with the count recorded | verify: `human — this file`

## Evidence

Measured 2026-07-30 across 31 configs, matching DVA's literal warning text:

```
/usr/bin/find . -maxdepth 2 -name dva.yml -not -path '*/node_modules/*' |
  while read f; do (cd "$(dirname "$f")" && dva validate 2>&1 |
    /usr/bin/grep -F 'section order: found'); done
```

`section order: found` is the exact string emitted at `validate_warnings.go:536`. An earlier
pass grepped for `'order|section'` instead and reported 18 configs — those hits were the
`modes` / `stack.*.order` / `applications` **migration** warnings, an entirely different
check. The pattern matched the word, not the finding.

### Also checked in the same pass, and clean

Recorded so they are not re-investigated:

- **dripter nginx publishing `16206:443`** — refuted. No nginx port rows exist in any of the
  5 compose files. The first search found 1 of 5 files because Python's `glob('**/compose*')`
  skips hidden directories; `/usr/bin/find` found all 5.
- **dripter jaeger `6831`/`6832`/`14250`/`14268` undeclared in `endpoints:`** — confirmed
  absent, but not a defect: these are ingest ports (two UDP), not browsable URLs. `endpoints:`
  advertises things a human opens. Jaeger's UI port is declared.
- **`.sb/dva` gitignore hygiene** — this one was a real defect, in DVA rather than in the
  configs, and is fixed in [TASK-065](../done/065-gitignore-check-misses-ancestor-rules.md).
