---
id: TASK-020
title: "Hook wrapper bypasses the --help guard, executing hooks on --help"
type: bug
priority: P1
status: done
archived-at: 2026-07-16T21:40:00+09:00
verified-at: 2026-07-16T21:40:00+09:00
verification-summary: >-
  Verified by orchestrator: replace and before hooks no longer fire on --help; hooks still
  run for real invocations; --help prints usage. In this repo `dva build --help` now shows
  usage instead of running `make build`. make test and go vet green.
  The regression test was proven to catch the defect: reverting the one-line guard makes
  TestWrapWithHooksSkipsHooksOnHelp fail, restoring it makes it pass. The test is hermetic
  (t.Chdir + its own dva.yml + config-cache reset) - an earlier draft silently picked up the
  repository's real build hook and ran make build from a temp cwd.
effort: S
created-at: 2026-07-16T21:30:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: rediscovery-phase-1
source-severity: HIGH
---

# Task 020: Hook Wrapper Bypasses The --help Guard

## Summary

TASK-011 stopped `--help` from executing commands, but the fix is **incomplete**:
`wrapWithHooks` replaces `cmd.RunE` with a wrapper that runs `before`/`replace` hooks
**before** delegating to the original RunE — and TASK-011's `helpRequested` guard lives
*inside* that original. So for any hookable command with hooks defined, `--help` still
executes user-defined commands.

This is the same hazard class as G3, reached by a different path.

## Evidence

Reproduction — `dva.yml` with a `replace` hook on the reserved `build` command:

```yaml
interaction:
  build:
    replace:
      - step: "probe"
        run: "touch ./BUILD_RAN.txt"
```

```
$ dva build --help
[hook:replace:build] [1/1] probe
  $ touch ./BUILD_RAN.txt
>> BUILD_RAN.txt was created — the hook EXECUTED despite --help
```

This is not hypothetical: **this repository's own `dva.yml:19-23`** defines exactly such a
`replace` hook, so `dva build --help` runs `make build` here today.

Mechanism — `internal/cli/hooks.go:20-36`: `wrapWithHooks` swaps `cmd.RunE` for a wrapper
that calls `loadConfig()`, then `runHookSteps(..., "before", ...)` / `"replace"` before ever
reaching `original(cmd, args)`. `buildCmd`'s guard (`internal/cli/compose.go:344-345`) is
inside `original`, so it never runs.

Affected: the 7 hookable commands registered at `internal/cli/root.go:116-124` — `up`,
`down`, `stop`, `restart`, `build`, `clean`, `logs` — whenever the loaded `dva.yml` defines
hooks for them. Commands whose `DisableFlagParsing` is false (e.g. `clean`) are protected by
cobra intercepting `--help` before RunE; the `DisableFlagParsing: true` ones are not.

Control (already safe): `dva mycmd --help` and `dva run mycmd --help` print usage and do not
execute.

## Suggested Approach

Add the same guard at the top of the wrapper in `wrapWithHooks`, delegating to `original`
so the existing per-command guard renders help:

```go
if helpRequested(args) {
    return original(cmd, args)
}
```

Place it before `loadConfig()` so `--help` also works in a directory with no/invalid config.

## Out Of Scope

- Redesigning the hook system or hook phases.
- Changing which commands are hookable.

## Completion Criteria

- [x] `dva build --help` prints usage and does not run its replace hook | verify: `cd "$(mktemp -d)" && printf 'version: "0.1.44"\ninteraction:\n  build:\n    replace:\n      - step: p\n        run: "touch ./RAN.txt"\n' > dva.yml && "$OLDPWD/bin/dva" build --help && test ! -f ./RAN.txt`
- [x] A `before` hook does not fire on `--help` either | verify: `cd "$(mktemp -d)" && printf 'version: "0.1.44"\ninteraction:\n  up:\n    before:\n      - step: p\n        run: "touch ./RAN.txt"\n' > dva.yml && "$OLDPWD/bin/dva" up --help && test ! -f ./RAN.txt`
- [x] Hooks still run for real invocations (no regression) | verify: `cd "$(mktemp -d)" && printf 'version: "0.1.44"\ninteraction:\n  build:\n    replace:\n      - step: p\n        run: "touch ./RAN.txt"\n' > dva.yml && "$OLDPWD/bin/dva" build && test -f ./RAN.txt`
- [x] Suite and vet green | verify: `make test && go vet ./...`

## References

- [011-honor-help-flag.md](../_archive/011-honor-help-flag.md) — the incomplete fix
- [unified.md](../../tmp/gap-analysis-runs/20260716T091912Z-73dc094/unified.md) — G3, same hazard class
