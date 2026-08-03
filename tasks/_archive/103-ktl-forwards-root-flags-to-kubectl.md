---
id: TASK-103
title: "`dva ktl` forwards DVA's own root flags into kubectl's argv, so `--debug` becomes a kubectl argument"
type: fix
priority: P3
effort: S
status: done
created-at: 2026-07-31T00:00:00+09:00
closed-at: 2026-07-31T00:00:00+09:00
scope: "internal/cli/kubectl.go:34,66 — ktlCmd appends raw args to kubectl's argv on both exec paths"
verified-at: 2026-08-03T13:55:00+09:00
archived-at: 2026-08-03T13:55:00+09:00
verification-summary: |
  Fix is one line, internal/cli/kubectl.go:32, placed after loadEnv and before the args[0]
  entry lookup, reusing consumeRootPersistentFlags (internal/cli/root.go:276, also used at
  compose.go:43, compose.go:604, stack.go:320).
  Regression test internal/cli/ktl_flag_passthrough_test.go: 5 child-process cases, all PASS,
  each logging the argv a kubectl shim actually received.
  Non-vacuity proven by `go test -overlay` with the fix line stripped: 4 FAIL naming the
  leaked flags, plus `multiple kubectl entries: alpha, beta` for the placement case; the
  --dry-run=client control still passes (it is a control against over-stripping).
  End-to-end on bin/dva v0.1.44 + kubectl shim confirms both `dva --debug ktl …` and
  `dva ktl --debug …` strip the flag while still enabling debug logging.
  internal/cli: 600 RUN / 600 PASS / 0 FAIL under -v (hazard-checked, not a bare `ok`).
  DisableFlagParsing audit re-measured: 17 registered commands + 1 hooks_test.go fixture,
  matching the task's table exactly; ktl is the only former leak site.
---

# Task 103: `ktl` leaks root flags into kubectl

## Problem

The fourth instance of the defect [TASK-092](../_archive/092-stack-log-forwards-root-flags-to-docker.md)
fixed for docker. `ktlCmd` sets `DisableFlagParsing: true` (`kubectl.go:19`) and its only
argument guard is `helpRequested(args)` (`kubectl.go:21`). It never calls `parseDvaFlags` nor
the `consumeRootPersistentFlags` helper TASK-092 added, so DVA's own root flags travel into
kubectl's argv on **both** exec paths:

- no-kubectl-entries fallback — `kubectl.go:30-35`
- resolved-entry path — `kubectl.go:62-67`

Both end in `dvaexec.ExecReplace(e, "kubectl", kubectlArgs, false)`, i.e. `syscall.Exec` of the
real kubectl binary.

```go
kubectlArgs = append(kubectlArgs, passArgs...)          // kubectl.go:66
return dvaexec.ExecReplace(e, "kubectl", kubectlArgs, false)
```

So `dva --debug ktl get pods` is expected to reach kubectl as `kubectl get pods --debug`
(or `kubectl --namespace X get pods --debug` when the entry declares one).

## How it was found

An audit of every `DisableFlagParsing: true` command in `internal/cli/`, commissioned while
closing TASK-092 to check whether the three sites fixed there were the whole set. 17 real
commands examined (an 18th `grep` hit is an inline `cobra.Command` fixture in `hooks_test.go`,
not a registered command):

| verdict | count | commands |
|---|---|---|
| leaks | 1 | `ktl` — this task |
| fixed in TASK-092 | 3 | `compose`, `logs`, `stack log` |
| safe: strips via `parseDvaFlags` | 11 | `app up/restart/build`, `up`, `down`, `stop`, `restart`, `build`, `stack up/stop/down` |
| safe: rejects flags before exec | 2 | `infra up`, `infra down` |

The `infra` pair is worth recording as *why* it is safe rather than assuming it: it only calls
`consumeDryRunFlag`, but `resolveInfraTargets` (`infra.go:53-74`) hard-errors on any remaining
`-`-prefixed token before anything reaches the orchestrator, so a root flag produces a clean
error instead of a leak.

## Why it is not folded into TASK-092

Same defect class, but a different target and no test seam. The docker passthroughs route
through `execComposePassthrough`, which honours the package-global `forceSubprocess` and so can
be driven from a test that survives to assert. `ktl` calls `ExecReplace` directly, so a
naive test would `syscall.Exec` kubectl **over the test binary** — kubectl's exit status
becomes the test's, and the assertions never run while `go test` prints `ok`. That is the
exact false-pass that cost [TASK-094](../_archive/094-kubectl-runner-discards-steps.md) two
probes.

## Proposed fix

Apply `consumeRootPersistentFlags(args)` in `ktlCmd.RunE` after `loadEnv`, before the entry
resolution at `kubectl.go:42` — it must run before `args[0]` is read as an entry name, or
`dva --debug ktl myentry ...` would look up `--debug` as the entry. Keep `--dry-run` forwarded
for the same reason TASK-092 did, unless kubectl's own `--dry-run` semantics argue otherwise
(kubectl's is `--dry-run=client|server`, a value flag — worth confirming while fixing).

Test with the child-process pattern from `internal/runner/kubectl_steps_test.go`: re-exec the
test binary with an env marker, put a `kubectl` shim first on a PATH rebuilt as
`shim:/bin:/usr/bin`, and `t.Fatal` unless `exec.LookPath("kubectl")` resolves to the shim —
a real kubectl is at `/opt/homebrew/bin/kubectl` on this machine and must not be reachable.

## Acceptance criteria

- [x] `--debug` no longer reaches kubectl | verify: child-process test asserts the captured kubectl argv contains no `--debug`
- [x] Both exec paths covered | verify: one case with zero kubectl entries (`kubectl.go:34`) and one with a resolved entry (`kubectl.go:66`)
- [x] User args still pass through | verify: `get pods -o wide` survives intact, or the fix broke the passthrough
- [x] Entry-name resolution still works with a leading root flag | verify: `dva --debug ktl <entry> get pods` resolves `<entry>`, not `--debug`
- [x] Test is non-vacuous | verify: reverting the fix makes it FAIL, and the failure names the leaked flag
- [x] Full suite passes | verify: `make test`

## Resolution

One line in `internal/cli/kubectl.go` — `args = consumeRootPersistentFlags(args)` after
`loadEnv`, reusing the helper [TASK-092](../_archive/092-stack-log-forwards-root-flags-to-docker.md) added.
Test: `internal/cli/ktl_flag_passthrough_test.go`.

### Measured, on a kubectl shim

Four cases, each a real `ktlCmd.RunE` in a child process that ends in `syscall.Exec` of a
shim that prints its argv:

| case | argv kubectl received |
|---|---|
| zero kubectl entries (`kubectl.go:34`) | `get pods` |
| one resolved entry (`kubectl.go:66`) | `--namespace my-namespace get pods -o wide` |
| entry name behind a leading `--debug` | `--namespace ns-beta get pods` |
| kubectl's own `--dry-run=client` | `--namespace my-namespace apply -f pod.yaml --dry-run=client` |

### Placement is load-bearing, and the probe proves it

The strip has to run **before** the `args[0]` entry lookup at `kubectl.go:42-51`, not after.
Reverting the fix fails all three strip cases, and the third fails in the way that shows why:

```
zero-entry fallback:  --debug reached kubectl in "--debug get pods"
resolved entry:       --debug reached kubectl in "--namespace my-namespace --debug --json get pods -o wide"
                      --json  reached kubectl in "--namespace my-namespace --debug --json get pods -o wide"
entry name:           ktl returned multiple kubectl entries: alpha, beta
```

That last one is not a leak — it is the command failing outright, because without the strip
`--debug` is what gets tried as the entry name. So a fix applied one statement later would
have turned a wrong-argv bug into a can't-run-at-all bug.

The `--dry-run=client` case still passes with the fix reverted. That is correct: it is a
control against over-stripping, not a test of the strip, and it would fail only if someone
"fixed" this with `parseDvaFlags`.

### Why a child process

`ktl` calls `dvaexec.ExecReplace` → `syscall.Exec` directly and has no `forceSubprocess` seam
like the compose passthroughs. In-process, the test binary would be *replaced* by kubectl and
kubectl's exit status would become the test's — `go test` prints `ok` over assertions that
never ran. Each case therefore re-execs the test binary with an env marker; the parent reads
the shim's line out of the child's output.

The child rebuilds `PATH` as `shim:/bin:/usr/bin` and `t.Fatal`s unless
`exec.LookPath("kubectl")` is the shim. That guard is not decoration: this process is about to
`syscall.Exec` whatever `kubectl` resolves to, and a real one lives at
`/opt/homebrew/bin/kubectl` on this machine.

### Left open

- Bare `--dry-run` is forwarded to kubectl, matching the compose sites and
  `applyRootPersistentFlagsFromArgs`'s carve-out. kubectl's own flag is the value form
  (`--dry-run=client`), which is what the test pins; whether kubectl accepts the bare form was
  not measured, so `dva --dry-run ktl …` may or may not be meaningful. Not changed here
  because diverging from the other three passthroughs needs its own decision.
- `--debug=true` is still neither applied nor stripped, family-wide — see TASK-092.
- ~~The fixtures use the deprecated top-level `kubectl:` form because the modern one is
  invisible to `KubectlEntries()`~~ — closed by
  [TASK-102](../_archive/102-detectplugin-blind-to-runners-form.md), which added a `runners.kubectl` case
  here rather than re-pointing the existing ones: keeping both shapes means a regression in
  either is caught.

## Related

- [TASK-092](../_archive/092-stack-log-forwards-root-flags-to-docker.md) — same defect, docker side;
  contributes the `consumeRootPersistentFlags` helper this task reuses
- [TASK-094](../_archive/094-kubectl-runner-discards-steps.md) — source of the child-process test
  pattern, and of the `syscall.Exec`-in-a-test false-pass this task must avoid
- [TASK-102](../_archive/102-detectplugin-blind-to-runners-form.md) — `ktl`'s entry resolution
  (`kubectl.go:46`, `KubectlEntries`) is also blind to the `runners:` form; independent defect
  in the same command
