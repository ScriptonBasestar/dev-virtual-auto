---
id: TASK-053
title: "Portable intermediate skills → per-platform conversion (single source in skills/)"
type: feature
priority: P2
status: todo
effort: M
created-at: 2026-07-22T00:00:00+09:00
scope: "dva repo — skills/, claude-plugin/, .cursor/, AGENTS.md, tools/"
---

# Task 053: Portable skills, one source, many platforms

## Decision (settled)

Skills are authored **once** in the **Agent Skills `SKILL.md` format** at repo
root `skills/` (the canonical/intermediate source). Every other platform artifact
is **generated or symlinked** from it — never hand-maintained in parallel
(single-source-of-truth, consistent with [[dva-config-single-source]] / TASK-052).

Selected targets and their output shapes:

| Target       | Output                                   | Conversion            |
| ------------ | ---------------------------------------- | --------------------- |
| Antigravity  | `skills/` (canonical, read directly)     | none (identity)       |
| Claude Code  | `claude-plugin/skills/` → symlink to `../skills` | none (identity) |
| Cursor       | `.cursor/rules/*.mdc`                     | MDC down-projection   |
| Codex        | `AGENTS.md` (merged section, non-destructive) | AGENTS.md-family |
| OpenCode     | `AGENTS.md` + `.opencode/` (format TBD)   | AGENTS.md-family      |

README.md:176-178 already advertises this layout; `skills/dva/SKILL.md` and
`.cursor/rules/dva.mdc` are currently **dangling** and this task makes them real.

## Field-mapping spec

Canonical is the superset; converters only **down-project**. Reference-only
targets link `references/` by repo path (never inline — ~850 lines would blow
always-injected context budgets). Full spec: `skills/README.md`.

## Phases

- **Phase 1 — Canonical + spec (no tooling)**
  - `git mv` `claude-plugin/skills/{config,dva}` → `skills/{config,dva}` (verbatim).
  - `claude-plugin/skills` → symlink `../skills` (Claude Code keeps working).
  - Write `skills/README.md` (format spec, mapping table, degradation policy).
  - Write `skills/_targets.yaml` (target defs, output paths, default globs).
- **Phase 2 — Validate mapping on ONE target (Cursor)**
  - Hand-write `.cursor/rules/{dva,config}.mdc` per the spec.
  - Add a minimal `x-targets:` override to a canonical `SKILL.md` and confirm the
    `.mdc` reflects it (validates the override mechanism before automating).
- **Phase 3 — Generator + remaining targets** (tooling choice deferred)
  - Build converter → Cursor + AGENTS.md-family (Codex/OpenCode), merge non-destructively.
  - Verify OpenCode/Antigravity file conventions against current docs first.
  - Wire into `make generate` + CI freshness check (`generate` then `git diff --exit-code`).

## Acceptance criteria

- [ ] Canonical skills live at root | verify: `test -f skills/dva/SKILL.md -a -f skills/config/SKILL.md`
- [ ] Claude Code plugin still resolves skills | verify: `test -f claude-plugin/skills/dva/SKILL.md`
- [ ] `claude-plugin/skills` is a symlink, not a copy | verify: `test -L claude-plugin/skills`
- [ ] Format spec exists | verify: `test -f skills/README.md`
- [ ] Target manifest exists | verify: `test -f skills/_targets.yaml`
- [ ] README Cursor reference resolves | verify: `test -f .cursor/rules/dva.mdc`
- [ ] README Antigravity reference resolves | verify: `test -f skills/dva/SKILL.md`
- [ ] No dangling refs to old path | verify: `! grep -rn 'claude-plugin/skills/dva/references' --include=*.go internal cmd`
- [ ] Phase 3 generator wired | verify: human — deferred until tooling decision
