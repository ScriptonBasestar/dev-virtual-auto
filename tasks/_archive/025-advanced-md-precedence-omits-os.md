---
id: TASK-025
title: "advanced.md precedence list omits OS environment variables"
type: docs
priority: P2
status: done
effort: XS
completed-at: 2026-07-16T23:45:00+09:00
verified-by: orchestrator (independent re-derivation, not agent report)
created-at: 2026-07-16T23:10:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: convergence-check-2
source-severity: MEDIUM
---

# Task 025: advanced.md Precedence List Omits OS

## Summary

`claude-plugin/skills/dva/references/advanced.md:65-71` lists the environment variable
precedence order as six ascending layers ending at "6. Explicit CLI flags" as the highest.
**OS environment variables are omitted entirely.** In reality OS wins over every layer,
including `--var`.

This is the sixth instance of the class fixed by TASK-012 and TASK-023. It survived both
because it lives in `claude-plugin/`, a root the original audit classified as input/config
and explicitly declared **unverified**. TASK-023's repo-wide greps targeted the inverted-chain
wording; this file does not state an inverted chain — it simply stops short — so no grep for
the wrong claim could match it.

## Evidence

`advanced.md:65-71` as shipped:

```
Environment variables are merged in precedence order:
1. Global `environment:` section
2. `env_file:` loaded values
3. Interaction command-level `environment:`
4. Mode `environment:` (if `--mode` set)
5. Environment preset (if `--env` set)
6. Explicit CLI flags
```

Reproduced by convergence check 2: `FOO=from_os dva up p1 --var FOO=from_cli` resolves to
`from_os`, i.e. OS beats `--var`, contradicting "6. Explicit CLI flags" being highest.

Corroborating code: `internal/config/environment.go` `MergeVars` applies OS values on top of
the resolved map. The already-corrected sources state this explicitly
(`USAGE.md:495`, `docs/30-config-merge-semantics.md:338`).

The ascending order of items 1 and 2 is **correct** and must be preserved:
`internal/cli/root.go:244-256` `loadEnv` seeds the environment from `c.Environment` and then
lets `LoadEnvFile` overwrite it, so `env_file` does outrank `environment:` (this is the
behavior TASK-018 established).

## Why this is safety-relevant, not cosmetic

A reader trusting this list believes `--var` is authoritative. It is not: a stray OS variable
silently overrides it with no warning. That is the same falsehood TASK-012 was raised for.

## Completion Criteria

- [x] `advanced.md`'s precedence list names OS environment variables as the highest layer | verify: `grep -A9 'precedence order' claude-plugin/skills/dva/references/advanced.md | grep -qi 'OS'` — PASS
- [x] No precedence list anywhere in the repo terminates at CLI flags as highest | verify: `grep -rn --include='*.md' -A9 'precedence order' . | grep -v '^\./tasks/' | grep -iE '[0-9]+\. *(Explicit )?CLI flags *$'` — PASS (no match; 12 live doc lines swept, >0 confirms the sweep is live)
- [x] Items 3-6 are verified against the binary or removed if they name layers that do not exist — **resolved by evidence, not by human judgment**: every named layer was proven to exist (table below)
- [x] `make test` passes | verify: `make test` — exit 0; `go vet ./...` exit 0

## Verification Record (orchestrator, independent of the agent's report)

The implementing agent went beyond the spec: rather than appending OS to one list, it split the
list into three command paths. That is a larger claim surface, so every claim was re-derived
from the binary before acceptance. Each probe ran with a positive control to avoid vacuous
passes.

| Claim in the new text | Probe | Result |
| --- | --- | --- |
| `env_file` > `environment:` | same key in both, `dva run` | `from-envfile` wins ✅ |
| interaction `environment:` > `env_file` | same key in both | `from-interaction` wins; control without it yields `from-envfile` ✅ |
| OS > every layer incl. `--var` | `FOO=from-os dva up p1 --var FOO=from-cli` | `from-os` ✅ |
| `-E` < `-M` | both set same key | `from-mode` wins, **independent of arg order** ✅ |
| `dva run` has no `-M`/`-E`/`--var` | `dva run showvar --var BAR=cli` | `ERROR: unknown flag: --var` ✅ |
| `dva up` / `dva stack up` accept `-M`/`-E` | `dva stack up s1 -E dev` | `ERROR: env 'dev' not found` — i.e. the flag **parsed and acted** ✅ |

Two traps caught during verification, both worth recording:

1. **`dva stack up --help` lists no `-M`/`-E`, but both work.** An initial grep of `--help`
   suggested the agent's grouping was wrong. It was not — the help text is incomplete. `--help`
   is not the source of truth for this binary.
2. **A criterion of this very task was itself vacuous.** The original C2 binding used an
   unquoted `--include=*.md`, which zsh tried to glob: the command died with
   `no matches found` and the `!` negation turned that failure into a green PASS. The binding
   above is the quoted, control-checked replacement. The same class of error this run has
   flagged to five agents was reproduced by the orchestrator writing the check.

## Follow-on found while verifying (not fixed here)

`--var` is accepted and works on `dva up` (proven: `from-global` -> `from-cli`), but it is
**absent from `dva up --help`'s "DVA-specific flags" list**. A user cannot discover it from the
binary. Recorded as TASK-027 rather than fixed here, to keep one task per commit.

## References

- [023-repo-wide-precedence-sweep.md](../_archive/023-repo-wide-precedence-sweep.md) — the sweep that missed this root
- [012-fix-env-precedence-claim.md](../_archive/012-fix-env-precedence-claim.md) — the original class
- [018-fix-env-file-precedence.md](../_archive/018-fix-env-file-precedence.md) — why `env_file` > `environment:`
