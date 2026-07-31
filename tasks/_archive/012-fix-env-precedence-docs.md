---
id: TASK-012
title: "Correct inverted ENV precedence in USAGE.md, docs/30 and schema.json"
type: docs
priority: P1
status: done
archived-at: 2026-07-16T20:45:00+09:00
verified-at: 2026-07-16T20:45:00+09:00
verification-summary: >-
  Verified by orchestrator: full chain proven empirically, not merely reversed -
  env_file < global vars < environment vars < site vars < plan vars < CLI vars < OS env.
  Only OS's position was wrong; the middle layers' documented order was already correct.
  Runtime-proven: --var beats plan vars, and OS beats --var (K_PC=from-os won over
  --var K_PC=from-cli-vars). --var is real (plan_lifecycle.go:60-70), not aspirational.
  All three sources corrected; old inverted chain absent; schema parses; tests green.
  Two out-of-scope findings raised as TASK-018 (CLAUDE.md:66 env_file/environment pair
  inverted; top-level vars: inert on the dva run path).
effort: S
created-at: 2026-07-16T09:19:12Z
source-run-id: 20260716T091912Z-73dc094
source-unified: tmp/gap-analysis-runs/20260716T091912Z-73dc094/unified.md
source-unified-sha256: e62018b67e3ac63021f034d888b8b6f64a2c008c299f3bfa82e5d2b2e94ef1b2
source-gap: G4
source-severity: HIGH
repo-snapshot: "dev-virtual-auto@73dc094 (master, clean)"
---

# Task 012: Fix Inverted ENV Precedence Documentation

## Summary

Three current sources document the OS environment variable as the **lowest** priority
layer. The implementation puts it **highest**. This is not a missing sentence — it is
the inverse of the truth on the most safety-relevant precedence rule, and one of the
three sources is the JSON schema that ships to users' editors.

**Decision (from unified report): correct the docs to match the code. Do not change code.**

## Evidence

| Source | Claim |
| ------ | ----- |
| `CLAUDE.md:66` | `env_file` < `environment:` < **OS 환경 변수** (OS highest) — **correct** |
| `USAGE.md:376` | `OS` < env_file < global vars < environment vars < site vars < plan vars < CLI vars (OS lowest) — **wrong** |
| `docs/30-config-merge-semantics.md:327-330` | same as USAGE (OS lowest) — **wrong** |
| `internal/config/schema.json:369` | `"description": "... Priority: OS < env_file < ..."` (OS lowest) — **wrong, ships to users** |

Code truth — `internal/config/environment.go:58-66`:

```go
func (e *Environment) MergeVars(vars map[string]string) {
    for k, v := range vars {
        if envVal, ok := os.LookupEnv(k); ok {
            e.Vars[k] = envVal          // OS overrides config
        } else {
            e.Vars[k] = e.Interpolate(v)
        }
    }
}
```

Runtime proof:

```
$ ./bin/dva run showvar                       → PROBE_VAR=            (config value)
$ PROBE_VAR=from-os-env ./bin/dva run showvar → PROBE_VAR=from-os-env
```

Supporting comment: `internal/config/envfile.go:48` — "env_file < OS env".

Risk: a user reading any of the three wrong sources could believe a config value
overrides a production OS variable. It does not.

## Out Of Scope

- Changing `MergeVars` or any precedence behavior — the code is authoritative here.
- `internal/config/environment.go:87` — the comment "Config vars override OS
  environment variables" reads ambiguously in isolation but describes no behavioral
  conflict (`MergeVars` resolves OS precedence before `EnvSlice` emits at
  `environment.go:88-99`). Optional clarification only; not required by this task.

## Completion Criteria

- [x] `USAGE.md:376`, `docs/30-config-merge-semantics.md:327-330`, and `internal/config/schema.json:369` state OS environment as the highest-priority layer, consistent with `CLAUDE.md:66` | verify: `! grep -rnE 'OS *<' USAGE.md docs/30-config-merge-semantics.md internal/config/schema.json`
- [x] No current doc still places OS at the bottom of the chain | verify: `grep -rn "환경 변수 우선순위\|Priority:" USAGE.md docs/30-config-merge-semantics.md internal/config/schema.json CLAUDE.md`
- [x] `schema.json` remains valid JSON and all examples still validate | verify: `python3 -c "import json;json.load(open('internal/config/schema.json'));print('schema.json parses')" && go test ./internal/config/ -run 'Schema|Example'`
- [x] Full suite stays green | verify: `make test`

## Dependencies

None. Doc-only, zero runtime risk.

## References

- `unified.md` (gap-analysis run `20260716T091912Z-73dc094`, untracked) — G4
- `doc-to-code.md` (gap-analysis run `20260716T091912Z-73dc094`, untracked) — L1
- `evidence-contradictions.md` (gap-analysis run `20260716T091912Z-73dc094`, untracked)
