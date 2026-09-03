---
id: TASK-279
title: "Repair plan-path flags that are accepted and then discarded"
type: bug
priority: P2
effort: S
exec-tier: standard
created-at: 2026-09-03T12:55:00+09:00
source: "TASK-273 audit — surfaced as evidence there, excluded from its scope as behaviour rather than guidance defects"
scope: "internal/cli/plan_lifecycle.go restart/stop/down plan routes, internal/lifecycle StopOptions/DownOptions"
status: todo
depends-on: []
---

# Task 279: repair plan-path flags that are accepted and then discarded

## Summary

`parsePlanFlags` accepts `--force` and `--no-wait` on every lifecycle verb, but two of the plan
routes drop what it parsed. `restart` overwrites `--force` with a hardcoded `true`, so the flag
does nothing and restart force-recreates whether or not it was typed. `stop` and `down` accept
`--no-wait` and then pass it to option structs that have no field to receive it. In both cases
the CLI answers exit 0, which reads as "your flag was honoured".

## Problem

1. **`restart` discards `flags.force` and hardcodes `Force: true`.**
   `runPlanRestart` builds `lifecycle.UpOptions{DryRun: effectiveDryRun, Force: true, Wait:
   flags.wait, ...}` (`/usr/bin/grep -n 'Force:  true' internal/cli/plan_lifecycle.go`), while
   the `up` route one screen earlier passes `Force: flags.force` faithfully
   (`/usr/bin/grep -n 'Force:  flags.force' internal/cli/plan_lifecycle.go`). Two consequences,
   and the second is the worse one:

   - `dva restart <plan> --force` and `dva restart <plan>` are the same command. The flag is
     accepted and has no effect.
   - Restart force-recreates unconditionally. A user who did *not* ask for `--force-recreate`
     gets it anyway, which for the compose plugin means containers are destroyed and rebuilt
     rather than restarted.

   The manifest describes the option as `Compose only: pass --force-recreate; other plugins
   ignore it` (`optForce`, `/usr/bin/grep -n 'optForce' internal/cli/manifest.go`). On the
   restart route that description is false in both directions: passing it changes nothing, and
   not passing it does not prevent it.

2. **`stop` and `down` accept `--no-wait` into a struct that cannot hold it.**
   `parsePlanFlags` sets `flags.wait = false` for `--no-wait`
   (`/usr/bin/grep -n 'flags.wait = false' internal/cli/plan_lifecycle.go`), but
   `lifecycle.StopOptions` and `lifecycle.DownOptions` declare no `Wait` field — only
   `UpOptions` does (`/usr/bin/grep -n 'type StopOptions' -A8 internal/lifecycle/orchestrator.go`).
   The value is parsed, stored, and never read.

   Measured against the binary built at `fdc6925`, on a fixture with one plan `local-dev`:

   ```
   $ dva stop local-dev --no-wait --dry-run   → [lifecycle] stopping db / stopping web   exit 0
   $ dva down local-dev --no-wait --dry-run   → [lifecycle] stopping db / stopping web   exit 0
   ```

   Neither warns. The manifest does not advertise `--no-wait` on `stop` or `down`
   (`/usr/bin/grep -n '"down":' -A2 internal/cli/manifest_static_commands_test.go`), so the
   parser is more permissive than the advertised surface — the flag is neither documented nor
   rejected, just absorbed.

## Direction

Two directions, and the card does not prejudge which:

- **(a) Honour the flags.** Pass `flags.force` through on `restart`; add `Wait` to
  `StopOptions`/`DownOptions` and thread it. This makes the accepted flags mean what their
  names say.
- **(b) Reject what is not implemented.** Make `parsePlanFlags` verb-aware so `--no-wait` on
  `stop`/`down` fails the way an unsupported plan flag already does, and decide explicitly
  whether restart's force is a flag or a property of the verb — if it is a property, say so in
  the manifest and reject `--force` there.

Direction (a) is the smaller change for `restart` and the larger one for `stop`/`down`, because
"wait" has no meaning for every plugin. An implementer should not mix directions across the two
defects without saying why in the commit.

## Completion Criteria

- [ ] `dva restart <plan> --force` and `dva restart <plan>` are distinguishable — either the flag reaches the orchestrator, or it is rejected on this route | verify: `go test ./internal/cli -count=1`
- [ ] Restart no longer force-recreates on behalf of a user who did not ask for it, or the manifest states that restart always force-recreates | verify: `go test ./internal/cli -count=1`
- [ ] `--no-wait` on `stop`/`down` either reaches the orchestrator or is rejected; it is not silently absorbed | verify: `go test ./internal/cli -count=1`
- [ ] A regression test pins the chosen behaviour for all three routes, so a later refactor cannot quietly restore the discard | verify: `go test ./internal/cli -count=1`
- [ ] `optForce`'s manifest text matches what every route actually does with the flag | verify: `human — read optForce against the up and restart call sites: the description must hold on both routes, not only on up`
- [ ] Repository gates pass | verify: `make lint && make test && make test-integration && make doc-check && make commit-check`

## Non-goals

- No change to `up`, which already passes `Force` and `Wait` faithfully.
- No change to which flags `parsePlanFlags` accepts beyond the two named here.
  `--tag`/`--exclude-tag`/`--mode`/`--env` are path-conditional and owned by
  [TASK-273](273-repair-misleading-cli-guidance.md).
- No change to the `--purge` confirmation gate, which was reviewed and closed in
  [PLAN-004](../plan/004-restore-documentation-truth.md).
