---
id: TASK-100
title: "`DVA_HOOK_DEPTH=1` is inherited by every `syscall.Exec`-replaced child, so a nested `dva` silently skips its own hooks"
type: fix
priority: P4
effort: S
status: done
created-at: 2026-07-31T00:00:00+09:00
resolved-at: 2026-07-31T00:00:00+09:00
resolution: "A, corrected — EnvSlice filters the guard out of the os.Environ() passthrough; hook steps opt back in via WithHookDepth"
scope: "internal/cli/hooks.go:52-53 — Setenv plus a defer that cannot fire on the ExecReplace path; internal/config/environment.go:88-106 — EnvSlice passes os.Environ() through"
verified-at: 2026-08-03T13:40:00+09:00
archived-at: 2026-08-03T13:40:00+09:00
verification-summary: |
  Fix is present and behaves as documented. internal/config/environment.go:121 drops
  EnvHookDepthKey from the os.Environ() passthrough; WithHookDepth (environment.go:94-100)
  returns a *copy* whose Vars re-add the guard, and EnvSlice appends Vars after the filter,
  so ExecReplace'd targets (internal/exec/exec.go:28-29) lose it while hook children
  (provision.go:514, :538) keep it. hooks.go:65 hands the copy to all three phases.
  Three regression tests pass and were independently mutation-tested via `go test -overlay`
  (no repo file touched): disabling the filter, disabling WithHookDepth, and making it mutate
  in place each kill exactly one test with the recorded message. `go test ./internal/cli/
  -run Hook` selects 18 real tests, ok. `make lint` → `0 issues.`
  The recorded residual is confirmed real and correctly scoped: exec.go:81-85
  ExecSubprocessOutput sets no Cmd.Env, so os.Setenv at hooks.go:59 still reaches its docker
  child. Follow-on commit 91ce03b hardened the same boundary — internal/runner
  TestEnvVarsExcludesDVAPrefix asserts DVA_HOOK_DEPTH never crosses into a compose container
  (PASS).
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

## Resolution

**Option A, with a correction that A as written required.**

### Why literal A would have been a regression

A says the guard "has no business in a child's environment". That is true of one child and false
of another, and `EnvSlice` cannot tell them apart because it serves both:

| call site | child | should it see the guard? |
| --- | --- | --- |
| `internal/exec/exec.go:28` — `ExecReplace` → `syscall.Exec` | the target `docker`/`kubectl` | **no** — this is the leak |
| `internal/cli/provision.go:516` — `runShellCommand` | a **hook step**'s `sh -c` | **yes** — this is what the guard is for |
| `internal/cli/provision.go:492` — `runProvisionCompose` | a hook step's compose call | yes |

A blanket filter in `EnvSlice` closes the first row and opens a worse hole in the second: a hook
step that shells back into `dva` would no longer be suppressed, its nested `dva` would run its own
hooks, and those hooks would shell back into `dva` again. Unbounded recursion is strictly worse
than the latent leak being fixed.

### What was implemented

The guard stops travelling by ambient inheritance and starts travelling explicitly.

1. `EnvSlice` skips `EnvHookDepthKey` in the `os.Environ()` passthrough. Not-leaking is now the
   default, so no future `ExecReplace` call site has to remember anything — which is the part of
   A's rationale worth keeping, and the reason B was rejected.
2. `Environment.WithHookDepth()` returns a **copy** whose `Vars` carry `DVA_HOOK_DEPTH=1`.
   `EnvSlice` appends `Vars` after the filter and is not subject to it, so hook children opt back
   in. `wrapWithHooks` passes that copy to all three `runHookSteps` phases.

The copy is not stylistic. `cli.loadEnv` (`root.go:356-359`) caches one `*Environment` in a package
global and returns the same pointer to the hook executor and to the built-in path that reaches
`ExecReplace`. Mutating in place would have put the guard straight back into the target's
environment — the bug, reintroduced by the fix. `TestWithHookDepthDoesNotMutateSource` pins it.

`os.Setenv` at `hooks.go:52` is **kept**. It no longer carries the guard to hook steps, but it
still covers subprocesses that inherit this process's environment wholesale rather than through
`EnvSlice`. There is exactly one such production call site:
`internal/runner/docker_compose.go:216` → `ExecSubprocessOutput("docker", "compose", "ps", ...)`,
which sets no `Cmd.Env`. Its child is docker, which cannot recurse into `dva`, so the residual is
recorded rather than closed.

### The gap that let this sit

`grep` over `internal/**/*_test.go` found **no test asserting either direction** before this
change. A blanket strip would therefore have passed `make test` green while introducing the
recursion. Three tests now pin both directions and the aliasing hazard.

### Non-vacuity, measured rather than assumed

Each new assertion was mutation-tested — the fix was disabled, the test was confirmed to fail for
the right reason, and the disable was reverted:

| mutation | result |
| --- | --- |
| `if false && key == EnvHookDepthKey` (filter off) | `TestEnvSliceDropsHookDepth` FAILs: slice grows 93 → 94 entries, reports `[DVA_HOOK_DEPTH=1]` |
| `WithHookDepth` stops setting the var | `TestWithHookDepthCarriesGuardToHookChildren` FAILs: `got [], want exactly [DVA_HOOK_DEPTH=1]` |
| both reverted | `ok` — and Go's content-keyed test cache reported `(cached)`, confirming the revert was byte-exact |

`TestEnvSliceDropsHookDepth` also carries its own control: it fails fast if the guard is not in the
process environment to begin with, and if `EnvSlice` returns zero entries. Without those, "no
`DVA_HOOK_DEPTH` in the output" would be satisfied by an empty slice.

## Acceptance criteria

- [x] The child does not inherit the guard | verify: `go test ./internal/config/ -run HookDepth -v` — `TestEnvSliceDropsHookDepth`, 93 entries returned, 0 matching `DVA_HOOK_DEPTH`
- [x] The guard still works where it is needed | verify: `go test ./internal/config/ -run HookDepth -v` — `TestWithHookDepthCarriesGuardToHookChildren` asserts a hook child gets exactly `[DVA_HOOK_DEPTH=1]`
- [x] `forceSubprocess` behaviour is unchanged | verify: `go test ./internal/cli/ -run Hook` — 18 tests selected, ok
- [x] Not vacuous | verify: mutation table above — both directions fail when their fix is disabled, and both revert to green
- [x] Full suite passes | verify: `make test` — all ok; `internal/config` coverage 66.6% → 66.7%. `make lint` reports `0 issues.`

## Related

- [TASK-091](../_archive/091-compose-steps-stop-after-the-first-command.md) — the audit of every
  `ExecReplace` call site that turned this up. All 7 sites are otherwise correct: none is in a
  loop, none has stranded code after the call.
