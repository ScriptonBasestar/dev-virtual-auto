---
id: TASK-098
title: "`dva stack status <typo>` and `dva stack <unknown-subcommand>` still report success over work that did not happen"
type: fix
priority: P3
effort: S
status: done
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/cli/stack.go — stackStatusCmd's inline nameSet filter; the stack parent command's unknown-subcommand handling"
---

# Task 098: the two exit-0 paths TASK-087 deliberately left behind

[TASK-087](../_archive/087-unrecognized-stack-args-become-entry-names.md) fixed the orchestrator-backed
subcommands and recorded these two in its *Left open* because they fail through different
mechanisms. This file promotes them to their own task so they are worked rather than remembered.

## Measured at `2e0cfd6`

Fixture: one real entry `realstack`, compose runner pointed at a nonexistent file.

| command | exit | stdout | stderr |
| --- | --- | --- | --- |
| `dva stack up <typo>` | **1** | 0 | `ERROR: no such stack entry: <typo>` |
| `dva stack stop <typo>` | **1** | 0 | same |
| `dva stack down <typo>` | **1** | 0 | same |
| `dva stack status <typo>` | **0** | 34 bytes | only a plugin WARN |
| `dva stack <unknown-subcommand>` | **0** | 1222 bytes of help | — |

The first three rows are TASK-087's fix, re-confirmed here — a prior audit reported them as still
broken, which was a measurement against a pre-087 binary. The last two rows are the live defect.

Negative control: `dva stack up --mode nonexistent-mode` → exit 1, so the command is fully capable
of reporting failure on this same path.

## Why each one escapes

- **`status`** has its own inline `nameSet` filter rather than going through
  `orchestrator.filterEntries`, so neither helper TASK-087 added reaches it. A name that matches
  nothing produces an empty table, which is indistinguishable from "the stack is empty".
- **unknown subcommand** — cobra normally rejects these; something in the stack parent's
  configuration (likely `Run`/`RunE` being set, or `DisableFlagParsing`) turns the rejection into
  a help print. Needs a look before choosing the fix.

## Acceptance criteria

- [x] `stack status <typo>` fails | verify: exit must be 1 and stderr must name the entry; print both
- [x] A real name still works | verify: `dva stack status realstack` exits 0 and prints the row — the control that separates "rejects typos" from "rejects everything"
- [x] Bare `dva stack status` is unchanged | verify: byte-compare stdout before and after
- [x] Unknown subcommand fails | verify: `dva stack nosuchsub` exits non-zero; print the exit code and the message
- [x] `dva stack --help` still exits 0 | verify: print the exit code — an explicit help request is not an error
- [x] Not vacuous | verify: human — revert each hunk alone and confirm its own row regresses
- [x] Full suite passes | verify: `make test`

## Resolution

Two hunks, because the two rows fail through unrelated machinery.

**1. `stack status` validates its names** (`stack.go`). TASK-087 already left the right helper
behind — `validateStackNames(c, sub, names)` — and `status` simply never called it. The call goes
in before `orch.Status()` runs, matching the sibling subcommands, so a typo costs nothing.

**2. The parent rejects unknown subcommands** (`stack.go`). `Args: cobra.NoArgs` and a `RunE`
are a pair and neither works alone:

- cobra's `legacyArgs()` lets a **non-root** command with subcommands accept arbitrary args for
  backwards compatibility, so `dva stack nosuchsub` reached this parent rather than being
  rejected at dispatch;
- and `Command.execute()` returns `flag.ErrHelp` for a command with no `Run`/`RunE` **before** it
  ever calls `ValidateArgs`, so `NoArgs` on its own would never be consulted. `ExecuteC` turns
  that `ErrHelp` into a help print and a nil error — the exit 0.

Making the parent runnable moves the unknown subcommand into `NoArgs`' hands. The `RunE` body is
reached only with no args, where it keeps printing help and exiting 0.

`Use` also changed from `"stack [command]"` to `"stack"`: cobra prints `UseLine()` as an extra
Usage row once a command is runnable, and the old value rendered it as
`dva stack [command] [flags]` — implying the runnable form takes a subcommand, which `NoArgs`
rejects. Now the block reads `dva stack [flags]` / `dva stack [command]`, which is both accurate
and cobra's standard shape. This is the only change to `dva stack` output: +1 line, nothing removed.

**3. A latent bug this surfaced** (`root.go`). `suggestCommands` ranges `config.ReservedCommands()`
— **top-level names only** — and the caller fed it `args[0]`. That was correct while
"unknown command" could only come from the top level, where `args[0]` *is* the unknown token.
Once a nested subcommand could produce it, `dva stack nosuchsub` scored `"stack"`, found `"stack"`
within edit distance 2, and printed *"Did you mean? dva stack"* — the command that had just
failed. Fixed with a `unknownCommandToken(errMsg) == args[0]` guard, which parses the name cobra
quotes in the message and stays silent when the miss is nested. Shipping the fix without this
would have replaced a wrong exit code with misleading advice.

### Measured (fixture: one entry `realstack`, compose runner on a nonexistent file)

| command | pre-fix | post-fix |
| --- | --- | --- |
| `dva stack status nosuchentry` | exit **0**, 34B table | exit **1**, `ERROR: no such stack entry: nosuchentry` + the defined-entry list |
| `dva stack status realstack` | exit 0, 33B | exit 0, 33B — **byte-identical** |
| `dva stack status` | exit 0, 33B | exit 0, 33B — **byte-identical** |
| `dva stack nosuchsub` | exit **0**, 1222B help | exit **1**, `ERROR: unknown command "nosuchsub" for "dva stack"` |
| `dva stack --help` | exit 0 | exit 0 |
| `dva stack` | exit 0, 1222B | exit 0, 1252B (+`  dva stack [flags]`) |
| `dva stat` (top-level miss) | suggests stack/status/stop | unchanged — the guard's regression control |

### Non-vacuity: one binary per hunk

Four binaries, same fixture. Each hunk fixes its own row and neither fixes the other:

| build | `stack status <typo>` | `stack nosuchsub` |
| --- | --- | --- |
| HEAD, neither hunk | exit 0 | exit 0 |
| only the `status` hunk | exit **1** | exit 0 |
| only the parent hunk | exit 0 | exit **1** |
| both | exit **1** | exit **1** |

The Go tests were probed the same way: against HEAD's `stack.go` both fail, reporting
*"returned <nil>, want an error naming …"* and *"exited 0 over a subcommand that does not exist"*.

### Tests

`internal/cli/stack_exit_code_test.go` (new). The status test reuses TASK-087's script-stack
fixture, so no docker is involved, and carries two accepting rows as controls — asserting on the
`no such stack entry` *message* rather than on `err != nil`, because an accepted name may still
fail via `StatusExitError`, which is a different verdict. The subcommand test goes through
`rootCmd.ExecuteC()` rather than calling `RunE`, since what is under test is cobra's dispatch —
calling the body directly would skip the `ValidateArgs` step that does the work.
`TestUnknownCommandToken` pins the parser, including the empty returns that make the guard
fail safe.

`make test` green under `-race`; `internal/cli` 62.7% → 63.3%.

## Left open

- `suggestCommands` ranges a map without sorting, so the "Did you mean?" list comes out in a
  different order on every run (measured: `stack, status, stop` then `stop, status, stack` for
  the same input). Pre-existing and unrelated to this fix — the same class of defect that
  `lessByOrderName`'s Name tiebreak documents in `lifecycle_helpers.go` — but user-visible
  nondeterminism worth a two-line `sort.Strings`.
- A top-level miss prints the suggestion header **twice** — cobra emits its own
  `Did you mean this?` block and `Execute()` then adds `Did you mean?`. Visible on `dva stat`
  both before and after this change, so it is pre-existing, but the two lists can disagree
  (cobra's uses its own distance rules over registered commands; ours uses
  `config.ReservedCommands()`).
- `suggestCommands` still cannot answer for a nested miss at all. `dva stack statu` now errors
  correctly but offers nothing, where suggesting `status` from the parent's own subcommands
  would be natural. Left out deliberately: it needs a second suggestion path, not a guard.
- `internal/cli/stack.go` and `internal/cli/root.go` are both in [TASK-078](../_archive/078-nine-files-do-not-satisfy-gofmt-and-nothing-checks.md)'s
  nine gofmt-drifting files, unchanged by this task. Every line added here is gofmt-canonical;
  reformatting the rest is 078's call, and 078 is a pending decision.

## Related

- [TASK-087](../_archive/087-unrecognized-stack-args-become-entry-names.md) — fixed the sibling
  subcommands and scoped these out; this file is its *Left open* section promoted to a task,
  and its `validateStackNames` helper is what hunk 1 calls.
