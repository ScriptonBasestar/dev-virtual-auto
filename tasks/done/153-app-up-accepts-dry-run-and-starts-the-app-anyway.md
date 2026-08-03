---
id: TASK-153
title: "app up and app restart advertise --dry-run and start the app anyway"
type: bug
priority: P2
status: done
effort: S
created-at: 2026-08-03T14:30:00+09:00
source: "TASK-113 finalize verification — the honest-subset list has one dishonest entry"
depends-on: [TASK-113]
scope: "dva repo — internal/cli/app.go:168, :237"
---

# Task 153: Wire --dry-run through app up and app restart

## Problem

`--dry-run` is named as accepted on `dva app up` and `dva app restart`, and both start the
process for real. Measured 2026-08-03 on `bin/dva` v0.1.44 against a fixture whose only app is
`web` (`strategy: native`, `run: sleep 300`):

```
$ dva app up --dry-run
[+] started app web [native]
  NAME  STRATEGY  STATUS   HEALTH   URL  PID
  web   native    running  unknown  -    41642
                                    ^ a real PID; .sb/dva/logs/app-web.log was created
$ dva up --dry-run                                # the control, same fixture
[app] (dry-run) would start web [native]: sleep 300
```

So the flag that exists so a user can ask *what would happen* makes the thing happen.

## Cause

`app up` (`internal/cli/app.go:168-173`) and `app restart` (`:237-242`) build
`lifecycle.AppStartOptions{Names, DevMode, Wait, Mode}` with no `DryRun:` field. `app build`
(`:286`) and `dva up` (`internal/cli/compose.go:219`) both pass `DryRun: dryRun`.

The capability is present and reachable — `startWave` has the branch at
`internal/lifecycle/app_manager.go:128` (`if opts.DryRun { … "would start" … }`), which is what
prints the control line above. Two call sites simply do not set the field.

## Why this is TASK-113's residual and not a stale bug

The omission predates the fix (`git show 4bf8fe3^:internal/cli/app.go:139-144`). What TASK-113
added is the *claim*: `appSelectorFlags` (`internal/cli/stack.go:378`) lists `--dry-run` among
the flags the app family honours, and its Resolution justifies that list as "the honest subset",
chosen so that no `accepted here:` line states something false — see
`tasks/_archive/113-…md:166-171`, and the comment at `stack.go:374-377`, which says the
discarded flags are "silently ignored rather than rejected … a separate defect, recorded in
TASK-113". That record covers `--env` and the tag lists. It does not cover `--dry-run`, which is
listed on the *surviving* side with the reasoning that it "survives because `parseDvaFlags`
writes package globals". It survives parsing and is then dropped one layer down.

Not TASK-145's family: the token parses correctly and the `dryRun` global is set. Not TASK-146
either — that is `Explain` on the `dva run <interaction>` path.

## Acceptance criteria

- [x] `app up` and `app restart` pass `DryRun: dryRun` into `AppStartOptions`.
- [x] `dva app up/restart --dry-run` print the `would start` line, exit 0, start no process and
      create no log file — pinned by `TestStartAppsDryRunDoesNotSpawn`, which asserts no pidfile
      AND no log file under `.sb/dva/` (the load-bearing proof), plus the `would start` line.
- [x] The restart halt half is simulated too — see "Decision" below.
- [x] Tests cover both halves: `TestStartAppsDryRunDoesNotSpawn` (start half, no spawn) and
      `TestHaltAppsDryRunDoesNotSignal` (halt half, no signal). Each asserts the side effect, not
      just the printed line.
- [x] Remaining `AppStartOptions` sites — see "Construction sites" below (4 total; 2 omitted, now
      fixed; 2 already correct).
- [x] `make test` exits 0.

## Decision (restart halt half)

`app restart` runs `HaltApps` (SIGTERM) before `StartApps`. Rejecting the flag was ruled out:
criterion 2 requires `app restart --dry-run` to print `would start` and exit 0, which a rejection
(non-zero) would contradict. So both halves are simulated. A new `HaltAppsDryRun` names each
running app that would stop, without sending the signal; `app restart` calls it under `--dry-run`
and `HaltApps` otherwise.

> **Correction (review):** an earlier draft claimed `app stop` and `dva down` "have no `--dry-run`
> half to honour." That is wrong — both accept the flag (`dva stop` via `parseDvaFlags`, `app stop`
> via cobra's inherited persistent flag) yet call `HaltApps` unconditionally, so they send real
> SIGTERM under `--dry-run`. That is the same defect class as this task, in scope-out siblings —
> filed as a follow-up (see Review). This task's scope stays `app up` + `app restart` as written.

## Review

Independent `core:code-reviewer` pass: **No Critical/High.** Core fix sound (DryRun wiring
correct; `HaltAppsDryRun` withholds only the signal; tests assert side effects, not printed lines).

- **M1 (sibling defect, follow-up)** — the same asymmetry survives in `dva stop --dry-run`
  (`compose.go:399`) and `app stop --dry-run` (`app.go:76`): both accept the flag but call
  `HaltApps` unconditionally. Filed as TASK-166 rather than expanding this task's stated scope.
- **M2 (test-strength gap, documented)** — the two tests call the lifecycle layer directly, so
  dropping `DryRun: dryRun` from `app.go:173`/`:251` would not fail them. The lifecycle contract is
  pinned; the 2-line wiring is verified by inspection. A cobra-`RunE` test per command would close
  the gap; left as a possible follow-up.
- **L1** — `HaltAppsDryRun`'s print predicate probes the leader (`IsProcessRunning`) where HaltApps
  gates on the group signal succeeding; negligible (dva apps are group leaders via `Setpgid`).
  Comment softened to state the divergence.
- **L2/L3** — third copy of the pidfile-read block (a `liveRecordedPID` helper would help);
  construction-site census accurate for production (4 sites, 2 fixed). Informational.

## Construction sites

Four `AppStartOptions{}` sites exist; two omitted `DryRun`, two already passed it:

| site | DryRun before | after |
| --- | --- | --- |
| `internal/cli/compose.go:216` (`dva up`) | ✓ | — |
| `internal/cli/app.go:168` (`app up`) | ✗ | ✓ (this task) |
| `internal/cli/app.go:237` (`app restart`) | ✗ | ✓ (this task) |
| `internal/cli/app.go:283` (`app build`) | ✓ | — |

## Resolution

Added `DryRun: dryRun` to the `app up` and `app restart` `AppStartOptions`, and a
`HaltAppsDryRun` method on `AppManager` so the restart halt half is simulated too. The CLI
wiring is the two-line `DryRun` addition (the field is visibly present at both sites); the
behaviour it unlocks is pinned at the `AppManager` level by `TestStartAppsDryRunDoesNotSpawn`
(no pidfile, no log, `would start`) and `TestHaltAppsDryRunDoesNotSignal` (helper process left
alive, `would stop`). A CLI-level smoke run was blocked by the session's `cd`-in-compound-command
permission rule, so the proof stands on the unit tests plus wiring inspection rather than an
end-to-end `dva app up --dry-run`.

## Notes

`app build` is already correct, so the fix has a working model in the same file eight lines
away. That also means a reader comparing the three subcommands sees the field on one and not the
others, which is how this stayed invisible.
