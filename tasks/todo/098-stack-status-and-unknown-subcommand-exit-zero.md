---
id: TASK-098
title: "`dva stack status <typo>` and `dva stack <unknown-subcommand>` still report success over work that did not happen"
type: fix
priority: P3
effort: S
status: todo
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/cli/stack.go — stackStatusCmd's inline nameSet filter; the stack parent command's unknown-subcommand handling"
---

# Task 098: the two exit-0 paths TASK-087 deliberately left behind

[TASK-087](../done/087-unrecognized-stack-args-become-entry-names.md) fixed the orchestrator-backed
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

- [ ] `stack status <typo>` fails | verify: exit must be 1 and stderr must name the entry; print both
- [ ] A real name still works | verify: `dva stack status realstack` exits 0 and prints the row — the control that separates "rejects typos" from "rejects everything"
- [ ] Bare `dva stack status` is unchanged | verify: byte-compare stdout before and after
- [ ] Unknown subcommand fails | verify: `dva stack nosuchsub` exits non-zero; print the exit code and the message
- [ ] `dva stack --help` still exits 0 | verify: print the exit code — an explicit help request is not an error
- [ ] Not vacuous | verify: human — revert each hunk alone and confirm its own row regresses
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-087](../done/087-unrecognized-stack-args-become-entry-names.md) — fixed the sibling
  subcommands and scoped these out; this file is its *Left open* section promoted to a task.
