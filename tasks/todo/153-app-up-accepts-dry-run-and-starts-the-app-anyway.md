---
id: TASK-153
title: "app up and app restart advertise --dry-run and start the app anyway"
type: bug
priority: P2
status: todo
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

- [ ] `app up` and `app restart` pass `DryRun: dryRun` into `AppStartOptions`.
- [ ] `dva app up --dry-run` and `dva app restart --dry-run` print the `would start` line, exit
      0, start no process and create no log file. Prove the last two — a PID column and
      `ls .sb/dva/logs/`, not just the absence of the `[+] started` line.
- [ ] Decide what `app restart --dry-run` should say about the halt half. `HaltApps`
      (`app.go:236`) runs before `StartApps`, so a dry run currently stops the app and then
      reports what it *would* start. Either both halves are simulated or the command rejects
      the flag; record which and why.
- [ ] A test covers each of the two subcommands and fails if the field is dropped again.
      Asserting on the printed line alone is not enough — assert no process was spawned.
- [ ] Check the remaining `AppStartOptions` construction sites for the same omission and say
      how many there are.
- [ ] `make test` exits 0.

## Notes

`app build` is already correct, so the fix has a working model in the same file eight lines
away. That also means a reader comparing the three subcommands sees the field on one and not the
others, which is how this stayed invisible.
