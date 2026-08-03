---
id: TASK-087
title: "An unrecognized argument to `dva stack up` becomes a stack entry name, so a mistyped flag silently loses its effect and still exits 0"
type: fix
priority: P2
effort: M
status: done
created-at: 2026-07-31T00:00:00+09:00
completed-at: 2026-07-31T08:10:00+09:00
scope: "internal/cli/stack.go — the hand-rolled arg loops behind DisableFlagParsing; internal/lifecycle/orchestrator.go:80-83"
verified-at: 2026-08-03T13:00:00+09:00
archived-at: 2026-08-03T13:00:00+09:00
verification-summary: |
  Re-measured against ./bin/dva (v0.1.44) on the task's own reproduction fixture (two compose
  entries pointing at an absent does-not-exist.yml). All eight criteria hold.
  Rejections: `stack up infra --nowait --dry-run` → exit 1, names --nowait, suggests --no-wait;
  `--forse`, `--nowait` with no NAME, and `infr` all exit 1; `nosuchentry` exits 1 identically on
  up, stop and down. Controls unchanged: `--no-wait` → `up -d` (0 × --wait), default → `up -d
  --wait`, `--force` → `+ --force-recreate`, `down -v` → `down --remove-orphans --volumes`,
  bare `stack up --dry-run` → 2 entries, exit 0. Exit channel proven live by `stack up infra`
  without --dry-run → exit 1.
  Implementation is at internal/cli/stack.go:417 (rejectUnknownFlags) and :451
  (validateStackNames), called from up/stop/down at :94/:97, :168/:171, :231/:234 — exactly the
  three sites the criterion names, with `stack log` excluded and pinned by
  TestStackLogKeepsForwardingUnknownFlags. The two error strings exist nowhere else in internal/,
  so the regression test's assertions are genuine discriminators, not tautologies.
  Follow-up TASK-092 exists at tasks/done/092-stack-log-forwards-root-flags-to-docker.md.
  Note: both items the task listed under "Left open" have since been closed by later work —
  `dva stack nosuchsubcmd` now exits 1, and `dva stack status nosuchentry` now errors via
  validateStackNames (stack.go:274). The section is stale, not owed.
---

# Task 087: a mistyped flag is read as an entry name and then thrown away

## Problem

`stack up`/`stop`/`down`/`log` set `DisableFlagParsing: true` and parse their own arguments.
Everything the hand-rolled loop does not recognise falls into the `default` branch and becomes a
**stack entry name** — `internal/cli/stack.go:65-74`:

```go
for _, a := range names {
    switch a {
    case "--force":   force = true
    case "--no-wait": noWait = true
    default:
        filteredNames = append(filteredNames, a)   // "--nowait" lands here
    }
}
```

Downstream, `filterByNames` (`internal/lifecycle/orchestrator.go:395-407`) keeps entries whose
name is in the requested set. It never asks the reverse question — whether every *requested* name
was found — so a name matching nothing contributes nothing and is never reported. The only
feedback exists when the result is completely empty, and it is a stderr warning with a **`return
nil`** (`orchestrator.go:80-83`):

```go
if len(filtered) == 0 {
    fmt.Fprintln(os.Stderr, "[warn] no lifecycle entries matched filters")
    return nil
}
```

A typo therefore has two outcomes, both exit 0: the flag quietly loses its effect, or the whole
selection empties and nothing runs.

## Evidence (measured on 0.1.44, all under `--dry-run`)

**A mistyped flag loses its effect.** The dry-run line shows the exact compose invocation, so the
flag's effect is directly observable:

| invocation | compose args produced | effect |
| --- | --- | --- |
| `stack up infra --no-wait` (control) | `… up -d` | flag applied |
| `stack up infra --nowait` | `… up -d --wait` | **flag silently ignored** |

Exit 0, one entry still started, and `--nowait` appears **0 times** anywhere in stdout or stderr —
no complaint of any kind.

**With no NAME, the same typo empties the selection.** This is the realistic CI shape:

| invocation | entries started | exit |
| --- | --- | --- |
| `stack up --no-wait` (control) | **2** | 0 |
| `stack up --nowait` | **0** | **0** |

The user asked to start the whole stack; nothing started; the command reported success. In a
pipeline, `dva stack up --nowait && echo DEPLOY-OK` prints `DEPLOY-OK` over an empty stack.

`--forse` (typo of `--force`) behaves identically: 0 mentions in the output, entry starts without
`--force-recreate`.

**The exit code is not simply always 0 — that is what makes this a defect and not a convention.**
The positive control matters here:

| bad input | exit | behaviour |
| --- | --- | --- |
| valid name, compose file absent | **1** | `ERROR: entry "infra" up failed: compose config is invalid…` with two `→` hints |
| `--mode nosuchmode` | **1** | `ERROR: mode 'nosuchmode' not found. No modes defined…` |
| `--env nosuchenv` | **1** | error |
| `dva ls --nosuchflag`, `dva validate --nosuchflag` | **1** | cobra rejects the flag |
| **`stack up nosuchentry`** | **0** | generic stderr warn |
| **`stack up --nowait`** | **0** | generic stderr warn |

So the project already validates modes, envs and flags elsewhere and reports them well. The
stack subcommands' positional NAME is the one selector that does not, and `DisableFlagParsing`
extends that hole to cover every flag those four commands do not explicitly list.

A first attempt at this measurement produced a **failed positive control** — `--nosuchflag` and
`dva stack nosuchsubcmd` both exited 0, which looked like "this command can never exit nonzero"
and would have made the whole finding vacuous. Only running a genuinely failing case (valid name,
absent compose file → exit 1) established that the exit channel works, which is what makes the 0s
above meaningful. Keep that control in any test written for this.

## Root cause, stated once

There is no such thing as an unrecognized argument on this path. Every argument is either a known
flag or a name, and an unmatched name is not an error. The fix has to introduce the missing
category.

## Proposed fix

1. **Reject unknown `-`-prefixed arguments** in the hand-rolled loops. Anything starting with `-`
   that is not a known flag is a user error, not an entry name — no stack entry can be named
   `--nowait`. This alone kills the entire flag-typo class and is a few lines at each site.
2. **Report requested names that matched nothing.** `filterByNames` should return the unmatched
   subset so the caller can fail with `no stack entry named "infr"` (plus a did-you-mean, since
   the entry list is right there). This is the wrong-answer half: today `stack up infr` and
   `stack up infra` differ only in exit-code-invisible ways.
3. Decide what `[warn] no lifecycle entries matched filters` + `return nil` should be. It is
   defensible for a *tag* filter that legitimately matches nothing, but not for an explicitly
   named entry. Splitting those two cases is the smallest honest change.

Blast radius to cover: `DisableFlagParsing` is set at `stack.go:51` (up), `:131` (stop), `:177`
(down), `:260` (log). Three route args through `parseDvaFlags` (`:59`, `:139`, `:185`) and two
carry the `default → filteredNames` funnel (`:72`, `:195`). Fix them together or the inconsistency
just moves.

## Non-goals

- Not removing `DisableFlagParsing`. It exists so DVA can hand off flags to the underlying plugin;
  that design stays. This is about the arguments it fails to classify.
- Not changing `--tag` behaviour. A tag matching nothing is plausibly legitimate; a *name*
  matching nothing is not. Do not silently promote the tag case into an error.

## Acceptance criteria

- [x] A mistyped flag is rejected, not renamed | verify: `dva stack up infra --nowait --dry-run` — must exit non-zero and name `--nowait`; today exit 0 with 0 mentions of it — **exit 1, 1 mention, plus a did-you-mean pointing at `--no-wait`**
- [x] A name matching no entry is an error | verify: `dva stack up nosuchentry` — must exit non-zero; today 0 — **exit 1, and identically on `stop` and `down`**
- [x] The correctly-spelled flag still works | verify: `dva stack up infra --no-wait --dry-run` — compose args must contain `up -d` and NOT `--wait`, proving the fix did not break parsing — **`up -d`; the default is `up -d --wait` and `--force` still adds `--force-recreate`**
- [x] Starting everything still works | verify: `dva stack up --dry-run` — must still start 2 entries on the fixture, so the new rejection is not over-broad — **2 entries, exit 0**
- [x] The exit channel control is included in the test | verify: `human — the test must assert a known-failing case exits 1, or a "typo exits nonzero" assertion proves nothing` — **`TestStackAcceptsKnownArgsAndNames`: 5 invocations that must return nil *and* touch their marker file. Inverted but equivalent — "returns an error" only discriminates if a neighbouring case returns nil**
- [x] The subcommands that own their arguments agree, and the one that forwards them is excluded on evidence | verify: `grep -n 'rejectUnknownFlags(' internal/cli/stack.go` — **3 call sites: up, stop, down. Criterion rewritten; see "Why three sites, not four" below**
- [x] Covered by a test that fails without the fix | verify: `go test ./internal/cli/ -run TestStackRejectsUnknownArgs` — **8 subtests; with both helpers stubbed to `return nil` all 8 fail and the 5 accepting subtests stay green**
- [x] Full suite passes | verify: `make test` — **all packages ok under `-race`; `internal/cli` at 60.9%**

## Resolution

Two helpers in `internal/cli/stack.go`, called from `up`, `stop` and `down` after
`parseDvaFlags` has taken its share:

```go
rejectUnknownFlags("up", filteredNames, "--force", "--no-wait")  // a leading dash is not a name
validateStackNames(c, "up", filteredNames)                       // a name must match c.Stack
```

`validateStackNames` checks against `c.Stack` — the same map `SortedStack` builds the
orchestrator's entries from — so a name it accepts cannot then fail to match in
`filterByNames`, and it cannot reject a name that would have worked. Both messages reuse
`levenshtein` (`root.go:358`) for a did-you-mean, matching what `resolveProvisionProfile`
already does for provision profiles.

| invocation | before | after |
| --- | --- | --- |
| `stack up infra --nowait --dry-run` | exit 0, `up -d --wait`, 0 mentions | **exit 1**, names `--nowait`, suggests `--no-wait` |
| `stack up --nowait --dry-run` | exit 0, **0 entries started** | **exit 1** |
| `stack up infra --forse --dry-run` | exit 0, no `--force-recreate` | **exit 1**, suggests `--force` |
| `stack up nosuchentry` / `stop` / `down` | exit 0, generic warn | **exit 1**, `no such stack entry: nosuchentry` |
| `stack up infr` | exit 0 | **exit 1**, suggests `infra` |
| `stack up infra --no-wait --dry-run` (control) | `up -d` | `up -d` — unchanged |
| `stack up --dry-run` (control) | 2 entries | 2 entries — unchanged |
| `stack down -v --dry-run` (control) | `down --remove-orphans --volumes` | unchanged |

### Why three sites, not four

The criterion asked for all four `DisableFlagParsing` sites. Measured, that would have
been wrong: `stack log` hands its arguments straight to docker.

```
$ dva --debug stack log infra --tail=5 --since=1h
[debug] compose: docker [compose -f … logs --debug infra --tail=5 --since=1h]
```

`--tail` and `--since` are docker's flags, not DVA's, and rejecting them would delete a
working feature — which the task's own non-goal ("it exists so DVA can hand off flags to
the underlying plugin; that design stays") already forbade. The real distinction is not
"does it disable flag parsing" but **"does it consume its arguments or forward them"**:
up/stop/down consume them as selector NAMEs and must classify every one; `log` forwards
them and must not. `TestStackLogKeepsForwardingUnknownFlags` pins both halves.

### Item 3 of the proposed fix: decided, not changed

`orchestrator.go:80-83` (`[warn] no lifecycle entries matched filters` + `return nil`) is
left as it stands. With names validated at the CLI boundary the remaining ways to empty
the selection are a tag, mode, or environment filter that matches nothing — and the task's
own non-goal protects the tag case as legitimate. Splitting the warning would now be
splitting a case that no longer carries user typos.

### Found while fixing

- **`--debug` leaks into docker's argv on the `stack log` path** — see the trace above:
  `logs --debug infra …`. `stack log` never calls `parseDvaFlags`, so root persistent flags
  are not stripped before the passthrough. Filed as
  [TASK-092](../done/092-stack-log-forwards-root-flags-to-docker.md).
- **`dva stack up --var FOO=x` now exits 1** where it used to be silently swallowed. That
  is the behaviour archived [TASK-027](../_archive/027-up-silently-ignores-unknown-args.md)
  called "correct" when it measured `dva run --var` rejecting the same flag, and it closes
  half of that task's out-of-scope note. Whether the stack path should instead *honor*
  `--var` remains open there — this change only stops it lying about having done so.
- `internal/cli/stack.go` carries pre-existing gofmt drift (77 diff lines, pure
  indentation) owned by TASK-078. New lines match the surrounding block rather than
  gofmt, so the eventual reformat stays one atomic change.

## Reproduction fixture

```yaml
version: "0.1.44"
stack:
  infra:
    order: 1
    default_runner: compose
    runners:
      compose:
        files: [does-not-exist.yml]
  cache:
    order: 2
    default_runner: compose
    runners:
      compose:
        files: [does-not-exist.yml]
```

The absent compose file is deliberate: it keeps every probe safe under `--dry-run` and doubles as
the exit-1 positive control when run without it.

## Left open

Both re-measured after the fix and still exit 0 — different mechanisms, deliberately not
folded in:

- **`dva stack <unknown-subcommand>` prints help and exits 0** (1222 bytes of help, exit 0).
  Cobra normally rejects unknown subcommands.
- **`dva stack status nosuchentry` prints an empty table and exits 0** (314 bytes). It has
  its own inline `nameSet` filter rather than going through the orchestrator, and cobra does
  parse its flags, so neither helper here reaches it.

## Related

- [TASK-085](085-interaction-steps-silently-drop-compose-keys.md),
  [TASK-086](086-parallel-steps-discard-their-note.md) — the same silent-drop family found in the
  TASK-083 audit; this one differs by producing a *wrong action*, not just lost output.
- [TASK-079](../_archive/079-json-flag-does-not-cover-failures.md) — the machine-consumer thread: an
  exit code that says 0 is exactly the signal that task made loadbearing.
- [TASK-027](../_archive/027-up-silently-ignores-unknown-args.md) — the same defect one command
  over. It fixed `dva up <typo>`'s plan-name half and left the flag half open, warning that a
  guard scanning every argument would misread `--var FOO=x`. This guard classifies by leading
  dash instead of by position, so that hazard does not arise.
- [TASK-092](../done/092-stack-log-forwards-root-flags-to-docker.md) — found by the trace that
  decided the `stack log` exclusion.
