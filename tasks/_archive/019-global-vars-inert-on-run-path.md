---
id: TASK-019
title: "Top-level vars: is inert on the dva run path, contradicting its schema description"
type: bug
priority: P2
status: done
effort: S
created-at: 2026-07-16T20:45:00+09:00
source-run-id: 20260716T091912Z-73dc094
discovered-in: TASK-012
source-severity: MEDIUM
needs-human: false
decision-status: accepted
decision: "Option A — inject cfg.Vars in loadEnv"
decision-at: 2026-07-17T11:10:00+09:00
moved-at: 2026-07-17T11:15:00+09:00
verified-at: 2026-07-17T11:15:00+09:00
verification-summary: >-
  Option A implemented: loadEnv seeds NewEnvironment with c.Vars, then MergeVars(c.Environment),
  then LoadEnvFile. Run-path order is vars < environment: < env_file < OS.
  TestLoadEnv_IncludesGlobalVars proves P_GLOBAL from vars reaches the env, and env_file
  wins over environment over vars for shared keys. go test ./internal/cli/ -count=1 ok.
---

# Task 019: Global vars Inert On The run Path

## Decision

**Option A accepted** — `loadEnv` merges `cfg.Vars` beneath `environment:`/`env_file`.

Run-path precedence: `vars < environment: < env_file < OS env`.

## Summary

`schema.json` describes `vars` as "Global variables injected into **all execution
contexts**". They are not: `dva run <interaction>` receives nothing, while the plan
path receives them. Either the code or the description is wrong, and choosing which
is a design call.

## Evidence

Same `dva.yml` declaring `vars: {P_GLOBAL: from-global-vars}`, one interaction and one
plan:

```
$ dva run showvar        # interaction path
RESULT P_GLOBAL=[]                    # <-- inert

$ dva up p1              # plan path
STACK P_GLOBAL=[from-global-vars]     # <-- injected
```

Mechanism: `loadEnv` (`internal/cli/root.go:245-255`) builds the environment from
`c.Environment` and `env_file` only — it never reads `cfg.Vars`. Only
`lifecycle.ResolvePlan` folds `cfg.Vars` into the resolved plan vars. So "global vars"
exists solely on the plan path.

Schema text: `internal/config/schema.json` → `properties.vars.description` —
"Global variables injected into all execution contexts."

## Why this needs a decision

Two defensible fixes, with different blast radius:

- **Option A (code fix)** — make `loadEnv` merge `cfg.Vars` beneath `environment:`/`env_file`,
  so `vars:` behaves as documented everywhere. Matches the stated contract and the word
  "global". Risk: variables that previously never reached interaction scripts suddenly do,
  which can change existing users' runtime behavior.
- **Option B (doc fix)** — narrow the description to say `vars` applies to the plan path
  (`dva up <plan>`), not to `dva run` interactions. Zero runtime risk, but leaves `vars:`
  a misleading name and the two paths inconsistent.

Recommendation: **Option A**, because `schema.json` states the contract users rely on and
`dva run` is plainly an execution context — but it is a behavior change, so a product owner
should confirm before it ships.

## Out Of Scope

- The precedence chain itself (TASK-012, TASK-018).

## Completion Criteria

- [x] Option A or B is chosen and recorded here | verify: decision field = Option A
- [x] Implementation matches the chosen option, with `vars` behavior and its schema description agreeing | verify: `go test ./internal/cli/ -count=1` ok; TestLoadEnv_IncludesGlobalVars green

## Implementation

- `internal/cli/root.go` `loadEnv`: `NewEnvironment(c.Vars, ...)` then `MergeVars(c.Environment)` then `LoadEnvFile`
- `internal/cli/root_test.go` `TestLoadEnv_IncludesGlobalVars`: locks vars injection + precedence

## References

- [012-fix-env-precedence-docs.md](../_archive/012-fix-env-precedence-docs.md) — discovered here
