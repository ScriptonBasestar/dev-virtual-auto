---
id: TASK-101
title: "`default_args` inherits into a subcommand that replaces `command:` outright, so `dva run rails console` executes `console server -p 3000 -b 0.0.0.0`"
type: fix
priority: P3
effort: M
status: done
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/runner/interaction_tree.go:176,226-228 — mergeInteraction copies parent.DefaultArgs unconditionally; internal/runner/runner.go:124-132 — commandArgs falls back to it when Argv is empty"
verified-at: 2026-08-03T13:40:00+09:00
archived-at: 2026-08-03T13:40:00+09:00
verification-summary: |
  Both hunks are present and live in the working tree. interaction_tree.go:306-311 replaces the
  unconditional DefaultArgs override with a switch that zeroes the inherited value when the child
  declares command:/command-as-list; a description-only container still inherits. runner.go:101
  and :125-131 make both plan branches read commandArgs(cmd) — the same function local.go:48,
  docker_compose.go:174 and kubectl.go:49 call — annotated "(from default_args)" when argv is empty.
  Measured with ./bin/dva on the task's own fixture: demo -> PARENT-DEFAULT-LEAKED,
  demo sub -> newline only, demo sub EXTRA -> EXTRA, demo container -> PARENT-DEFAULT-LEAKED.
  On examples/full-stack.yml: rails console / db migrate / db seed print no Arguments line, while
  rails and rails db still do. Non-vacuity confirmed read-only via `go test -overlay` with each
  hunk reverted in a scratch copy — the merge revert reproduces the exact failure text quoted in
  the task, the explain revert fails the default_args row. schema.json:408 now states the
  inheritance rule and the container exception. Re-measured blast radius: default_args appears in
  exactly 1 of 101 YAML files (examples/full-stack.yml:161); the --json "arguments" key has one
  producer and no consumer anywhere in the repo.
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
**Decision: A.** B was rejected because it would also break a container node passing arguments to
its children — a shape nothing ships today but which the config language plainly allows, and
removing it is a larger change than this defect justifies. C was rejected because it means writing
down that the project's own example is wrong.

## Acceptance criteria

- [x] A subcommand does not inherit arguments it did not declare | verify: the `defargs` fixture above — `dva run demo sub` must print nothing; print the exact bytes
- [x] The parent is unaffected | verify: `dva run demo` must still print `PARENT-DEFAULT-LEAKED`
- [x] The shipped example is correct | verify: `dva run rails console --explain` on `examples/full-stack.yml` must not carry `server -p 3000 -b 0.0.0.0`; print the effective arguments
- [x] Effective arguments are visible in the plan | verify: `--explain` prints the arguments that will actually be passed, `default_args` included; print the plan
- [x] Explicit argv still wins | verify: `dva run demo sub EXTRA` must print `EXTRA` and nothing else
- [x] The characterization test is updated deliberately | verify: `go test ./internal/runner/ -run Interaction` — `TestInteractionDefaultArgsInheritIntoSubcommands` must be rewritten, not deleted; print the tests selected
- [x] Not vacuous | verify: human — revert the merge hunk and confirm the new assertion fails
- [x] Full suite passes | verify: `make test`

## Resolution

Option A, in two hunks — one fixes what runs, the other fixes what the user is told will run.

**1. `mergeInteraction` stops inheriting across a command replacement**
(`internal/runner/interaction_tree.go`). The unconditional `if child.DefaultArgs != ""` override
became a switch: the child's own arguments win, otherwise a child that declares `command:` or
`command:` as a list starts with none. A child that declares neither — a pure container like
`rails db` — still inherits both the command and its arguments, which is what lets a group share
one argument list.

**2. `Explain` reports the arguments the runners will actually receive**
(`internal/runner/runner.go`). Both the text plan and the `--json` plan read `commandArgs(cmd)`,
the same function `local.go`, `docker_compose.go` and `kubectl.go` call, instead of `cmd.Argv`.
Arguments that came from `default_args` rather than from the invocation are marked
`(from default_args)`, since those are exactly the ones the user did not type.

**3. The rule is now written down** (`internal/config/schema.json`). Per the survey below, that
description was the only account of `default_args` anywhere in the repo and it said nothing about
subcommands; it now states the inheritance rule and the container exception.

### Measured (fixture from *Measured* above, plus a container control)

| invocation | HEAD | fixed |
| --- | --- | --- |
| `dva run demo` | `PARENT-DEFAULT-LEAKED` | `PARENT-DEFAULT-LEAKED` — its own, untouched |
| `dva run demo sub` | `PARENT-DEFAULT-LEAKED` | *(nothing)* |
| `dva run demo sub EXTRA` | `EXTRA` | `EXTRA` — explicit argv still wins |
| `dva run demo container` | `PARENT-DEFAULT-LEAKED` | `PARENT-DEFAULT-LEAKED` — declares no command, still inherits |

On `examples/full-stack.yml`, rendered with a binary carrying only hunk 2 so HEAD's *effective*
arguments are visible rather than inferred:

| `--explain` | HEAD (hunk 2 only) | fixed |
| --- | --- | --- |
| `rails console` | `Arguments: server -p 3000 -b 0.0.0.0  (from default_args)` | *(no Arguments line)* |
| `rails db migrate` | same | *(no Arguments line)* |
| `rails db seed` | same | *(no Arguments line)* |
| `rails db` | same | unchanged — the container control |

`rails db seed` was found by the blast-radius survey, not by this task's original measurement,
which had recorded only `console` and `db migrate`.

### Non-vacuity: one binary per hunk

| build | `dva run demo sub` executes | `rails console --explain` says |
| --- | --- | --- |
| HEAD, neither hunk | `PARENT-DEFAULT-LEAKED` | *(no Arguments line)* |
| only the merge hunk | *(nothing)* | *(no Arguments line)* |
| only the explain hunk | `PARENT-DEFAULT-LEAKED` | `Arguments: server -p 3000 -b 0.0.0.0  (from default_args)` |
| both | *(nothing)* | *(no Arguments line)* |

Row 3 is the point of hunk 2: the diagnostic alone turns a silent defect into a visible one. Row 2
is why it is not sufficient on its own — with the exec fixed but the plan still reading `Argv`, the
next regression of this kind would be just as invisible as this one was.

At the test level, the rewritten assertions compiled against HEAD's `interaction_tree.go` fail on
exactly the two command-declaring nodes and pass on both controls:

```
--- FAIL: TestInteractionDefaultArgsStopAtACommandOverride
    [rails console]    DefaultArgs = "server -p 3000 -b 0.0.0.0", want ""
    [rails db migrate] DefaultArgs = "server -p 3000 -b 0.0.0.0", want ""
```

### Tests

- `TestInteractionDefaultArgsInheritIntoSubcommands` → rewritten as
  `TestInteractionDefaultArgsStopAtACommandOverride`, same shipped fixture, four rows: the parent
  and the container keep the arguments, the two command-declaring children lose them. The
  container row is what stops option B from passing this test.
- `internal/runner/runner_explain_test.go` (new) — `Explain` had **0% coverage**, which is the
  other half of why this survived. Four rows including argv-beats-default_args and a negative
  control that no Arguments line is printed when there is nothing to pass. Reuses the existing
  `captureStdout` from `inert_step_test.go` rather than adding a second one.

`make test` green; `internal/runner` coverage 51.9%.

### Blast radius (surveyed repo-wide)

`default_args` is declared in exactly **one** of 80 YAML files — `examples/full-stack.yml:161` —
and that node is the one with `subcommands:` whose children declare `command:`. Every other config
in the repo, shipped example or fixture, is unaffected. `commandArgs` is the single consumer, read
by all three runners.

## Left open

- A child that declares `script:`, `script_file:` or `steps:` rather than `command:` still inherits
  the parent's `default_args`. Left alone deliberately: those paths do not read `commandArgs`, so
  nothing is passed anywhere today, and widening the condition would be a change with no
  observable behaviour to verify against.
- `docs/30-config-merge-semantics.md` documents interaction merging *across config layers*
  (`internal/config/merge.go`) and not parent-node-to-child-subcommand inheritance inside one tree
  (`mergeInteraction`). The two are different code paths with different rules; only the schema
  description now states the second one. A short section pairing them would be worth it.
- The `--json` plan's `arguments` key changed meaning with hunk 2: it is now the effective
  arguments, not the literal invocation. No test or document pinned the old meaning, and no
  consumer in the repo reads it, but it is a visible change to a machine-readable surface.

## Related

- [TASK-095](../_archive/095-third-level-subcommands-never-expand.md) — found while verifying that
  fix; it is what made depth-3 subcommands reach this code, and it left the characterization test
  behind.
