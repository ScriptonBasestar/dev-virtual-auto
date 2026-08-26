---
id: TASK-113
title: "`dva up` and the `dva app` family silently swallow unknown flags, and `app` turns them into app names"
type: fix
priority: P2
effort: M
status: done
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/cli/compose.go:122-133 (up), internal/cli/app.go:106-112 (app up), plus the sibling app subcommands; internal/lifecycle/app_manager.go:718-732 and :79-83 for the swallow"
verified-at: 2026-08-03T14:30:00+09:00
archived-at: 2026-08-03T14:30:00+09:00
verification-summary: |
  Every criterion measured against ./bin/dva with a fresh fixture (one app `web`, port
  13113, inert `echo` run/dev commands) at scratchpad/v113/, not read off the task file.
  All ten measured rows in the task's "Measured after the fix" table reproduce with the
  same exit codes; byte counts differ slightly on the `up` rows (243 vs 236, 216 vs 209)
  because the accepted-here list is longer than when the table was written — behaviour is
  identical.
  Controls hold: `dva app build` exit=0 ("no build command"), `dva up --no-wait --dry-run`
  exit=0. The `--var` warning is live on both spellings: `dva up --var FOO=bare` and
  `--var=FOO=bare` both exit 0 and print `[warn] --var applies only when running a plan`.
  `dva up --dev nosuchthing` exit=1 via the second rejectUpPositionalArg at compose.go:170.
  TASK-117's fix is visible too: `dva app up --dev` now exits 1 on the readiness FAIL
  rather than the exit=0 recorded in 113's control row.
  Repo left clean (`git status --porcelain` empty); no repo files written.
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
having started nothing**. The user asked to start their apps in dev mode and was told it worked.

**Correction from the measured run below:** this file originally said "printed nothing". It prints
the application status table. That is worse, not better — the output looks like a report of work
done, and the only thing distinguishing it from success is the `stopped` in a column the user has no
reason to be checking.

Note step 4 is not reached through the `len(c.Applications) == 0` guard at `app.go:115` — that
guard fires only when *no* applications are configured. With applications configured and a
mistyped flag, the guard passes and the silent path is taken.

## Why the difference matters

`dva stack up`, `dva stack down` and `dva infra` are protected: they call `rejectUnknownFlags`.
That is the existing, working pattern in this codebase — the fix is to extend it, not invent
anything.

So the repo already knows how to do this, and the two commands most likely to be typed by hand are
the ones that skipped it.

## Evidence status — reproduced against a built binary

Fixture `tmp/task-113/dva.yml`: one application `web`, `port: 13113`, `run.native` and `dev.native`
set to inert `echo` commands so a start that should not happen is visible if it does. `make build`,
then `bin/dva` run from that directory.

```
$ dva app ls
  web   stopped   stopped   -   http://localhost:13113   -      # the app exists

$ dva app up --dev=true            # mistyped flag
exit=0
(blank line)
Applications:
  NAME  STRATEGY  STATUS   HEALTH  URL                     PID
  web   stopped   stopped  -       http://localhost:13113  -

$ dva app up nosuchapp             # no flag involved at all
exit=0
  web   stopped   stopped  -       …                       -    # identical output

$ dva app up --dev=true --debug
DEBUG msg="app not found" name="--dev=true" err="application \"--dev=true\" not found"
DEBUG msg="app not found" name="--dev=true" err="application \"--dev=true\" not found"

$ dva up --forse                   # unknown flag on up
exit=0                             # --forse ignored; the up proceeded in full

$ dva up --force=true              # the habitual form
exit=0                             # identical to the line above — indistinguishable from success
```

Three things the reading did not predict:

1. The command prints a **status table**, not nothing. See the correction above.
2. The `app not found` Debug line is emitted **twice** for one name, so `ResolveApp` runs at least
   twice per unresolved name. Harmless, but it means the resolution path is entered more than once
   and a fix that errors on the first failure should not produce two messages.
3. `dva app up nosuchapp` is byte-for-byte identical to the mistyped-flag case. Proposed-fix point 4
   is therefore not a secondary concern: it is the same user-visible bug, and fixing only the flag
   parsing would leave an unchanged reproduction.

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

- [x] The current behaviour is reproduced and recorded | verify: `human — build with make build, run 'dva app up --dev=true' in a fixture with >=1 application, record exit code and full stdout+stderr` — done, see Evidence status; exit=0, 138 bytes of output, app left stopped
- [x] An unknown flag is an error on `dva up` | verify: `dva up --forse` — must exit non-zero and name the offending token — exit=1, 236 bytes, names `--forse` and suggests `dva up --force`
- [x] An unknown flag is an error on the `app` family | verify: `dva app up --dev=true` — must exit non-zero, and must not report success — exit=1, 216 bytes; same for `app restart --dev=true` (exit=1) and `app build --dokcer` (exit=1)
- [x] A named app that does not exist is an error | verify: `dva app up nosuchapp` — must exit non-zero naming the app; print the exit code — exit=1, 72 bytes: `no such application: nosuchapp / → declared in dva.yml: web`
- [x] The protected commands are unchanged | verify: `go test ./internal/cli/ -run 'Flag|Unknown|Reject'` — print the number of tests selected — 125 tests+subtests selected, `ok … 2.802s`
- [x] Regression tests exist for both loops | verify: `/usr/bin/grep -rc 'forse\|--dev=true' internal/cli/*_test.go` — non-zero — `app_args_test.go:7`, `stack_args_test.go:2`
- [x] Full suite passes | verify: `make test` — exit=0, 0 FAIL lines, 5 `ok` lines; `make lint` exit=0

## Resolution

### What changed

`rejectUnknownFlags` (`internal/cli/stack.go`) was generalised from the three `stack`
subcommands to the whole family. It took `path` and `noun` parameters so the message reads
`dva app up` rather than `dva stack app up`, and `known` moved from being assembled inside
the function to being supplied by the caller.

That last change was a reversal made mid-implementation and it is the load-bearing one.
The first version appended `stackSelectorFlags` unconditionally. But `app up/restart/build`
call `parseDvaFlags` and then keep **only its first return value** — `mode, _, _, _, args`
at `app.go:101`, `:183`, `:223` — so `--env` and both tag lists are consumed and discarded.
Advertising them as "accepted here" would have put a false statement inside an error message
whose entire job is to tell the user what works. Hence `appSelectorFlags`, the honest subset:
`--mode`, `-M`, `--dry-run`, `--debug`, `--json`. (`--dry-run`/`--debug`/`--json` survive
because `parseDvaFlags` writes package globals rather than returning them.)

| site | guard added |
| --- | --- |
| `compose.go` `dva up` | `default:` on the loop + `rejectUnknownFlags` + a second `rejectUpPositionalArg` on the leftovers |
| `app.go` `app up`, `app restart`, `app build` | `rejectUnknownFlags` before the `len(c.Applications) == 0` check |
| `app.go` `app up/restart/build/stop/down` | `validateAppNames` after it |

Two details worth keeping:

- **The `up` positional guard runs twice, deliberately.** `rejectUpPositionalArg` inspects
  `args[0]` only and returns nil on a leading dash, so `dva up --dev nosuchthing` passed it
  and the name was dropped by the loop. Measured before the fix: exit 0, whole stack started,
  argument silently gone. This was not in the original write-up — it was found while building
  the fix.
- **Flag rejection runs before the empty-`applications:` check, name validation after it.**
  A malformed argument is wrong whatever the config holds, and `noApplications` would
  otherwise have been handed `"--dve"` as though it were an application name. But an unknown
  *name* in a project with no `applications:` section deserves `noApplications`' explanation,
  not an empty "declared in dva.yml".

### Proposed-fix point 4, resolved differently than proposed

The write-up asked `StartApps` to distinguish "names supplied, none matched" from "no names
supplied". It is implemented in the CLI instead, as `validateAppNames` — the app family's
counterpart to the existing `validateStackNames`.

`HaltApps` and `DownApps` return no error at all, so lifecycle offers no single seam that
covers the family; `StartApps` and `BuildApps` would have been fixed and `app stop nosuchapp`
/ `app down nosuchapp` would have stayed silent. One CLI call per subcommand covers all five
with one message. `selectApps` is untouched.

### Mutation-tested

A passing suite proves nothing until it can fail. Each guard was reverted in turn:

| mutation | result |
| --- | --- |
| app-family `rejectUnknownFlags` fed `nil` | 4 subtests fail — `--dokcer` reported as `no such application` instead of `unknown flag` |
| `validateAppNames` fed `nil` | 6 subtests fail on `returned nil; an unresolved name is indistinguishable from success` |
| `compose.go` loop `default:` removed | 3 subtests fail on `the argument was discarded and the whole stack started` |

The seventh row of the second mutation panicked rather than asserting —
`context.WithCancel(nil)` at `app_manager.go:99`, because `cmd.Context()` is nil when `RunE`
is invoked directly from a test. A harness artifact, not a production path; cobra supplies a
context under `Execute`.

### One existing test caught a real regression

`TestUpWithoutPlansGuardOnlyInspectsPlanNameSlot` failed on `dva up --var FOO=bare`.
`--var` is a **known** flag on `up` — `upCmd.Long` documents it as "Ignored off the plan
path" — so rejecting it would have broken a documented contract that a previous task
deliberately established. It is now consumed by the loop **together with its value**, since
`FOO=bare` carries no leading dash and would otherwise have been rejected as a stray
positional.

Silently ignoring it is the very shape this task is about, so it now says so on stderr:
`[warn] --var applies only when running a plan ('dva up <plan>'); ignored here`. Exit code
and documented semantics are unchanged.

### Measured after the fix

```
dva app up --dev=true       exit=1  216 bytes   unknown flag "--dev=true" for "dva app up"
dva app up nosuchapp        exit=1   72 bytes   no such application: nosuchapp
dva app up wbe              exit=1   98 bytes   … Did you mean?  dva app up web
dva app build --dokcer      exit=1  260 bytes   … Did you mean?  dva app build --docker
dva app restart --dev=true  exit=1  221 bytes
dva app stop nosuchapp      exit=1   72 bytes
dva app down nosuchapp      exit=1   72 bytes
dva up --forse              exit=1  236 bytes   … Did you mean?  dva up --force
dva up --force=true         exit=1  209 bytes
dva up --dev nosuchthing    exit=1  203 bytes   unexpected argument 'nosuchthing'

controls, still working:
dva app up --dev            exit=0   starts web
dva app build               exit=0   "no build command"
dva up --no-wait            exit=0   starts the stack and web
```

First attempt at this table read `exit=1` for every row including the controls — zsh does not
word-split an unquoted `$cmd`, so each whole string arrived as a single argv element and every
run failed at `unknown command`. The numbers above are from argv arrays.

### Left open, deliberately

- `dva app up --dev` exits **0** while printing `[FAIL] app web: process did not listen on
  port 13113 within 30s`. That is [TASK-117](117-startapps-prints-fail-and-returns-nil.md),
  filed while reproducing this one, and it is why the control row above shows exit=0 next to a
  FAIL line.
- `--dev=true` gets no "Did you mean? --dev" suggestion — `levenshtein("--dev=true","--dev")`
  is 5, past the threshold of 2. The `accepted here:` line names `--dev`, so the message is
  still actionable.
- The `app not found` Debug line was emitted twice per unresolved name because `PortConflicts`
  calls `selectApps` again after `StartApps`. Unreachable now for unknown names (rejected
  earlier), but the duplicate call remains.
- ⚠️ `--dry-run` is in `appSelectorFlags` as an honest survivor, and on `app up` / `app restart`
  it is dropped one layer below parsing: neither passes `DryRun:` into `AppStartOptions`
  (`app.go:168`, `:237`), so `dva app up --dry-run` starts the process for real. `app build`
  (`:286`) and `dva up` both pass it. Found while verifying this task for archival; tracked as
  [TASK-153](../todo/153-app-up-accepts-dry-run-and-starts-the-app-anyway.md).

## Related

- [TASK-092](../_archive/092-stack-log-forwards-root-flags-to-docker.md) — the other end of the same
  problem: flags DVA should have consumed reaching docker. Here they reach nothing at all.
- [TASK-091](../_archive/091-compose-steps-stop-after-the-first-command.md) — also `exit 0` while doing
  less than asked; the recurring failure shape in this codebase is silence, not crashes.
- [TASK-117](117-startapps-prints-fail-and-returns-nil.md) — found while reproducing this one, on the
  same code path but a distinct defect: `StartApps` prints `[FAIL]` for a readiness failure and still
  returns nil. Fixing 113 alone still leaves `dva up` exiting 0 on an app that never started.
