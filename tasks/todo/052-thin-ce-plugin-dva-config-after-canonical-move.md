---
id: TASK-052
title: "Thin or remove claude-ce-plugin tool-dva-config after DVA repo becomes canonical"
type: chore
priority: P3
status: todo
effort: S
created-at: 2026-07-22T00:00:00+09:00
scope: "Cross-repo: claude-ce-plugin (source of tool-dva-config)"
---

# Task 052: Retire ce-plugin `tool-dva-config` after canonical move

## Summary

The `dva:config` skill now lives canonically in this repository at
`claude-plugin/skills/config/`, ported from `claude-ce-plugin`'s
`tool-dva-config`. Two copies now exist and will drift. This task retires the
ce-plugin copy so DVA configuration knowledge has a single owner.

## Context

- Canonical (new): `claude-plugin/skills/config/SKILL.md` (this repo).
- Source (to retire): `~/mywork/scripton/claude-ce-plugin/src/plugins/tool/skills/dva-config/`
  (built into `plugins/tool/skills/dva-config/` and cached under
  `~/.claude/plugins/cache/claude-ce-plugin/tool/0.1.0/skills/dva-config/`).
- The port was made from source commit `dff2de3`
  (`docs(dva-config): document dva stack up minimal-default via compose profiles`),
  which the cache did not yet contain.

## Options (decide in ce-plugin repo)

1. **Remove** `tool-dva-config` from ce-plugin entirely; rely on the DVA plugin.
2. **Thin to a pointer** — leave a short stub noting the skill moved to the DVA
   plugin, keeping the `chains-with` graph intact for other ce-plugin skills.

## Acceptance criteria

- [ ] ce-plugin no longer ships a full-content `tool-dva-config` SKILL.md (removed or stubbed).
- [ ] ce-plugin build output and cache regenerated so the stale full copy is gone.
- [ ] Any ce-plugin `chains-with`/docs references to `tool:dva-config` resolved (updated or removed).
- [ ] Note recorded that the DVA repo `claude-plugin/skills/config/` is the source of truth.

## Notes

- Cross-repo change — do not edit the ce-plugin repo from a DVA-repo session
  without explicit scope. This task only records the follow-up.
- If the two copies must coexist for a while, capture a divergence-check step so
  future edits land in the DVA repo first.
