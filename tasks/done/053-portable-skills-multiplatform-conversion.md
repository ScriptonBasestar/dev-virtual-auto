---
id: TASK-053
title: "Portable intermediate skills → per-platform conversion (single source in skills/)"
type: feature
priority: P2
status: done
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

Selected targets — corrected against official docs in Phase 3 (OpenCode and
Antigravity both consume the Agent Skills format natively; they are symlinks, not
`AGENTS.md`):

| Target       | Output                              | Conversion        | Committed? |
| ------------ | ----------------------------------- | ----------------- | ---------- |
| Claude Code  | `claude-plugin/skills` → `../skills` | symlink (none)    | yes        |
| Antigravity  | `.agents/skills` → `../skills`       | symlink (none)    | yes        |
| OpenCode     | `.opencode/skills` → `../skills`     | symlink (none)    | no (gitignored) |
| Cursor       | `.cursor/rules/*.mdc`                | MDC (full body)   | no (gitignored) |
| Codex        | `AGENTS.md` marked section           | pointer-only merge | yes       |

**Antigravity reads `.agents/skills/<name>/SKILL.md`, not bare root `skills/`** —
so README.md:178 (`Antigravity: skills/dva/SKILL.md`) is **wrong** and needs a human
fix (root README is AI-deny).

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
- **Phase 3 — Generator (DONE)** — Go tool `tools/skillgen`, wired into `make generate`.
  - Verified OpenCode + Antigravity conventions against official docs (both native Agent Skills).
  - Emits Cursor `.mdc` + Codex `AGENTS.md` section; ensures the 3 skill symlinks idempotently.
  - `make check-generate` diffs `AGENTS.md` + committed symlinks.

## Acceptance criteria

- [x] Canonical skills live at root | verify: `test -f skills/dva/SKILL.md -a -f skills/config/SKILL.md`
- [x] Claude Code plugin still resolves skills | verify: `test -f claude-plugin/skills/dva/SKILL.md`
- [x] `claude-plugin/skills` is a symlink, not a copy | verify: `test -L claude-plugin/skills`
- [x] Format spec exists | verify: `test -f skills/README.md`
- [x] Target manifest exists | verify: `test -f skills/_targets.yaml`
- [x] Cursor rules generated | verify: `test -f .cursor/rules/dva.mdc`
- [x] Antigravity skill resolves at documented path | verify: `test -f .agents/skills/dva/SKILL.md`
- [x] OpenCode skill resolves | verify: `test -f .opencode/skills/dva/SKILL.md`
- [x] Codex section present in AGENTS.md | verify: `grep -q 'skills:auto:start' AGENTS.md`
- [x] Generator compiles + module vets | verify: `go vet ./tools/skillgen/`
- [x] `make generate` is idempotent | verify: `make generate >/dev/null && git diff --exit-code AGENTS.md`
- [x] README Antigravity path corrected + OpenCode/Codex added | verify: `grep -q '.agents/skills/dva/SKILL.md' README.md`
