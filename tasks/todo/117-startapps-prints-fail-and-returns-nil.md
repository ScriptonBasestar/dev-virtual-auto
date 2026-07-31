---
id: TASK-117
title: "`StartApps` prints `[FAIL]` for a readiness failure and still returns nil, so `dva up` and `dva app up` exit 0"
type: fix
priority: P2
effort: M
status: todo
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
ambiguous — a slow-starting app may become ready later — and turning it into an error would change
behaviour for anyone relying on `wait: false` semantics. It is listed above for completeness; the
three `[FAIL]` sites are the ones this task claims are wrong.

Decide `:207` explicitly rather than sweeping it in.

## Proposed fix

1. Append an error at :204, :220 and :222 alongside the existing print, so `errors.Join` carries
   them. Keep the printed message — it is better than the joined error text for a human.
2. Make sure the message is not then printed twice by the CLI layer's own error handling. Check
   what `dva app up` does with a returned error before assuming.
3. Decide `:207` on the record.

## Acceptance criteria

- [ ] A readiness failure exits non-zero | verify: `human — with tmp/task-113/dva.yml, run 'dva app up web'; print exit code; must be non-zero`
- [ ] `dva up` agrees | verify: `human — same fixture, 'dva up'; print exit code; must be non-zero`
- [ ] The message is not duplicated | verify: `human — same run; count the [FAIL] lines; must be 1`
- [ ] A healthy app still exits 0 | verify: `human — fixture whose run command actually binds its port; print exit code; must be 0`
- [ ] Unit coverage for the join | verify: `go test ./internal/lifecycle/ -run 'StartApps' -v` — print the number of tests selected; a readiness failure must produce a non-nil error
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-113](../done/113-up-and-app-commands-swallow-unknown-flags.md) — found during its reproduction. 113
  is about input DVA never validates; this is about output DVA validates correctly and then discards.
- [TASK-091](../done/091-compose-steps-stop-after-the-first-command.md) and
  [TASK-079](../done/079-json-flag-does-not-cover-failures.md) — the same shape. The recurring defect
  in this codebase is not a wrong answer, it is a correct answer that does not reach the exit code.
