---
id: TASK-018
title: "Fix inverted env_file/environment precedence in CLAUDE.md"
type: docs
priority: P1
status: done
archived-at: 2026-07-16T20:50:00+09:00
verified-at: 2026-07-16T20:50:00+09:00
verification-summary: >-
  Verified by orchestrator: CLAUDE.md:66 now reads `environment:` < `env_file` < OS,
  matching root.go:249-252. Runtime-proven both halves: env_file value wins over
  top-level environment: on the dva run path, and an OS value still beats both.
  Added a pointer to USAGE.md for the plan path's full chain, since the two paths
  describe different layer sets.
effort: XS
created-at: 2026-07-16T20:45:00+09:00
source-run-id: 20260716T091912Z-73dc094
discovered-in: TASK-012
source-severity: HIGH
---

# Task 018: Fix CLAUDE.md env_file/environment Precedence

## Summary

`CLAUDE.md:66` states the env precedence as `env_file` < `environment:` < OS. The OS
half is correct, but the **first pair is inverted**: `env_file` actually **overrides**
top-level `environment:`. This was missed by the original audit, which proved only the
OS half and then marked the whole clause Verified.

## Evidence

`internal/cli/root.go:249-252` — `environment:` is loaded first, then `env_file`
overwrites it:

```go
env = config.NewEnvironment(c.Environment, wd, c.FileDir())
if c.EnvFile != nil {
    if err := config.LoadEnvFile(c.EnvFile, c.FileDir(), env); err != nil {
```

Runtime proof — `.env` sets `P_TOPENV=from-env-file`, `environment:` sets
`P_TOPENV=from-top-environment`:

```
$ dva run showvar
RESULT P_TOPENV=[from-env-file]      # env_file wins, contradicting CLAUDE.md:66

$ P_TOPENV=from-os dva run showvar
RESULT P_TOPENV=[from-os]            # OS still wins (that half of CLAUDE.md:66 is right)
```

TASK-012 corrected `USAGE.md`, `docs/30` and `schema.json` to the proven chain
(`env_file < global vars < environment vars < site vars < plan vars < CLI vars < OS`),
but `CLAUDE.md` was outside its edit scope, so the root operational policy still
carries the inverted pair.

Note the two chains describe different paths: TASK-012's chain covers the plan path,
where `environment vars` means `environments.<name>.environment`. `CLAUDE.md:66`'s
middle term is the **top-level** `environment:` block on the `dva run` path. Both
statements must be true for their own path; only the `CLAUDE.md` one is currently wrong.

## Out Of Scope

- Changing any Go behavior. The code is authoritative; only the doc is wrong.
- Top-level `vars:` being inert on the `dva run` path — see TASK-019.

## Completion Criteria

- [x] `CLAUDE.md` states that `env_file` overrides top-level `environment:`, with OS highest | verify: `! /usr/bin/grep -n 'env_file` < `environment:' CLAUDE.md`
- [x] The corrected claim matches observed behavior | verify: `cd "$(mktemp -d)" && printf 'P_T=from-env-file\n' > .env && printf 'version: "0.1.44"\nenv_file:\n  - .env\nenvironment:\n  P_T: from-top-environment\ninteraction:\n  s:\n    description: p\n    script: '"'"'echo "R=[$P_T]"'"'"'\n' > dva.yml && "$OLDPWD/bin/dva" run s 2>&1 | /usr/bin/grep -q 'R=\[from-env-file\]'`

## References

- [012-fix-env-precedence-docs.md](../_archive/012-fix-env-precedence-docs.md) — discovered here
- `unified.md` (gap-analysis run `20260716T091912Z-73dc094`, untracked) — G4 (same defect class)
