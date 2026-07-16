---
id: TASK-022
title: "vars precedence chain never places the root environment: map"
type: docs
priority: P3
status: todo
effort: XS
created-at: 2026-07-16T21:45:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: rediscovery-phase-1
source-severity: LOW
---

# Task 022: vars Chain Omits The Root environment: Map

## Summary

The corrected vars precedence chain (TASK-012) never places the **root** `environment:`
map, and its "environment vars" term invites the reader to assume it means exactly that —
which would give them the wrong order relative to `env_file`.

## Evidence

The chain now published at `USAGE.md`, `docs/30-config-merge-semantics.md` and
`internal/config/schema.json`:

```
env_file < global vars < environment vars < site vars < plan vars < CLI vars < OS
```

Here "environment vars" means `environments.<name>.environment` (the plan path). The
**root** `environment:` block is a different thing on a different path, and its order is
the inverse of what the chain implies:

- Root `environment:` is applied by `loadEnv` (`internal/cli/root.go:249-252`) **before**
  `env_file` overwrites it → `environment:` < `env_file`, as TASK-018 corrected in
  `CLAUDE.md`.
- The chain lists `env_file` **lowest**, so a reader mapping "environment vars" onto the
  root `environment:` block would conclude `env_file < environment:` — exactly backwards.

Both statements are individually correct for their own path; the hazard is the collision of
terms. This is a docs-clarity gap, not a behavioral defect — no code change is implied.

## Out Of Scope

- Any Go behavior change. Both paths behave correctly.
- Renaming config keys.

## Completion Criteria

- [ ] The chain states which path it describes, and that "environment vars" means `environments.<name>.environment`, not the root `environment:` block | verify: `grep -n "environments.<name>.environment\|environments\.<name>" USAGE.md docs/30-config-merge-semantics.md`
- [ ] The root `environment:` vs `env_file` order is stated or cross-referenced where the chain appears | verify: `grep -rn "env_file" USAGE.md docs/30-config-merge-semantics.md | head`
- [ ] No claim contradicts CLAUDE.md's `environment:` < `env_file` < OS | verify: `grep -n "environment:` < `env_file" CLAUDE.md`

## References

- [012-fix-env-precedence-docs.md](../_archive/012-fix-env-precedence-docs.md) — established the chain
- [018-fix-claude-md-env-precedence.md](../_archive/018-fix-claude-md-env-precedence.md) — the root-path pair
