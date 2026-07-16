---
id: TASK-047
title: "dva up --dry-run executes for real; --dry-run is dropped by every DisableFlagParsing command"
type: bug
priority: P1
status: done
effort: M
created-at: 2026-07-17T08:40:00+09:00
archived-at: 2026-07-16T23:20:00+09:00
verified-at: 2026-07-16T23:20:00+09:00
verification-summary: |
  Fixed in internal/cli/compose.go by consuming --dry-run in parseDvaFlags, the shared
  seam all 12 DisableFlagParsing lifecycle callers route through. composeCmd (raw docker
  passthrough) does not call it, so docker's own --dry-run is unaffected.
  Regression test internal/cli/dryrun_flag_test.go proven RED before the fix (reverted the
  parse: "parseDvaFlags did not set dryRun" + "filtered = [--dry-run postgres]") while its
  negative control still PASSED, then GREEN after restore.
  E2E re-measured on a script-side-effect probe (liveness gate: dva validate EXIT=0):
  up --dry-run no longer executes and now emits the same msg=dry-run log as the plan route;
  positive control (plain `dva up`) still executes; plan route not regressed; stack up
  --dry-run and stack down --dry-run now print real plans instead of "no lifecycle entries
  matched filters"; `dva down --dry-run` previews, so its own recommended command works.
  make test EXIT=0 (0 FAIL), go vet clean.
  Audit outcome: --debug and --json ARE dropped identically (measured: `dva up --debug`
  produced 0 level=DEBUG lines vs 1 for the root-parsed `dva validate --debug`), but they
  are consumed by logger.Init in PersistentPreRun (root.go:40) which runs BEFORE RunE, so
  they cannot be fixed at this seam. Filed separately as TASK-048.
source-run-id: 20260716T112622Z-5729d98
discovered-in: sweep-checks report, independently re-verified by the orchestrator at 413f9fc
source-severity: HIGH
---

# Task 047: `dva up --dry-run` Prints A Plan And Performs It

## Summary

`dva up --help` advertises:

```
Global Flags:
      --dry-run   Show execution plan without running
```

`dva up --dry-run` **runs the stack for real**. The flag is accepted, no error is printed, and the
output looks like a preview.

This is the **harmful direction** and the sharpest instance found in this entire audit. Every other
finding in the queue is DVA doing *less* than promised. This one does *more*: the user explicitly
asks for "show me what would happen without doing it", and DVA does it.

## Root cause

`upCmd` (`internal/cli/compose.go:62`) sets `DisableFlagParsing: true` (`:83`), so cobra never parses
the root persistent `--dry-run` (`internal/cli/root.go:54`). Its hand-rolled loop
(`internal/cli/compose.go:100-111`) handles exactly four flags:

```go
for _, a := range args {
    switch a {
    case "--force":   force = true
    case "--no-wait": noWait = true
    case "--dev":     devMode = true
    case "--docker":  docker = true
    }
}
```

`--dry-run` is not among them. `--dry-run` therefore falls through, and `DryRun: dryRun`
(`compose.go:139`) reads the package global, which cobra left at `false`.

**18 commands set `DisableFlagParsing: true`** (`internal/cli/`: compose, down, stop, restart, build,
logs, app up/restart/build, ktl, infra up/stop/down/log, ...). Exactly **one** place in the tree
manually parses the flag — `internal/cli/plan_lifecycle.go:100` (`case a == "--dry-run":`), the plan
route. Every other `DryRun: dryRun` reader under a `DisableFlagParsing` command is reading a global
that can never be set on that path.

## Evidence — measured at `413f9fc` (liveness gate: `dva validate` EXIT=0)

```yaml
version: "0.1.0"
stack:
  sideeffect:
    script:
      up: "touch UP_HAPPENED"
      down: "touch DOWN_HAPPENED"
plans:
  p1:
    entries:
      - name: sideeffect
```

### Positive control — the probe is wired

```
$ dva up                       # no flag
UP_EXIT=0  -> UP_HAPPENED created    # the script runner really runs
```

### The finding

```
$ rm -f UP_HAPPENED && dva up --dry-run
[lifecycle] sideeffect (script)
  $ touch UP_HAPPENED
UP_DRYRUN_EXIT=0
-> UP_HAPPENED EXISTS            # <-- executed
```

Note the output: `$ touch UP_HAPPENED` **reads like a dry-run preview**. The command is echoed as if
it were being shown rather than run. A user watching this scroll by has no signal that anything
happened.

### The in-band control — the same command honors the flag on its other route

This is what makes the finding decisive rather than a debate about intent. `plans:` is present, so
the same binary, same config, same flag:

```
$ dva up p1 --dry-run          # PLAN route -> plan_lifecycle.go:100 parses --dry-run
[plan: p1] environment= site= entries=1
[lifecycle] sideeffect (script)
level=INFO msg=dry-run entry=sideeffect plugin=script script="touch UP_HAPPENED"
-> UP_HAPPENED absent            # HONORED

$ dva up --dry-run             # BARE route -> compose.go:100-111, no --dry-run case
-> UP_HAPPENED created           # IGNORED
```

The plan route proves `--dry-run` is a real, implemented, working capability of the lifecycle layer.
The bare route drops it before it ever reaches that layer. This cannot be "dry-run isn't supported
here" — it is supported, on the sibling route, three lines away in the same file.

## Scope — measured, not assumed. Four different outcomes, one root cause

| Command | `--dry-run` | Why |
|---|---|---|
| `dva up` | **EXECUTES** | flag silently dropped; `DryRun: dryRun` reads an unset global |
| `dva stack up` | never previews | flag consumed as a bogus **entry-name filter** → `[warn] no lifecycle entries matched filters`, exit 0 |
| `dva stack down` | never previews | same; exit 0, prints nothing |
| `dva down` | errors, safe | rejects the positional: `ERROR: 'dva down' downs all services. Use 'dva stack down --dry-run' ...` |
| `dva up <plan>` | **honored** | `plan_lifecycle.go:100` parses it manually |

Two distinct defects share the root cause, and the difference matters for triage:

1. **`dva up --dry-run` executes.** Harmful. This is the P1.
2. **`dva stack up/down --dry-run` silently previews nothing.** Harmless direction, but the flag is
   swallowed as an entry name, so the user gets a confusing `no lifecycle entries matched filters`
   instead of a plan. `stack up` fails safe **by accident**, not by design — it can never preview.

And a third-order consequence worth its own line: **`dva down --dry-run`'s error message recommends
`dva stack down --dry-run`** — a command that, per the table above, silently does nothing and previews
nothing. The tool's own remediation advice points at a broken path.

## Why P1 and not P0

Not P0: the destructive paths happen to be the safe ones. `dva down --dry-run` errors out, and
`dva stack down --dry-run` no-ops. So "user previews a teardown and destroys their stack" — the
worst version of this bug — **does not reproduce**; I probed it specifically and it did not occur.

Not P2: `dva up` is the single most-run command in the tool, `--dry-run` is advertised in its own
`--help`, and `up` on a real config is not inert — it starts containers, runs `script.up` bodies, and
executes provision hooks. `stack.<entry>.script.up` is arbitrary shell. A user running
`dva up --dry-run` against an unfamiliar or freshly-cloned `dva.yml` to see what it *would* do gets
every one of those side effects, with output that looks like a plan.

## Fix

Mechanical, and no product decision is required — the flag's contract is already fixed by its own
help text and already implemented on the plan route:

- Handle `--dry-run` in the manual loop at `compose.go:100-111` alongside `--force`/`--no-wait`, the
  same way `plan_lifecycle.go:100` already does.
- Better: strip/parse root persistent flags **centrally** for all 18 `DisableFlagParsing` commands
  rather than per-command, so the next added flag does not silently regress the same way. This is the
  real fix; the per-command patch is the minimum.
- For `stack up`/`stack down`, `--dry-run` must also stop being collected as an entry name.
- Audit the other `DisableFlagParsing` commands for the same class of drop (`--debug` and `--json`
  are the other two root persistent flags and are likely affected identically — **not verified**).

Whichever shape the fix takes, the plan route (`plan_lifecycle.go:100`) is the reference behavior and
must not regress.

## Completion Criteria

- [x] `dva up --dry-run` does not execute stack entries | verify: `human — reproduce the probe: stack entry with script.up "touch UP_HAPPENED"; run 'dva up --dry-run'; assert UP_HAPPENED is ABSENT and a plan is printed`
- [x] The positive control still passes — plain `dva up` DOES execute | verify: `human — same config, run 'dva up' with no flag; assert UP_HAPPENED IS created. A fix that breaks this has disabled up entirely`
- [x] The plan route still honors --dry-run (no regression) | verify: `human — 'dva up p1 --dry-run' must still log msg=dry-run and leave UP_HAPPENED absent`
- [x] `dva stack up --dry-run` prints a plan instead of '[warn] no lifecycle entries matched filters' | verify: `human — assert --dry-run is not collected as an entry name and a real plan is shown`
- [x] `dva down --dry-run`'s recommended command actually works | verify: `human — the error text points at 'dva stack down --dry-run'; assert that command previews rather than silently no-opping`
- [x] A regression test asserts --dry-run reaches UpOptions.DryRun on the bare route, proven to fail without the fix | verify: `human — revert the parse, confirm the test FAILS for the right reason (side effect occurs), restore, confirm it passes`
- [x] The other 17 DisableFlagParsing commands are audited for the same drop | verify: `human — --debug and --json are the other root persistent flags; either fix centrally or document per-command which are unaffected and why`
- [x] `make test` and `go vet ./...` pass | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && make test && go vet ./...`

## Verification Record (2026-07-16)

All criteria measured, not assumed. The audit criterion resolved as **affected, different seam** — see
frontmatter `verification-summary` and TASK-048.

Fix shape chosen: `parseDvaFlags` (`compose.go:490`) rather than the per-command loop at
`compose.go:100-111`. It is the single seam all 12 lifecycle callers share (upCmd, teardownCommon,
downCmd, stopCmd, restartCmd, buildCmd, stackUp/Stop/Down, appUp/Restart/Build), so one 6-line case
fixes `up` **and** the `stack up`/`stack down` entry-name-swallowing defect together. `composeCmd`
(raw docker passthrough) was verified NOT to call it, so `docker compose --dry-run` still passes
through untouched.

Known remaining gap, deliberately not fixed here (minimal-change): `--dry-run=false` and `--dry-run=true`
are still not recognized (only the bare `--dry-run`), matching the existing hand-parsed idiom for
`--force`/`--no-wait`. This is not a regression — cobra never parsed them on these commands either.

## References

- [040-up-force-flag-documented-and-inert.md](./040-up-force-flag-documented-and-inert.md) — `--force` on the same command, same "documented and inert" class. **Check whether one central fix resolves both** before implementing either
- [028-flag-suppresses-default-plan-route.md](./028-flag-suppresses-default-plan-route.md) — also concerns `dva up` flag/route interaction; same file
- [044-legacy-structured-provision-shell-sleep-docker-inert.md](./044-legacy-structured-provision-shell-sleep-docker-inert.md) — the run's theme, inverted: that one silently does nothing, this one silently does everything
