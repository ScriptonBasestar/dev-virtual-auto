---
id: TASK-009
title: "Fix plugin resolution for the runners/default_runner stack shape"
type: bug
priority: P1
status: done
archived-at: 2026-07-16T20:10:00+09:00
verified-at: 2026-07-16T20:10:00+09:00
verification-summary: >-
  Verified by orchestrator: make build / make test / go vet all exit 0 (config
  coverage 55.6%->57.2%); non-compose runner executes via all four orchestrator
  paths (up/status/stop/down); nested, flat and sole-runner shapes unchanged.
  Independent review found a CRITICAL silent-no-op regression for runners.docker
  (DetectPlugin named a plugin whose typed config was nil); corrected before
  commit by removing the DetectPlugin fallback, with guard tests added.
effort: M
created-at: 2026-07-16T09:19:12Z
source-run-id: 20260716T091912Z-73dc094
source-unified: tmp/gap-analysis-runs/20260716T091912Z-73dc094/unified.md
source-unified-sha256: e62018b67e3ac63021f034d888b8b6f64a2c008c299f3bfa82e5d2b2e94ef1b2
source-gap: G1
source-severity: HIGH
repo-snapshot: "dev-virtual-auto@73dc094 (master, clean)"
blocks: [TASK-010]
---

# Task 009: Fix Runners Plugin Resolution

## Summary

`stack` entries declared with the schema-blessed `default_runner` + `runners:` shape
resolve their plugin **only for `compose`**. The other 13 implemented plugins fail at
runtime with `unknown lifecycle plugin ""`, even though `dva validate` reports the
config valid.

## Evidence

Only compose is backfilled — `internal/config/lifecycle_helpers.go:29-31`:

```go
if entry.Plugin == "" && entry.ComposeConfig() != nil {
    entry.Plugin = "compose"
}
```

Resolution chain:

- `internal/config/lifecycle.go:604-607` — `ResolvePluginFromName` returns early when
  `len(e.Runners) > 0`, leaving `Plugin == ""`.
- `internal/config/lifecycle.go` — `DetectPlugin()` inspects only typed fields; it never
  consults `DefaultRunner` or `Runners`.
- `internal/lifecycle/orchestrator.go:98,172,214,257` — `NewPlugin(entry.DetectPlugin())`
  receives `""` and errors.
- `internal/config/lifecycle_helpers.go:5-21` — `ComposeConfig()` is the only reader of
  the `Runners` map.

Reproduction (identical entry, only the runner key changed):

| Runner | `dva validate` | `dva stack up` |
| ------ | -------------- | -------------- |
| `compose` | schema-checked | reached `docker compose` |
| `script` | ✅ valid | `ERROR: entry "web": unknown lifecycle plugin ""` |
| `process` / `kubectl` / `helm` | ✅ valid | same error |

Both alternative shapes work today (verified — marker file created), so this is HIGH,
not CRITICAL:

```yaml
stack: {web: {script: {up: "touch marker"}}}        # nested → works
stack: {web: {plugin: script, up: "touch marker"}}  # flat   → works
```

The plan path is already correct: `internal/lifecycle/resolver.go:153` reads
`stackEntry.DefaultRunner`, and `:165-173` falls back to the sole runner key when
exactly one is declared.

## Suggested Approach

Generalize the compose-only special case so the direct-stack path agrees with the plan
path rather than inventing new semantics: resolve `Plugin` from
`normalizeRunnerName(DefaultRunner)`, falling back to the single key when
`len(Runners) == 1` — mirroring `resolver.go:165-173`.

## Out Of Scope

- Changing the `runners` schema (TASK-010 covers validation).
- Removing or deprecating the nested/flat shapes; both must keep working.

## Completion Criteria

- [x] A `stack` entry using `default_runner: script` + `runners.script` executes via the script plugin instead of failing | verify: `cd "$(mktemp -d)" && printf 'version: "0.1.44"\nstack:\n  web:\n    default_runner: script\n    runners:\n      script:\n        up: "touch ./ran.txt"\n' > dva.yml && "$OLDPWD/bin/dva" stack up && test -f ./ran.txt`
- [x] A regression test covers runners-shape plugin resolution for at least one non-compose plugin | verify: `go test ./internal/config/ ./internal/lifecycle/ -run 'Runner|Plugin' -v`
- [x] The nested (`script:`) and flat (`plugin: script`) shapes still resolve | verify: `go test ./internal/config/ ./internal/lifecycle/`
- [x] Full suite and vet stay green | verify: `make test && go vet ./...`
- [x] Build succeeds | verify: `make build`

## Dependencies

None. Blocks TASK-010 (schema must match the working resolution contract).

## References

- `unified.md` (gap-analysis run `20260716T091912Z-73dc094`, untracked) — G1
- `code-to-doc.md` (gap-analysis run `20260716T091912Z-73dc094`, untracked) — C2
