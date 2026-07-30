---
id: TASK-092
title: "`dva stack log` forwards DVA's own root flags into docker's argv, so `--debug` becomes a `docker compose logs` argument"
type: fix
priority: P3
effort: S
status: todo
created-at: 2026-07-31T08:10:00+09:00
scope: "internal/cli/stack.go:261-284 — stackLogCmd never calls parseDvaFlags before the passthrough"
---

# Task 092: root flags travel into the plugin command

## Problem

Every other `DisableFlagParsing` command routes its arguments through `parseDvaFlags`,
which consumes the root persistent flags (`--dry-run`, `--debug`, `--json`) so they do not
reach the plugin. `stackLogCmd` skips that step and appends `args` to `logs` directly
(`stack.go:276` and `:282`), so DVA's flags become docker's.

## Evidence (measured on 0.1.44)

```
$ dva --debug stack log infra --tail=5 --since=1h
[debug] compose: docker [compose -f …/does-not-exist.yml logs --debug infra --tail=5 --since=1h]
```

`--debug` is DVA's — it turned on the very trace that printed this line — and it still
lands in the argv, positioned as a flag of `docker compose logs`. The user-supplied
`--tail=5 --since=1h` pass through correctly, which is the intended behaviour and the
control that shows the problem is specific to the root flags.

`--json` behaves the same way. `--dry-run` is the interesting one: `applyRootPersistentFlagsFromArgs`
(`root.go:253`) deliberately does **not** touch `--dry-run` because "compose passthrough must
keep docker's own `--dry-run`" — so any fix here has to keep that carve-out rather than
strip all three uniformly.

## Why it is P3

`docker compose logs --debug` is a wrong argv, but docker rejects or ignores it loudly
rather than doing something unintended, and the flag combination is rare. It is filed
because it is a concrete divergence from how every sibling command handles the same flags,
not because it is presently harmful.

## Proposed fix

Route `stackLogCmd`'s args through `parseDvaFlags` (or a narrower helper that consumes only
`--debug`/`--json`) before building the passthrough, keeping `--dry-run` forwarded as
`root.go:253` already documents. Check `execComposePassthrough`'s other callers
(`compose.go:553`) for the same gap while there.

## Acceptance criteria

- [ ] `--debug` no longer reaches docker | verify: `dva --debug stack log infra --tail=5` — the `[debug] compose:` argv must not contain `--debug` after `logs`
- [ ] User flags still pass through | verify: same command — `--tail=5` must still appear, or the fix has broken the passthrough it was protecting
- [ ] `--dry-run` still forwarded | verify: `dva stack log infra --dry-run` — must still reach docker, per the carve-out at `root.go:253`
- [ ] Covered by a test | verify: `go test ./internal/cli/ -run TestStackLog`
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-087](../done/087-unrecognized-stack-args-become-entry-names.md) — found while tracing
  `stack log`'s passthrough to decide whether it should reject unknown flags. It must not;
  that is what makes this leak visible.
