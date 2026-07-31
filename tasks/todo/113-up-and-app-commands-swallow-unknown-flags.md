---
id: TASK-113
title: "`dva up` and the `dva app` family silently swallow unknown flags, and `app` turns them into app names"
type: fix
priority: P2
effort: M
status: todo
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/cli/compose.go:122-133 (up), internal/cli/app.go:106-112 (app up), plus the sibling app subcommands; internal/lifecycle/app_manager.go:718-732 and :79-83 for the swallow"
---

# Task 113: a typo'd flag that reports success and does nothing

## Problem

Two different hand-rolled argument loops, two different failure modes, both silent.

### `dva up` — the flag is ignored

`internal/cli/compose.go:122-133`:

```go
for _, a := range args {
	switch a {
	case "--force":
		force = true
	case "--no-wait":
		noWait = true
	case "--dev":
		devMode = true
	case "--docker":
		docker = true
	}
}
```

The switch matches whole strings and has no `default`, and no `rejectUnknownFlags` call follows.
So `--force=true`, `--forse`, `--no_wait` and every other near-miss fall through to nothing. The
command runs as if the flag had not been passed and exits 0.

`--force=true` is the one that matters: it is the form a user writes by habit from every other CLI,
and it is indistinguishable from success.

### `dva app up` — the flag becomes an app name

`internal/cli/app.go:106-112`:

```go
for _, a := range args {
	if a == "--dev" {
		devMode = true
	} else {
		appNames = append(appNames, a)
	}
}
```

Anything that is not exactly `--dev` is treated as the name of an application to start. Follow it
down:

| step | code | behaviour |
| --- | --- | --- |
| 1 | `app.go:110` | `--dev=true` is appended to `appNames` |
| 2 | `app_manager.go:724-727` | `ResolveApp` fails, `am.logger.Debug("app not found", …)`, `continue` |
| 3 | `app_manager.go:722` | `selected` ends up empty |
| 4 | `app_manager.go:81-83` | `if len(apps) == 0 { return nil }` |

The Debug line is invisible without `--debug`. `StartApps` returns nil, so the command **exits 0
having started nothing and printed nothing**. The user asked to start their apps in dev mode and
was told it worked.

Note step 4 is not reached through the `len(c.Applications) == 0` guard at `app.go:115` — that
guard fires only when *no* applications are configured. With applications configured and a
mistyped flag, the guard passes and the silent path is taken.

## Why the difference matters

`dva stack up`, `dva stack down` and `dva infra` are protected: they call `rejectUnknownFlags`.
That is the existing, working pattern in this codebase — the fix is to extend it, not invent
anything.

So the repo already knows how to do this, and the two commands most likely to be typed by hand are
the ones that skipped it.

## Evidence status

Structurally confirmed by reading all four files listed in `scope`. Two independent probe sessions
also measured it against a real `bin/dva`; their reports agreed on behaviour. **A measured
reproduction against a built binary has not yet been recorded in this task** — that is the first
acceptance criterion, and it should produce the exit code and the empty output, not a description.

## Proposed fix

1. Route both loops through `rejectUnknownFlags` after the recognised flags are consumed, matching
   `stack`/`infra`.
2. Accept the `--flag=value` form for the booleans, or reject it with a message naming the bare
   form — silently ignoring it is the one option that must go.
3. For the `app` family specifically, a leftover token that starts with `-` must never become an
   app name.
4. `selectApps` returning empty when names *were* supplied is not the same event as no names being
   supplied. `StartApps` should distinguish them rather than returning nil for both.

Point 4 is the deeper one: even with flag parsing fixed, `dva app up nosuchapp` today exits 0 in
silence. That is the same defect reachable without any flag at all.

## Acceptance criteria

- [ ] The current behaviour is reproduced and recorded | verify: `human — build with make build, run 'dva app up --dev=true' in a fixture with >=1 application, record exit code and full stdout+stderr`
- [ ] An unknown flag is an error on `dva up` | verify: `dva up --forse` — must exit non-zero and name the offending token
- [ ] An unknown flag is an error on the `app` family | verify: `dva app up --dev=true` — must exit non-zero, and must not report success
- [ ] A named app that does not exist is an error | verify: `dva app up nosuchapp` — must exit non-zero naming the app; print the exit code
- [ ] The protected commands are unchanged | verify: `go test ./internal/cli/ -run 'Flag|Unknown|Reject'` — print the number of tests selected
- [ ] Regression tests exist for both loops | verify: `grep -rc 'forse\|--dev=true' internal/cli/*_test.go` — non-zero
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-092](../done/092-stack-log-forwards-root-flags-to-docker.md) — the other end of the same
  problem: flags DVA should have consumed reaching docker. Here they reach nothing at all.
- [TASK-091](../done/091-compose-steps-stop-after-the-first-command.md) — also `exit 0` while doing
  less than asked; the recurring failure shape in this codebase is silence, not crashes.
