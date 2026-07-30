---
id: TASK-087
title: "An unrecognized argument to `dva stack up` becomes a stack entry name, so a mistyped flag silently loses its effect and still exits 0"
type: fix
priority: P2
effort: M
status: todo
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/cli/stack.go — the hand-rolled arg loops behind DisableFlagParsing; internal/lifecycle/orchestrator.go:80-83"
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

- [ ] A mistyped flag is rejected, not renamed | verify: `dva stack up infra --nowait --dry-run` — must exit non-zero and name `--nowait`; today exit 0 with 0 mentions of it
- [ ] A name matching no entry is an error | verify: `dva stack up nosuchentry` — must exit non-zero; today 0
- [ ] The correctly-spelled flag still works | verify: `dva stack up infra --no-wait --dry-run` — compose args must contain `up -d` and NOT `--wait`, proving the fix did not break parsing
- [ ] Starting everything still works | verify: `dva stack up --dry-run` — must still start 2 entries on the fixture, so the new rejection is not over-broad
- [ ] The exit channel control is included in the test | verify: `human — the test must assert a known-failing case exits 1, or a "typo exits nonzero" assertion proves nothing`
- [ ] All four subcommands agree | verify: `grep -n 'DisableFlagParsing' internal/cli/stack.go` — 4 sites; each must reject an unknown `-`-prefixed arg
- [ ] Covered by a test that fails without the fix | verify: `go test ./internal/cli/ -run TestStackRejectsUnknownArgs`
- [ ] Full suite passes | verify: `make test`

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

- **`dva stack <unknown-subcommand>` prints help and exits 0** (measured: 1222 bytes of help,
  exit 0). Cobra normally rejects unknown subcommands. Same family — unrecognized input reported
  as success — but a different mechanism, so not folded in here.

## Related

- [TASK-085](../done/085-interaction-steps-silently-drop-compose-keys.md),
  [TASK-086](086-parallel-steps-discard-their-note.md) — the same silent-drop family found in the
  TASK-083 audit; this one differs by producing a *wrong action*, not just lost output.
- [TASK-079](../done/079-json-flag-does-not-cover-failures.md) — the machine-consumer thread: an
  exit code that says 0 is exactly the signal that task made loadbearing.
