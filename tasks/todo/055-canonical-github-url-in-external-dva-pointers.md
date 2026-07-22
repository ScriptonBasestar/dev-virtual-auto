---
id: TASK-055
title: "External DVA pointers (ce-plugin, prmpt) must carry the canonical GitHub URL, not a bare mention"
type: chore
priority: P3
status: todo
effort: XS
created-at: 2026-07-22T00:00:00+09:00
scope: "Cross-repo: claude-ce-plugin + prmpt — apply from each repo's own session"
---

# Task 055: Canonical GitHub URL in external DVA pointers

## Problem

The reference-only pointers that other repos keep for DVA content name it only as
"the DVA repo / dva plugin" — with **no address and no path**. A reader (or agent)
in `ce-plugin` or `prmpt` can't resolve *which* repo or *which* file. Local
filesystem paths (`/Users/...`) are worse — machine-specific and unresolvable.

Every external pointer must use the **canonical GitHub URL**.

## Canonical URLs (source of truth: dva repo `skills/README.md`, `workflows/README.md`)

- Repo: `https://github.com/ScriptonBasestar/dev-virtual-auto`
- Config skill: `.../blob/master/skills/config/SKILL.md`
- CLI skill: `.../blob/master/skills/dva/SKILL.md`
- Dogfood workflow: `.../tree/master/workflows/dva-dogfood`

## Change-set

1. **claude-ce-plugin** (TASK-052 already removed the skill; only the pointer needs
   enriching — apply from a ce-plugin session):
   - `src/plugins/INDEX.md:201` and `src/plugins/tool/CLAUDE.md:31` currently say
     "DVA 리포의 dva 플러그인이 canonical 소스." Append the GitHub URL + skill path so
     the pointer is resolvable. Re-run `./build/ce` so generated outputs
     (`plugins/`, `.opencode/`, `build/codex/`) carry the URL too.

2. **prmpt** — folded into TASK-054: the new pointer README uses the workflow GitHub
   URL above (already specified there).

## Acceptance criteria

- [ ] ce-plugin DVA pointer contains the canonical GitHub URL | verify: human — ce-plugin session
- [ ] prmpt pointer contains the workflow GitHub URL | verify: human — prmpt session (see TASK-054)
