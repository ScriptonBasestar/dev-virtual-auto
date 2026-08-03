---
id: TASK-117
title: "`StartApps` prints `[FAIL]` for a readiness failure and still returns nil, so `dva up` and `dva app up` exit 0"
type: fix
priority: P2
effort: M
status: done
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/lifecycle/app_manager.go:79-232 StartApps — three [FAIL] branches at :204, :220, :222 write to stderr but never append to errs; only :144 and :165 do"
---

# Task 117: DVA detects the failure, names it, prints it, and exits 0

## Measured, not inferred

Found while reproducing [TASK-113](../done/113-up-and-app-commands-swallow-unknown-flags.md) against a real
binary. It is a different defect that happens to be on the same path, so it is filed separately.

Fixture (`tmp/task-113/dva.yml`): one application `web`, `port: 13113`, `run.native: echo …`. The
echo exits immediately, so the port is never bound — a stand-in for any app that crashes on startup.

```
$ dva app up web
[+] started app web [native]
[FAIL] app web: process did not listen on port 13113 within 30s — see …/.sb/dva/logs/app-web.log
exit=0
```

`dva up` behaves the same way, printing the same `[FAIL]` and exiting 0.

DVA is not confused here. It waited the full 30s, correctly concluded the process never listened,
produced a precise message and a log path — and then reported success to the shell.

## Root cause

`StartApps` spans `internal/lifecycle/app_manager.go:79-232` and ends:

```go
	wg.Wait()

	return errors.Join(errs...)
```

Every failure it reports must reach `errs` to affect the exit code. Counted:

| line | what it reports | reaches `errs`? |
| --- | --- | --- |
| :144 | port already held by a PID dva did not start (pre-start check) | ✅ `errs = append(...)` |
| :165 | the start command itself returned an error | ✅ `errs = append(...)` |
| :204 | `[FAIL] app %s exited during startup` | ❌ stderr only |
| :220 | `[FAIL] app %s: port %d is held by PID %d, not the process dva started` | ❌ stderr only |
| :222 | `[FAIL] app %s: process did not listen on port %d within %s` | ❌ stderr only |
| :207 | `[warn] app %s not ready after %s` | ❌ stderr only |

`grep -n 'errs = append' internal/lifecycle/app_manager.go` returns exactly two lines, both above
:170. All three `[FAIL]` sites are below :200.

So the split is: **failures detected before the process starts are errors; failures detected after
it starts are only messages.** Nothing in the code states that as an intention, and the `[FAIL]`
prefix — used nowhere else for a non-fatal condition — says the opposite.

## Why this outranks its severity at first glance

The `[FAIL]` lines at :220 and :222 were added deliberately, with a comment at :212-216 explaining
that they catch "a child that crashed on bind and a green health probe that is really being
answered by a foreign orphan — both of which would otherwise be reported as a successful start."

That is the exact failure this task describes, and the fix stopped one half of it: the human reading
the terminal now sees the problem. The other half was left in place — the shell, CI, and any
`dva up && next-step` chain still see success. A wrapper script cannot read the `[FAIL]` line.

## The `[warn] not ready` case is a real decision, not an oversight

`:207` fires when a health check does not pass but the process is still alive. That is genuinely
ambiguous — a slow-starting app may become ready later — so it is listed above for completeness;
the three `[FAIL]` sites are the ones this task claims are wrong.

> **Correction, made during implementation.** The sentence originally continued "…and turning it
> into an error would change behaviour for anyone relying on `wait: false` semantics." That reason
> is wrong: the goroutine returns at `if !opts.Wait { return }` well above this branch, so
> `wait: false` never reaches it. The reason that does hold is that DVA cannot tell "slow" from
> "broken" from this signal alone. [TASK-118](../done/118-a-health-check-that-never-passes-is-still-exit-0.md)
> carries the decision, and the hole it leaves.

Decide `:207` explicitly rather than sweeping it in.

## Proposed fix

1. Append an error at :204, :220 and :222 alongside the existing print, so `errors.Join` carries
   them. Keep the printed message — it is better than the joined error text for a human.
2. Make sure the message is not then printed twice by the CLI layer's own error handling. Check
   what `dva app up` does with a returned error before assuming.
3. Decide `:207` on the record.

## Acceptance criteria

- [x] A readiness failure exits non-zero | verify: `human — with tmp/task-113/dva.yml, run 'dva app up web'; print exit code; must be non-zero`
- [x] `dva up` agrees | verify: `human — same fixture, 'dva up'; print exit code; must be non-zero`
- [x] The message is not duplicated | verify: `human — same run; count the [FAIL] lines; must be 1`
- [x] A healthy app still exits 0 | verify: `human — fixture whose run command actually binds its port; print exit code; must be 0`
- [x] Unit coverage for the join | verify: `go test ./internal/lifecycle/ -run 'StartApps' -v` — print the number of tests selected; a readiness failure must produce a non-nil error
- [x] Full suite passes | verify: `make test`

## Resolution

### What changed

`internal/lifecycle/app_manager.go` — `startWave` gained a `recordErr` closure and now uses it at
all five failure sites, so `grep -c 'errs = append'` returns **1** (inside `recordErr`) and
`grep -c 'recordErr('` returns **5**. The three post-start `[FAIL]` branches reach `errors.Join`;
the printed lines are unchanged, because a terminal message and a returned error want different
wording and the returned one is deliberately terser.

`internal/cli/compose.go` — the fix to `StartApps` was **not enough on its own**. `dva up` was
swallowing the result into `[warn] app start: %v`, so the app manager could report every failure it
liked and the command still returned nil. It now holds the result and ends in
`errors.Join(upErr, appErr)`, after the status and endpoint tables have printed.

`internal/cli/app.go` — `dva app up` did return the error, but immediately, which after this change
would have cost the user the status table exactly when it matters most: `dva app up web api` with
one bad app still needs to say what the good one is listening on. The error is now held in
`startErr` and joined with the `PortConflicts` check at the end, matching `dva up`'s ordering.

### Measured

Failing fixture `tmp/task-113/dva.yml` (`run.native: echo …`, never binds port 13113):

```
$ dva app up web                        $ dva up
exit=1                                  exit=1
[FAIL] lines: 1                         [FAIL] lines: 1
ERROR: lines: 1                         [warn] app start lines: 0
status table printed: 1                 elapsed 0.5s (not the full 30s timeout)
```

Healthy control `tmp/task-117/dva.yml` (`run.native: nc -l 13117`, actually binds):

```
$ dva app up web → exit=0, [FAIL] lines: 0, listener on 13117: 1
$ dva up         → exit=0, [FAIL] lines: 0
$ dva app down web → listener on 13117: 0
```

`make test` exit 0 (5 packages, 0 FAIL), `make lint` exit 0 (`0 issues`, 215 files formatted).

### The tests, and what proves they mean anything

`internal/lifecycle/app_start_exit_test.go` selects 4 tests (1 is the helper process, which skips
unless invoked as one):

| test | branch it pins |
| --- | --- |
| `TestStartAppsErrorsWhenProcessNeverListens` | `:245` — port never bound |
| `TestStartAppsErrorsWhenAppExitsDuringHealthCheck` | `:217` — process died under the probe |
| `TestStartAppsReturnsNilWhenProcessOwnsItsPort` | the control: a real bind must still return nil |

The healthy control starts **this test binary** re-invoked as `TestAppListenerHelper` rather than
`nc`, because BSD, openbsd, nmap-ncat and GNU netcat do not agree on how to spell "listen on a
port" — a control that skips on the CI host is not a control.

Each was mutation-tested by reverting the change it covers; all three mutants died:

| mutation | test | result |
| --- | --- | --- |
| drop `recordErr` on "did not listen" | `…ProcessNeverListens` | FAIL ✅ |
| drop `recordErr` on "exited during startup" | `…ExitsDuringHealthCheck` | FAIL ✅ |
| `recordErr` on every start, healthy or not | `…OwnsItsPort` | FAIL ✅ |

`diff` against the pre-mutation backup confirmed the file was restored byte-for-byte.

### Left open, deliberately

`:228` — `[warn] app %s not ready after %s` — is still a warning and still exits 0. The code comment
there now says so and points at [TASK-118](../done/118-a-health-check-that-never-passes-is-still-exit-0.md),
which also records the hole that leaves: **an app that binds its port but never answers its probe is
caught by neither branch.** Promoting it changes the exit code of every project with a slow or flaky
probe, which is a product decision, not a defect to fix unilaterally.

`:242` — port held by a foreign PID after our process started — is covered by `recordErr` but not by
a unit test. Reaching it hermetically means holding the port with a foreign listener while dva's
own child is alive, and the pre-start preflight at `:155` catches that shape first. `PortConflicts`
covers the same condition from the other side in `portowner_test.go`.

## Related

- [TASK-113](../done/113-up-and-app-commands-swallow-unknown-flags.md) — found during its reproduction. 113
  is about input DVA never validates; this is about output DVA validates correctly and then discards.
- [TASK-091](../done/091-compose-steps-stop-after-the-first-command.md) and
  [TASK-079](../_archive/079-json-flag-does-not-cover-failures.md) — the same shape. The recurring defect
  in this codebase is not a wrong answer, it is a correct answer that does not reach the exit code.
