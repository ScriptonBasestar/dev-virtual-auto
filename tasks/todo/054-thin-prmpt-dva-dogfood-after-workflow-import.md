---
id: TASK-054
title: "Make prmpt packages/dva/dogfood reference-only after importing the workflow into the dva repo"
type: chore
priority: P2
status: todo
effort: S
created-at: 2026-07-22T00:00:00+09:00
scope: "Cross-repo: prmpt (scripton/iac/devenv) — must be applied from a prmpt-scoped session"
---

# Task 054: Make prmpt reference-only for the DVA dogfood workflow

## Decision (settled)

The DVA dogfood/improvement workflow (audit skill → improve skill/prompts/tool →
apply → evaluate → feedback) is now **canonical in the dva repo** at
`workflows/dva-dogfood/`, decoupled from prmpt's CE controller / catalog / gateway.
`prmpt` keeps a lightweight **pointer**, not a second copy — no dual management
(same principle as [[dva-config-single-source]] / TASK-052).

**Primary rationale (access, not just single-source):** the dva repo is the
**public** distributable (`github.com/ScriptonBasestar/dev-virtual-auto`); `prmpt`
is in the **private, internal** `gitlab.polypia.net/scripton/iac/devenv`. DVA users
receive the dva repo, never devenv — so any DVA workflow they should run must be
canonical in the accessible repo. The import was decoupled to plain, gateway-free
Markdown precisely so it runs with the dva repo alone. Only `packages/dva` is
DVA-specific; prmpt's gateway/catalog/~31 non-DVA packages are general framework
infrastructure and stay in devenv.

Imported into the dva repo (canonical): `workflows/dva-dogfood/` — `00-start-cycle`
… `70-feedback`, `ref-*.md`, `METHODOLOGY.md`, `README.md`. The CE/gateway glue
(`entry.md`, `RUN.md`, `operate/RUN.md`, `contract/`) was intentionally **not**
imported — it is prmpt-framework-specific.

## Why this can't be done from the dva-repo session

Cross-repo writes to `/Users/archmagece/devenv/prmpt` are blocked by this session's
project-dir permission boundary. Apply the change-set below from a session scoped to
the prmpt repo (GitLab `scripton/iac/devenv`).

## Change-set (prmpt)

1. **`packages/dva/dogfood/`** — replace the duplicated workflow bodies (numbered
   `prmpt-NN-*.md` stage prompts + `ref-*.md`) with a **pointer README**. The pointer
   MUST use the **canonical GitHub URL**, not a local path or a bare "the DVA repo"
   mention (unresolvable for a reader):
   `https://github.com/ScriptonBasestar/dev-virtual-auto/tree/master/workflows/dva-dogfood`.
   Keep only what the prmpt framework genuinely needs to run its own dogfood (see
   open questions).

2. **CE dogfood adapter / contract** — decide (open question) whether prmpt still runs
   the DVA dogfood through `core.component-dogfood` + `contract/dogfood-manifest.yaml`.
   - If yes: the contract stays in prmpt, but its stage content should reference the
     dva-repo prompts, not re-embed them.
   - If no: drop `packages/dva/dogfood/contract/` and the catalog `dogfood` block.

3. **`packages/dva/operate/`** — unchanged. It is a generated gateway launcher for the
   `tool:dva-config` skill, whose canonical source already lives in the dva repo
   (`skills/config`). Single-source already holds here.

4. **`catalog.yaml`** — if the dogfood block is dropped, update the `dva` entry and
   re-run prmpt's catalog/schema validation.

## Open questions (resolve in the prmpt session)

- Does prmpt want to keep executing the DVA dogfood via its CE framework (pointer that
  reads dva-repo prompts), or fully hand off execution to the dva repo?
- `packages/devbox/` was **not** imported (more devenv-specific: improves plugin +
  local prompts). Decide separately whether any of it is DVA-owned.

## Acceptance criteria

- [ ] prmpt `packages/dva/dogfood/` no longer embeds the workflow bodies | verify: human — prmpt session
- [ ] prmpt points to dva repo `workflows/dva-dogfood/` as canonical | verify: human — prmpt session
- [ ] prmpt catalog/schema validation passes after the change | verify: human — prmpt session
