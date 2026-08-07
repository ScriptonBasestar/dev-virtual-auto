---
id: TASK-054
title: "Make prmpt packages/dva/dogfood reference-only after importing the workflow into the dva repo"
type: chore
priority: P2
status: done
effort: S
created-at: 2026-07-22T00:00:00+09:00
completed-at: 2026-08-07
scope: "Cross-repo: ~/workflow (prmpt) pointer; DVA repo remains SSoT"
---

# Task 054: Make prmpt reference-only for the DVA product dogfood

## Ownership decision (accepted 2026-08-07)

| Asset | SSoT |
|-------|------|
| DVA product skills | DVA repo `skills/` |
| DVA product dogfood loop | DVA repo `workflows/dva-dogfood/` |
| CE contract dogfood of installed DVA CLI | `~/workflow/prmpt/packages/dva/dogfood/contract/` |
| Operate stages (CLI inspect/lifecycle) | `~/workflow/prmpt/packages/dva/operate/` |
| ce-plugin | pointer / projection only — not SSoT |

Product improvement dogfood stays in the **public** DVA repo so users without the
workflow corpus can run it. workflow/prmpt keeps framework adapters only.

## Path note

Original card used `/Users/archmagece/devenv`. Canonical workflow corpus is now
`/Users/archmagece/workflow` (`prmpt/` under it).

## What was already true

Numbered `prmpt-NN-*.md` / `ref-*.md` product-loop mirrors are **gone** from
`prmpt/packages/dva/dogfood/` (only contract, entry, RUN, evidence, README remain).

## What this disposition completed

1. Rewrote `prmpt/packages/dva/dogfood/README.md` as ownership table + GitHub pointer:
   `https://github.com/ScriptonBasestar/dev-virtual-auto/tree/master/workflows/dva-dogfood`
2. Fixed stale `entry.md` line claiming `prmpt-*` files remain.
3. Recorded ownership so ce-plugin vs DVA vs workflow is not re-litigated without new facts.

## Acceptance criteria

- [x] prmpt `packages/dva/dogfood/` no longer embeds product-loop stage bodies
- [x] prmpt points to dva repo `workflows/dva-dogfood/` as canonical product dogfood
- [x] Ownership split recorded (contract/operate stay in workflow)

Catalog validation of the full workflow repo is that repo's CI; this package's
structure is pointer + existing contract only — no catalog block removal required
for the product-loop cutover (loop never lived in catalog as those files).
