# Portable Skills — one source, many platforms

This directory is the **single source of truth** for DVA's AI skills. Skills are
authored once here in the **Agent Skills `SKILL.md` format** and down-projected to
every other platform. Never hand-maintain a platform copy in parallel — edit the
canonical skill and regenerate (single-source-of-truth; see TASK-053).

**Canonical location for external references** — repos that reference these skills
(e.g. `claude-ce-plugin`, `prmpt`) MUST point at the GitHub URL, never a local
filesystem path (machine-specific, unresolvable elsewhere):

> `https://github.com/ScriptonBasestar/dva` → `skills/<name>/SKILL.md`
> e.g. `https://github.com/ScriptonBasestar/dva/blob/master/skills/dva-config/SKILL.md`

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
reverse. **Three targets read the format itself** (zero conversion) — each is a
symlinked directory pointing at canonical `skills/`, so an edit propagates with no
regeneration:

- **Claude Code** — `claude-plugin/skills` → `../skills`. Zero conversion, but **not
  auto-discovered**: this repo ships no `.claude/skills/`, so the skills reach a session
  only as part of the plugin, loaded per session with `claude --plugin-dir ./claude-plugin`
  (`README.md`, `claude-plugin/README.md`). No marketplace manifest ships, so there is no
  persistent install — the symlink is necessary, not sufficient.
- **Antigravity** — `.agents/skills` → `../skills` (reads `.agents/skills/<name>/SKILL.md`,
  **not** bare root `skills/`; `references/` supported). Ref: antigravity.google/docs/skills
- **OpenCode** — `.opencode/skills` → `../skills` (native Agent Skills at
  `.opencode/skills/<name>/SKILL.md`; unknown frontmatter ignored; `name` must match
  the dir). Ref: opencode.ai/docs/skills

Only **Cursor** (`.mdc`) and the repository's **Codex compatibility projection** (`AGENTS.md`)
require real conversion. Current Codex can also consume Agent Skills natively; the binary
installer uses `.agents/skills` instead of editing instruction files.

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

`x-targets` is an extension key. Agent-Skills consumers (Claude Code, Antigravity,
OpenCode) ignore unknown frontmatter keys, so it is safe in canonical files. A
skill's own `x-targets` **wins over** the `defaults` in `_targets.yaml`.

## Field mapping (canonical → target)

Agent-Skills targets (Claude Code, Antigravity, OpenCode) consume the file as-is via
symlink — no field mapping. Only the two converted targets remap fields:

| Canonical            | Agent-Skills targets | Cursor `.mdc`      | Codex (`AGENTS.md`) |
| -------------------- | -------------------- | ------------------ | ------------------- |
| `name`               | as-is                | filename           | section heading     |
| `description`        | as-is                | `description`      | prose "use when…" line (compatibility projection) |
| `x-targets.*.globs`  | ignored              | `globs`            | —                   |
| `x-targets.*.alwaysApply` | ignored         | `alwaysApply`      | (always present)    |
| `allowed-tools`      | as-is / ignored      | dropped            | dropped             |
| `user-invocable`     | as-is / ignored      | dropped            | dropped             |
| `references/`        | native / on-demand   | **linked by path** | (body not inlined)  |
| `assets/`            | native               | linked by path     | (body not inlined)  |
| body                 | as-is                | inlined (lazy-loaded) | pointer-only (always-injected) |

## Reference-degradation policy (asymmetric by load model)

The two converted targets differ in how their content is loaded, so they degrade
differently:

- **Cursor** rules are **lazy-loaded** on `globs` match → the generator **inlines
  the full body** but rewrites `references/`/`assets/` links to repo-root paths
  (Claude-only `` !`cmd` `` dynamics and now-empty sections are stripped).
- **Codex** `AGENTS.md` is **always-injected** → each skill degrades to a
  **pointer-only** entry (description + link to `skills/<name>/SKILL.md`), never the
  body. `references/` are never inlined anywhere.

## Targets

Defined in [`_targets.yaml`](./_targets.yaml), all verified against current docs:

| Target       | Output                          | How            |
| ------------ | ------------------------------- | -------------- |
| Claude Code  | `claude-plugin/skills`          | symlink (load: `--plugin-dir`) |
| Antigravity  | `.agents/skills`                | symlink (committed) |
| OpenCode     | `.opencode/skills`              | symlink (gitignored → local) |
| Cursor       | `.cursor/rules/*.mdc`           | generated (gitignored → local) |
| Codex compatibility | `AGENTS.md` marked section | generated (committed) |

`make generate` reproduces every row (symlinks are ensured idempotently), so a
fresh clone materializes the gitignored ones locally.

## Workflow

1. **Author / edit** `skills/<name>/SKILL.md` (+ `references/`, `assets/`).
2. **Set overrides** in the skill's `x-targets` or in `_targets.yaml` `defaults`.
3. **Generate** platform artifacts (Phase 3 — `make generate`; until then Cursor
   is hand-converted per this spec).
4. **Verify** generated outputs are fresh in CI (`generate` then `git diff --exit-code`).

## Binary installation

`make generate` projects skills only inside a DVA source checkout. It is not a user-level
installer, and `make install` installs only the `dva` binary. Released binaries embed the two
canonical skill directories and expose deterministic installation without an AI agent:

```bash
dva skill install
dva skill status
dva skill uninstall
```

The installer copies `dva` and `dva-config` to native runtime discovery paths, records hashes
under the user's XDG state directory, refuses unmanaged collisions, and removes only unchanged
DVA-owned files. Agent Mesh is supported by the same `dva skill install` command: it renders
`dva.md` and `dva-config.md` into its dedicated `skills/dva` namespace, strips frontmatter, and
inlines Markdown references and text assets in deterministic order. Relative bundle support is
not assumed, so local paths link to their inlined content. DVA owns file installation and receipt
verification only; `am skill sync` and its interaction with other targets are outside this
contract. See [USAGE.md](../USAGE.md#ai-스킬-설치) for runtime paths and scope options. The
Antigravity CLI flat format remains outside this contract.

Shared runtime roots are producer-neutral. DVA writes a versioned XDG claim for each installed
top-level `dva` skill, never for the root itself; a different producer's claim is a hard conflict.
Receipt-less DVA-name trees remain foreign unless `dva skill install --takeover` is explicit. That
option first writes a durable, manifest-verified backup of regular bytes, paths, empty directories,
and permission bits; it rejects symlinks and special files. Ordinary uninstall preserves that backup
as a backup-only receipt tombstone; restoration is the separate explicit
`dva skill uninstall --restore-takeover-backup` operation.

Do not edit generated artifacts (`.cursor/rules/*`, the `AGENTS.md` skills section,
`claude-plugin/skills`). Edit the canonical skill and regenerate.
