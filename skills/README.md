# Portable Skills — one source, many platforms

This directory is the **single source of truth** for DVA's AI skills. Skills are
authored once here in the **Agent Skills `SKILL.md` format** and down-projected to
every other platform. Never hand-maintain a platform copy in parallel — edit the
canonical skill and regenerate (single-source-of-truth; see TASK-053).

## Layout

```
skills/
  <name>/
    SKILL.md          # canonical: YAML frontmatter + Markdown body
    references/       # progressive-disclosure detail (loaded on demand)
    assets/           # templates / files the skill ships
  _targets.yaml       # conversion manifest: targets, output paths, defaults
  README.md           # this spec
```

## Why Agent Skills format is canonical

`SKILL.md` is the **superset**. Only it supports progressive disclosure
(`references/`), bundled `assets/`, and tool scoping (`allowed-tools`). Every other
target is a strict subset, so converters only ever **down-project** — never the
reverse. Two targets read it with **zero conversion**:

- **Antigravity** reads `skills/<name>/SKILL.md` directly.
- **Claude Code** reads it via `claude-plugin/skills` → symlink to `../skills`.

## Canonical frontmatter

```yaml
---
name: dva                       # identity (required)
description: >-                  # model-facing "use when…" trigger (required)
  Use when the user asks to build, run tests, start services...
allowed-tools: [Bash, Read]     # Claude Code only; dropped elsewhere
user-invocable: false           # Claude Code only; dropped elsewhere
x-targets:                       # OPTIONAL per-target overrides (extension key)
  cursor: { globs: ["dva.yml", "**/dva.yml"], alwaysApply: false }
---
```

`x-targets` is an extension key. Claude Code and Antigravity ignore unknown
frontmatter keys, so it is safe in canonical files. A skill's own `x-targets`
**wins over** the `defaults` in `_targets.yaml`.

## Field mapping (canonical → target)

| Canonical            | Claude Code / Antigravity | Cursor `.mdc`      | AGENTS.md-family (Codex/OpenCode) |
| -------------------- | ------------------------- | ------------------ | --------------------------------- |
| `name`               | `name`                    | filename           | section heading                   |
| `description`        | `description`             | `description`      | prose "use when…" line            |
| `x-targets.*.globs`  | —                         | `globs`            | —                                 |
| `x-targets.*.alwaysApply` | —                    | `alwaysApply`      | (always present in file)          |
| `allowed-tools`      | `allowed-tools`           | dropped            | dropped                           |
| `user-invocable`     | `user-invocable`          | dropped            | dropped                           |
| `references/`        | native (on demand)        | **linked by path** | **linked by path**                |
| `assets/`            | native                    | linked by path     | linked by path                    |
| body                 | body                      | body               | merged section (non-destructive)  |

## Reference-degradation policy

Reference/instruction targets are **always-injected context** (token cost on every
request). DVA's `references/` is large (~850 lines), so converters **link
references by repo path — never inline them**. This preserves the
progressive-disclosure intent as far as each target allows: the compact body stays
in the rule, deep detail stays one click away in `skills/<name>/references/`.

## Targets

Defined in [`_targets.yaml`](./_targets.yaml). Current set: Antigravity, Claude
Code (both identity), Cursor (`.mdc`), Codex + OpenCode (`AGENTS.md`-family, merged
non-destructively). OpenCode's `.opencode/` layout is **unverified** — confirm
against current docs before the generator emits it.

## Workflow

1. **Author / edit** `skills/<name>/SKILL.md` (+ `references/`, `assets/`).
2. **Set overrides** in the skill's `x-targets` or in `_targets.yaml` `defaults`.
3. **Generate** platform artifacts (Phase 3 — `make generate`; until then Cursor
   is hand-converted per this spec).
4. **Verify** generated outputs are fresh in CI (`generate` then `git diff --exit-code`).

Do not edit generated artifacts (`.cursor/rules/*`, the `AGENTS.md` skills section,
`claude-plugin/skills`). Edit the canonical skill and regenerate.
