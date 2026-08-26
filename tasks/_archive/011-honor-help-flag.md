---
id: TASK-011
title: "Honor --help on commands that disable flag parsing"
type: bug
priority: P1
status: done
archived-at: 2026-07-16T20:35:00+09:00
verified-at: 2026-07-16T20:35:00+09:00
verification-summary: >-
  Verified by orchestrator: make build / make test / go vet exit 0; all 16 documented
  commands print usage for --help (0/16 failures); 'up --help', 'stack down --help' and
  'stack stop --help' no longer execute (marker files absent) while real 'stack up' still
  executes. 17 DisableFlagParsing sites patched (3 more than the 16 user-facing
  invocations: compose build/logs and app build share the same defect).
  CORRECTION: this task's secondary claim - that 'dva down --help' printed an error yet
  exited 0 - was a MEASUREMENT ERROR by the auditor. The exit code was read after a pipe
  ($? captured head's status, not dva's). root.go:185-204 os.Exit(1)s on any returned
  error, so that path did exit non-zero. No exit-code change was needed or made; the
  help guard returns before teardownCommon's stray-positional path. The immutable run
  archive retains the original incorrect claim by design.
effort: M
created-at: 2026-07-16T09:19:12Z
source-run-id: 20260716T091912Z-73dc094
source-unified: tmp/gap-analysis-runs/20260716T091912Z-73dc094/unified.md
source-unified-sha256: e62018b67e3ac63021f034d888b8b6f64a2c008c299f3bfa82e5d2b2e94ef1b2
source-gap: G3
source-severity: HIGH
repo-snapshot: "dev-virtual-auto@73dc094 (master, clean)"
---

# Task 011: Honor --help On Passthrough Commands

## Summary

16 commands set `DisableFlagParsing: true`, which stops cobra from intercepting
`--help`. The flag is passed through to `Run` as a positional argument and the command
**executes instead of printing usage**. `dva up --help` starts services;
`dva stack down --help` removes them.

## Evidence

Reproduction in a directory with a valid `dva.yml`:

```
$ ./bin/dva up --help
[warn] no lifecycle entries matched filters
Lifecycle: (no entries configured)
EXIT=0
```

Lifecycle execution output, not usage. Control — `internal/cli/run.go:21` sets
`DisableFlagParsing: false` and behaves correctly:

```
$ ./bin/dva run --help
Execute a predefined script from 'dva.yml' (prefix 'run' can be omitted)
Usage: ...
```

Probe in a clean directory (no `dva.yml`): every affected command answered `--help`
with `ERROR: could not find dva.yml`, proving it entered the execution path rather
than the help path.

Affected (`DisableFlagParsing: true`): `internal/cli/compose.go:27,74,218,251,284,327,426`;
`internal/cli/app.go:91,162,198`; `internal/cli/stack.go:36,105,140,220`;
`internal/cli/infra.go:22,49`; `internal/cli/kubectl.go:19`.

| `--help` honored | Commands |
| ---------------- | -------- |
| **No (16)** | `up`, `down`, `stop`, `restart`, `stack up`, `stack stop`, `stack down`, `app up`, `app restart`, `compose up`, `compose down`, `compose stop`, `compose restart`, `infra up`, `infra down`, `ktl` |
| Yes (control) | `run`, `validate`, `status`, `ls` |

`USAGE.md` documents these commands and their DVA-specific flags, so a reader is
actively invited to run `--help` on them.

Secondary (LOW, same area): `dva down --help` prints
`ERROR: 'dva down' downs all services...` yet exits **0** — an error message on a
success exit code.

## Suggested Approach

`DisableFlagParsing` exists so DVA-specific flags can be forwarded to
`docker compose` / `kubectl`. Keep passthrough, but intercept `--help`/`-h` before
execution (e.g. check `args` at the top of `Run` and call `cmd.Help()`), so the
passthrough contract is preserved while help stays safe.

## Out Of Scope

- Redesigning flag passthrough.
- Documenting flags (not covered by this task; flags remain unaudited).

## Completion Criteria

- [x] `dva up --help` prints usage and does not execute the lifecycle | verify: `cd "$(mktemp -d)" && printf 'version: "0.1.44"\nstack:\n  web:\n    script:\n      up: "touch ./started.txt"\n' > dva.yml && "$OLDPWD/bin/dva" up --help && test ! -f ./started.txt`
- [x] All 16 affected commands print their own usage for `--help` | verify: `for c in "up" "down" "stop" "restart" "stack up" "stack stop" "stack down" "app up" "app restart" "compose up" "compose down" "compose stop" "compose restart" "infra up" "infra down" "ktl"; do ./bin/dva ${=c} --help 2>&1 | /usr/bin/grep -q "Usage:" || { echo "FAIL: $c"; exit 1; }; done; echo OK`
- [x] Flag passthrough to compose/kubectl still works | verify: `go test ./internal/cli/ -v`
- [x] Full suite and vet stay green | verify: `make test && go vet ./...`

## Dependencies

None.

## References

- `unified.md` (gap-analysis run `20260716T091912Z-73dc094`, untracked) — G3
- `code-to-doc.md` (gap-analysis run `20260716T091912Z-73dc094`, untracked) — C1
