---
id: TASK-092
title: "`dva stack log` forwards DVA's own root flags into docker's argv, so `--debug` becomes a `docker compose logs` argument"
type: fix
priority: P3
effort: S
status: done
created-at: 2026-07-31T08:10:00+09:00
closed-at: 2026-07-31T00:00:00+09:00
scope: "internal/cli/stack.go:261-284 — stackLogCmd never calls parseDvaFlags before the passthrough"
verified-at: 2026-08-03T13:20:00+09:00
archived-at: 2026-08-03T13:20:00+09:00
verification-summary: |
  All three leaking call sites exist and are exercised: stack.go:320, compose.go:43 (composeCmd),
  compose.go:604 (logsCmd), each calling consumeRootPersistentFlags (root.go:276), which applies
  and strips only --debug/--json and leaves --dry-run/--mode/--env untouched.
  Live binary against a docker shim on PATH:
    dva --debug stack log infra --tail=5 --since=1h -> `compose -f … logs --tail=5 --since=1h`
    dva --debug --json compose logs --tail=5        -> `compose -f … logs --tail=5`
    dva --debug logs --tail=5                       -> `compose -f … logs --tail=5`
    dva stack log infra --dry-run                   -> `compose -f … logs --dry-run`
  All three pre-fix leaks from the task's evidence table are gone; --debug still turned debug
  logging on in each case, so it is consumed, not merely deleted.
  go test ./internal/cli/ -run TestStackLog: 8 subtests PASS (pattern matches real tests).
  The "Left open" ktl site is closed: kubectl.go:32 uses the same helper and
  TestKtlDoesNotForwardRootFlags (5 subtests) passes; tasks/done/103-… is done.
  Audit claim of 17 DisableFlagParsing commands checks out: 18 occurrences in internal/cli/*.go,
  one of which is in hooks_test.go.
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

- [x] `--debug` no longer reaches docker | verify: `dva --debug stack log infra --tail=5` — the `[debug] compose:` argv must not contain `--debug` after `logs`
- [x] User flags still pass through | verify: same command — `--tail=5` must still appear, or the fix has broken the passthrough it was protecting
- [x] `--dry-run` still forwarded | verify: `dva stack log infra --dry-run` — must still reach docker, per the carve-out at `root.go:253`
- [x] Covered by a test | verify: `go test ./internal/cli/ -run TestPassthrough` — renamed from `TestStackLog…` by the command-surface restructure (`docs/43`); same two tests, same prefix-matching shape

> **Note added by the restructure.** `dva stack log` was removed. The first two criteria
> moved to `dva logs`, which strips root flags through the same `consumeRootPersistentFlags`
> call. The third could not: `logs` is a hookable built-in, so `wrapWithHooks` runs
> `consumeDryRunFlag` before the body and the token is DVA's before any passthrough sees it.
> `dva stack log` was not hookable, which made it the only command that ever demonstrated
> the `--dry-run` carve-out — `root.go`'s comment named `dva logs` alongside it, and that
> half was already untrue when written. The row now runs on `dva compose`, the one raw
> passthrough that is still not hookable, and `root.go`'s comment says so.
- [x] Full suite passes | verify: `make test`

## Resolution

Fixed in `internal/cli/root.go` (new `consumeRootPersistentFlags`), with call sites in
`internal/cli/stack.go` and `internal/cli/compose.go` (×2). Test:
`internal/cli/root_flag_passthrough_test.go`.

### The task named one leaking site; there were three

The Proposed fix said to "check `execComposePassthrough`'s other callers for the same gap".
There were two, and one is worse-positioned than the site that got the task filed:

| command | pre-fix argv handed to docker | leaked |
|---|---|---|
| `dva --debug stack log infra --tail=5` | `compose -f … logs --debug infra --tail=5` | `--debug` |
| `dva --debug --json compose logs` | `compose -f … --debug --json logs` | `--debug`, `--json` |
| `dva --debug logs --tail=5` | `compose -f … logs --debug --tail=5` | `--debug` |

The middle row is the one to notice. Those args are appended *before* the compose
subcommand, so the two flags were offered to `docker compose` itself rather than to its
`logs` subcommand — a different and slightly worse failure than the filed one.

### Why not `parseDvaFlags`

The obvious fix — reuse the helper every sibling command uses — is wrong here. `parseDvaFlags`
also consumes `--dry-run`, `--mode`, `--env` and the tag flags. On these three commands
`--dry-run` belongs to docker, and `root.go`'s existing comment already records that carve-out.
Routing the passthrough through `parseDvaFlags` would have fixed the leak by breaking the
passthrough it exists to protect. `consumeRootPersistentFlags` therefore consumes `--debug`
and `--json` only, and the "`--dry-run` is still forwarded" criterion is a regression test for
exactly that.

Matching is exact, like `applyRootPersistentFlagsFromArgs` above it: `--debug=true` is neither
applied nor stripped. That is a pre-existing limitation of every raw-arg scanner in the file
(`applyRootPersistentFlagsFromArgs`, `parseDvaFlags`, `consumeDryRunFlag`); handling it in the
new helper alone would leave the family disagreeing about what a root flag is. Pinned by a
test rather than left implicit — see "Left open".

### Non-vacuity

The test asserts on argv captured from a real `docker` shim on `PATH` (with a `t.Fatal` guard
unless `docker` resolves to the shim), not on simulated state. Four independent probes, each
reverting one call site and each expected to fail:

| probe | result |
|---|---|
| revert `stack log` | FAIL — `--debug reached docker` |
| revert `compose` | FAIL — `--debug`, `--json` reached docker |
| revert `logs` | FAIL — `--debug reached docker` |
| revert all three | FAIL — all of the above |

`forceSubprocess` is what makes this testable at all. The passthrough normally ends in
`syscall.Exec`, which replaces the *test binary* — the exec'd program's exit status becomes the
test's, and a broken assertion reports `ok`. That is the failure mode that cost
[TASK-094](094-kubectl-runner-discards-steps.md) two false-passing probes.
`forceSubprocess` is the existing seam (`execution_paths_test.go` already drives it), so the
passthrough runs as a subprocess and returns.

`TestStackLogRootFlagsStillTakeEffect` is the other half: a fix that merely deleted `--debug`
from the argv would satisfy every "absent" assertion above. Note it is deliberately stricter
than production — in the real binary `--debug` also survives a naive strip, because
`PersistentPreRun` reads `os.Args` before any `RunE` runs. That is why the pre-fix binary still
printed its `[debug] compose:` trace *while* leaking the flag.

### Left open

- **A fourth site: `dva ktl`** (`internal/cli/kubectl.go:34,66`) appends raw `args` straight into
  `kubectl`'s argv via `ExecReplace`, so `dva --debug ktl get pods` sends `--debug` to kubectl.
  Same defect class, found by an audit of all 17 `DisableFlagParsing` commands commissioned
  while closing this task. Filed as [TASK-103](103-ktl-forwards-root-flags-to-kubectl.md)
  rather than folded in here: it exec's kubectl rather than docker and has no `forceSubprocess`
  seam, so it needs child-process test machinery of its own.
- The audit cleared the other 13: 11 strip via `parseDvaFlags`, and `dva infra up`/`infra down`
  reject any `-`-prefixed arg before it can reach an external process.
- `--debug=true` and a literal `--` end-of-flags separator remain unhandled, family-wide.

## Related

- [TASK-087](../_archive/087-unrecognized-stack-args-become-entry-names.md) — found while tracing
  `stack log`'s passthrough to decide whether it should reject unknown flags. It must not;
  that is what makes this leak visible.
- [TASK-103](103-ktl-forwards-root-flags-to-kubectl.md) — the fourth site, split out.
