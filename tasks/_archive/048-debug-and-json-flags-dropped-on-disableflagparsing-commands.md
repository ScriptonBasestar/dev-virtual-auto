---
id: TASK-048
title: "--debug and --json are silently dropped by every DisableFlagParsing command"
type: bug
priority: P2
status: done
effort: S
created-at: 2026-07-16T23:24:00+09:00
completed-at: 2026-07-17T01:48:47Z
archived-at: 2026-07-17T10:51:00+09:00
verified-at: 2026-07-17T10:51:00+09:00
completion-summary: |
  Pre-parse --debug/--json from os.Args in PersistentPreRun before logger.Init
  (applyRootPersistentFlagsFromArgs). Also strip them in parseDvaFlags so lifecycle
  commands do not treat them as entry names. --dry-run left alone for compose passthrough.
verification-status: verified
verification-evidence:
  - "RED: go test failed with undefined: applyRootPersistentFlagsFromArgs before impl"
  - "GREEN: go test ./internal/cli/ -run TestApplyRootPersistentFlagsFromArgs|TestParseDvaFlagsConsumesDebugAndJSON|TestParseDvaFlags -count=1 EXIT=0"
  - "GREEN: go test ./internal/cli/ -count=1 EXIT=0"
  - "GREEN: go vet ./internal/cli/... clean"
  - "TASK-047 regression: TestParseDvaFlags* still pass"
verification-summary: |
  Deliverables verified:
  - internal/cli/root.go has applyRootPersistentFlagsFromArgs (defined at :251) and
    PersistentPreRun calls it (line 43) before logger.Init (line 44).
  - parseDvaFlags (compose.go:542) consumes --debug/--json (sets globals, excludes from filtered).
  - internal/cli/root_persistent_flags_test.go exists (covers TestApplyRootPersistentFlagsFromArgs*
    and TestParseDvaFlagsConsumesDebugAndJSON).
  - go test ./internal/cli/ -run TestApplyRoot -count=1 passes (ok).
  File moved from done/ to _archive/ following sibling TASK-047 archive convention (status: done).
source-run-id: 20260716T112622Z-5729d98
discovered-in: TASK-047's audit criterion, measured by the orchestrator at a9dd492
source-severity: MEDIUM
---

# Task 048: `--debug` And `--json` Are Accepted And Ignored On Lifecycle Commands

## Summary

The same root cause as TASK-047 (fixed in `a9dd492`), on the two remaining root persistent flags.
18 commands set `DisableFlagParsing: true`, so cobra never parses `--debug` (`root.go:53`) or
`--json` (`root.go:55`) for them. Both are accepted without error and have no effect.

This is the **harmless direction** — the user asks for more output and gets less — which is why it
is P2 and not P1. Nothing is destroyed and nothing lies about having happened. But `--json` is
documented as "LLM-optimized" output, so a pipeline invoking `dva up --json` and parsing the result
gets human text with no signal that the flag was ignored.

## Evidence — measured at a9dd492, with a positive control

```
$ dva up --debug        # DisableFlagParsing command
level=DEBUG lines: 0

$ dva validate --debug  # control: cobra parses flags normally here
level=DEBUG lines: 1
```

Same binary, same flag, same config. The control proves `--debug` genuinely works when cobra parses
it, so the 0 above is the flag being dropped — not an absence of debug-level statements.

```
$ dva up --json
[lifecycle] sideeffect (script)
  $ touch UP_HAPPENED
```

Human-formatted output; `--json` accepted and ignored.

## Why TASK-047's fix does not cover these

TASK-047 was fixed by consuming `--dry-run` in `parseDvaFlags` (`compose.go:490`). **That seam cannot
work for these two flags**, and the reason is a cobra lifecycle detail worth recording:

- `parseDvaFlags` is called from inside each command's `RunE`.
- `dryRun` is read *late* — `DryRun: dryRun` is evaluated during `RunE`, **after** `parseDvaFlags`
  has run. So setting it there lands in time.
- `debug` and `jsonOutput` are read *early* — `logger.Init(debug, jsonOutput)` at **`root.go:40`,
  inside `PersistentPreRun`**, which cobra invokes **before** `RunE`. Setting them in
  `parseDvaFlags` would happen after the logger is already constructed, and would silently do
  nothing.

So this needs a fix at a different point in the lifecycle: scan `os.Args` for the root persistent
flags inside (or before) `PersistentPreRun`, ahead of `logger.Init`.

## Fix

Pre-parse the root persistent flags centrally, before `logger.Init` at `root.go:40`. Doing it
centrally — rather than per-flag — is what stops the next persistent flag added to `root.go` from
regressing the same way for the third time. `--dry-run` may then be removed from `parseDvaFlags`
and handled in the same place, provided TASK-047's regression tests stay green.

Watch out for `composeCmd` (`compose.go:17`), the raw docker passthrough: it deliberately forwards
unparsed args to `docker compose`, which has its own `--dry-run`. It does not route through
`parseDvaFlags` today, and a central pre-parse **must not** start swallowing flags meant for docker.
That is precisely why TASK-047 chose the narrower seam.

## Completion Criteria

- [ ] `dva up --debug` emits debug-level logs | verify: `human — assert 'dva up --debug' produces level=DEBUG lines, matching the 'dva validate --debug' control`
- [ ] `dva up --json` emits JSON, or is rejected as unsupported on that command | verify: `human — assert output parses as JSON, OR that the flag errors loudly. Silently ignoring it is the bug`
- [ ] A regression test proves the flags reach their consumers, failing without the fix | verify: `human — revert the pre-parse, confirm the test FAILS for the right reason, restore, confirm it passes`
- [ ] TASK-047 does not regress | verify: `go test ./internal/cli/ -run TestParseDvaFlags`
- [ ] `docker compose --dry-run` passthrough still works via `dva compose` | verify: `human — assert composeCmd still forwards --dry-run to docker rather than consuming it`
- [ ] `make test` and `go vet ./...` pass | verify: `make test && go vet ./...`

## References

- [047-dry-run-ignored-on-disableflagparsing-commands.md](../_archive/047-dry-run-ignored-on-disableflagparsing-commands.md) — same root cause, fixed at the `RunE` seam; explains why that seam cannot work here
- `internal/cli/root.go:39-55` — `PersistentPreRun` / `logger.Init` and the three persistent flag declarations
