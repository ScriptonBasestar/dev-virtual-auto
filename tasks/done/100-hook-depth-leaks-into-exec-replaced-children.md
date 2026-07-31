---
id: TASK-100
title: "`DVA_HOOK_DEPTH=1` is inherited by every `syscall.Exec`-replaced child, so a nested `dva` silently skips its own hooks"
type: fix
priority: P4
effort: S
status: todo
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/cli/hooks.go:52-53 — Setenv plus a defer that cannot fire on the ExecReplace path; internal/config/environment.go:88-106 — EnvSlice passes os.Environ() through"
---

# Task 100: a recursion guard that outlives the process it was guarding

## Problem

`wrapWithHooks` sets the guard and registers its own cleanup:

```go
_ = os.Setenv(config.EnvHookDepthKey, "1")
defer func() { _ = os.Unsetenv(config.EnvHookDepthKey) }()
```

When `original(cmd, args)` ends in `dvaexec.ExecReplace`, `syscall.Exec` replaces the process
image and the deferred `Unsetenv` never runs. That part is harmless on its own — the process
ceases to exist, so nothing in *this* process can observe the stale variable.

The part that is not harmless: `Environment.EnvSlice()` builds its slice from `os.Environ()` at
call time, which is **after** the `Setenv`. So `DVA_HOOK_DEPTH=1` is in the environment handed to
the exec'd `docker`/`kubectl` process, and to everything that process spawns. If anything in that
subtree invokes `dva`, the nested `dva` reads the guard at `hooks.go:34-36` and silently skips its
own before/replace/after hooks.

## Two audits, two different answers — both correct about different things

One audit called this harmless (the defer's failure to fire has no observable effect in a replaced
process); the other called it a live leak (the variable is in the child's environment regardless).
Both are right: the *defer* is a non-issue, the *inheritance* is the actual defect. Recording it so
the question is not re-litigated.

## Scope

Reachable only on the real `ExecReplace` path, i.e. when `forceSubprocess` is false — which is the
common case, since `forceSubprocess` is set only when `len(ic.After) > 0`. Of the 7 hookable
commands (`up, down, stop, restart, build, clean, logs`) only `build` and `logs` reach
`ExecReplace` at all; the rest route through `lifecycle.NewOrchestrator`, which has no
`ExecReplace` call sites.

No recursive `dva` invocation exists inside this repo, so the leak is currently **latent** — a
compose entrypoint or a user script that shells back into `dva` is what would surface it. P4 for
that reason, not because the mechanism is uncertain.

## Options

- **A — strip the key in `EnvSlice`.** The guard is process-local state; it has no business in a
  child's environment. Narrow and matches intent.
- **B — unset it immediately before `ExecReplace`** rather than relying on a defer that cannot run.
  Requires every call site to remember, which is the shape that produced this.
- **C — pass the depth in-process** instead of through the environment, and keep the variable only
  for the genuine subprocess case that needs it to cross a process boundary.

A is the smallest change that fixes the class rather than the instance.

## Acceptance criteria

- [ ] The child does not inherit the guard | verify: PATH-shadowed shim that prints its own environment — `DVA_HOOK_DEPTH` must be absent; print the grep count
- [ ] The guard still works where it is needed | verify: the genuine subprocess-recursion case must still be suppressed — print evidence that a nested hook did not re-run
- [ ] `forceSubprocess` behaviour is unchanged | verify: `go test ./internal/cli/ -run Hook` — print the number of tests selected
- [ ] Not vacuous | verify: human — revert the fix and confirm the shim sees the variable again
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-091](../done/091-compose-steps-stop-after-the-first-command.md) — the audit of every
  `ExecReplace` call site that turned this up. All 7 sites are otherwise correct: none is in a
  loop, none has stranded code after the call.
