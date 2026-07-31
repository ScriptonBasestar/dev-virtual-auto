---
id: TASK-101
title: "`default_args` inherits into a subcommand that replaces `command:` outright, so `dva run rails console` executes `console server -p 3000 -b 0.0.0.0`"
type: fix
priority: P3
effort: M
status: todo
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/runner/interaction_tree.go:176,226-228 — mergeInteraction copies parent.DefaultArgs unconditionally; internal/runner/runner.go:124-132 — commandArgs falls back to it when Argv is empty"
---

# Task 101: the child replaces the command but keeps the parent's arguments

## Problem

`mergeInteraction` inherits `DefaultArgs` from the parent (`interaction_tree.go:176`) and lets the
child override it only if the child declares its own (`:226-228`). But `Command` is handled
differently — a child's `command:` **replaces** the parent's outright (`:201-204`), it does not
append to it.

So a subcommand that declares its own `command:` gets a different command with the parent's
arguments still attached. `commandArgs` (`runner.go:124-132`) then hands them straight to the
runner:

```go
if len(cmd.Argv) > 0 { return cmd.Argv }
if cmd.DefaultArgs != "" { return splitCommand(cmd.DefaultArgs) }
```

`Argv` is empty whenever every word of the invocation was consumed as the command name — which is
the normal case for a subcommand — so the fallback fires.

## Measured

Fixture, local runner so nothing is mocked:

```yaml
project_name: defargs
interaction:
  demo:
    command: echo
    default_args: PARENT-DEFAULT-LEAKED
    subcommands:
      sub:
        command: echo
```

| invocation | printed | expected |
| --- | --- | --- |
| `dva run demo` | `PARENT-DEFAULT-LEAKED` | `PARENT-DEFAULT-LEAKED` — correct, it is `demo`'s own |
| `dva run demo sub` | `PARENT-DEFAULT-LEAKED` | nothing — `sub` declared no arguments |

Both exit 0. The child prints the parent's arguments as though they were its own.

In `examples/full-stack.yml` this reaches the shipped example: `rails` carries
`default_args: server -p 3000 -b 0.0.0.0`, so `dva run rails console` executes
`console server -p 3000 -b 0.0.0.0` and `dva run rails db migrate` executes
`db:migrate server -p 3000 -b 0.0.0.0`. One of 16 example files is affected — the only one that
declares both keys.

## Why it was not caught

- `--explain` does not print `DefaultArgs` at all (`runner.go:100-121` prints `Arguments:` only
  when `len(cmd.Argv) > 0`), so the plan for `rails console` looks clean while the exec is not.
  Whatever fixes this should also make the effective arguments visible in the plan, or the same
  defect stays invisible to the next person checking by hand.
- No document states the inheritance rules. `grep -rn 'default_args' --include='*.md' .` outside
  `tasks/` returns **0** hits, and `internal/config/schema.json:407-411` describes the key only as
  "Default arguments for the command" — silent on subcommands. There is no written contract this
  contradicts, which is why it survived; it is still wrong against the one thing that is written
  down, the shipped example.

## Pre-existing, not introduced by TASK-095

Measured at depth 2 against `bin/dva` *before* the TASK-095 merge fix, so both the defect and its
reach into `rails console` predate that change. TASK-095 only lets depth 3 arrive at the same
code. `TestInteractionDefaultArgsInheritIntoSubcommands`
(`internal/runner/interaction_depth_test.go`) characterizes the current behaviour so that fixing
it here fails loudly rather than silently changing what depth-3 commands run.

## Options

- **A — stop inheriting `DefaultArgs` when the child overrides `Command`.** Narrowest reading: the
  arguments belong to the command they were written for. A child that redeclares `command:` starts
  clean; a child that only adds `description:` (a pure container, like `rails db`) still inherits.
- **B — never inherit `DefaultArgs`.** Simpler rule, no conditional. Breaks any config relying on
  a container node passing arguments down, though no shipped example does.
- **C — document the current behaviour and leave it.** Cheapest, but it means `dva run rails
  console` in the project's own example stays wrong, and the argument only holds if some real
  config wants this.

A is the smaller behavioural change and the one that makes the shipped example correct.
**Decision needed.**

## Acceptance criteria

- [ ] A subcommand does not inherit arguments it did not declare | verify: the `defargs` fixture above — `dva run demo sub` must print nothing; print the exact bytes
- [ ] The parent is unaffected | verify: `dva run demo` must still print `PARENT-DEFAULT-LEAKED`
- [ ] The shipped example is correct | verify: `dva run rails console --explain` on `examples/full-stack.yml` must not carry `server -p 3000 -b 0.0.0.0`; print the effective arguments
- [ ] Effective arguments are visible in the plan | verify: `--explain` prints the arguments that will actually be passed, `default_args` included; print the plan
- [ ] Explicit argv still wins | verify: `dva run demo sub EXTRA` must print `EXTRA` and nothing else
- [ ] The characterization test is updated deliberately | verify: `go test ./internal/runner/ -run Interaction` — `TestInteractionDefaultArgsInheritIntoSubcommands` must be rewritten, not deleted; print the tests selected
- [ ] Not vacuous | verify: human — revert the merge hunk and confirm the new assertion fails
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-095](../done/095-third-level-subcommands-never-expand.md) — found while verifying that
  fix; it is what made depth-3 subcommands reach this code, and it left the characterization test
  behind.
