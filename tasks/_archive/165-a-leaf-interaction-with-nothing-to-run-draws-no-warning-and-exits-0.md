---
id: TASK-165
title: "A leaf interaction with no execution target draws no warning and runs to a silent exit 0"
type: bug
priority: P3
effort: S
created-at: 2026-08-03T15:40:00+09:00
source: "TASK-128 finalize verification — pre-existing coverage boundary, not introduced there"
scope: "dva repo — internal/config/validate_warnings.go warnUnreachableCommands, hasExecutionTarget at :363"
status: done
quality-review: pass
quality-reviewed-at: 2026-08-07T18:05:08+09:00
verified-at: 2026-08-07T18:05:08+09:00
archived-at: 2026-08-07T18:05:08+09:00
quality-review-evidence: |
  - kind: test
    command-or-step: make test && make doc-check (mise go 1.26.4)
    result: exit 0; shared suite green
  - kind: recheck
    command-or-step: acceptance criteria re-observed
    result: warnUnreachableCommands dead leaf; tests PASS
verification-summary: |
  quality-review pass; re-checked deliverables. warnUnreachableCommands dead leaf; tests PASS. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 165: Warn on a leaf that cannot execute anything

## Problem

`warnUnreachableCommands` fires only on nodes that **have subcommands** — the "has subcommands
but is not directly callable" case. A leaf with neither subcommands nor an execution target
falls outside the check entirely, so nothing reports it, and running it succeeds having done
nothing.

Measured at `1695f9d`:

```yaml
interaction:
  grp:
    description: a group
    subcommands:
      leaf:
        description: does nothing at all
```

| invocation | output | exit |
|---|---|---|
| `dva config validate` | one warning, about `grp` only — `interaction.grp: has subcommands but is not directly callable` — then `✅ dva.yml is valid` | 0 |
| `dva run grp leaf` | *nothing* | **0** |

`leaf` is the node that can never do anything, and it is the one neither the validator nor the
runtime mentions. The parent, which at least routes to its children, is the one that gets the
warning.

This predates [TASK-128](../_archive/128-the-recursion-was-right-the-nodes-it-walked-were-not.md):
`hasExecutionTarget` (`validate_warnings.go:363`) is a byte-for-byte extraction of the pre-fix
`isCallable` condition, so 128 moved the predicate without changing which nodes it is applied
to. Filed against the boundary, not against that task.

## Acceptance criteria

- [x] A leaf with no execution target draws a semantic warning naming its full path
      (`interaction.grp.leaf`), in the same form the existing unreachable warning uses.
      *Met, with the path spelled `interaction.grp.subcommands.leaf` — see "Path format" below.*
- [x] Inheritance is respected: a leaf that inherits a runnable `command` from an ancestor must
      **not** warn. `hasExecutionTarget` deliberately ignores inheritance — whatever calls it
      here has to account for that, or the fix turns valid configs noisy.
- [x] Prove the gate fails on reverted code | verify: revert the new condition, run the new
      test, paste the failure.
- [x] Re-run the shipped corpus and report warnings added per config, including the zeros.
      TASK-128's standard was a per-config diff showing what changed in both directions; the
      denominator is 19 YAML files under `examples/`.
- [x] Decide and record whether `dva run` on such a node should also stop being exit 0. A
      command that runs nothing and reports success is the family closed in
      [TASK-118](../_archive/118-a-health-check-that-never-passes-is-still-exit-0.md); if it
      stays 0, say why. *Decided: stays 0 here, deferred to
      [TASK-173](../todo/173-a-node-with-nothing-to-run-runs-sh-c-empty-and-reports-success.md)
      — reasoning below.*
- [x] `make test` and `make lint` exit 0. *`make lint` blocked by pre-existing toolchain drift;
      its components run clean — see Gates.*

## Related

- [TASK-146](146-dry-run-shows-a-blank-command-for-a-steps-only-interaction.md) — same visible
  blank, opposite cause: there the node *has* a target (`steps:`) and Explain cannot see it.
  Here the node has none. A fix to either must not paper over the other.
- [TASK-162](162-a-command-inherited-through-a-merge-key-is-dropped-and-the-run-exits-0.md) —
  the third way to reach a silent exit-0 run, from the parser side.

## Result

`warnUnreachableCommands` now reports **two** shapes instead of one. Both share a single guard —
"this node cannot execute anything, itself or by inheritance" — and differ only in the advice,
chosen by whether the node has children:

| node | shape | message |
|---|---|---|
| parent, no target | pre-existing | `has subcommands but is not directly callable; add an execution target or remove subcommands` |
| leaf, no target | **new** | `has no execution target and no subcommands, so running it does nothing; add a command, script, steps or service — or remove the entry` |

Structurally this inverted the function: the old body early-returned on `len(Subcommands) == 0`
(the exact line that made the leaf case unreachable) and then tested callability. The new body
tests callability first and returns if the node *can* run; only then does the child count pick
which message to emit. The leaf case is not an added branch so much as the removal of a filter.

### Reproduction, before and after

Fixture (`grp`/`grp.leaf` from the Problem section, plus a top-level leaf `lone` and a `rails`
tree whose `db` node inherits `bundle exec rails` from its parent):

```
$ before/dva config validate
[warn] semantic: interaction.grp: has subcommands but is not directly callable; …
✅ dva.yml is valid                                                        exit=0

$ after/dva config validate
[warn] semantic: interaction.grp.subcommands.leaf: has no execution target and no subcommands, …
[warn] semantic: interaction.grp: has subcommands but is not directly callable; …
[warn] semantic: interaction.lone: has no execution target and no subcommands, …
✅ dva.yml is valid                                                        exit=0
```

`rails db` and `rails db migrate` stay silent in both — they inherit a runnable command, which is
criterion 2's requirement and the reason the guard consults `inherited.callable` (from
`eachInteractionNode`'s `inheritedExec`) and not just the raw node.

### Path format — a deviation from criterion 1's parenthetical, stated rather than glossed

Criterion 1 asks for "its full path (`interaction.grp.leaf`), in the same form the existing
unreachable warning uses". Those two clauses disagree. The walker builds paths as YAML key paths
(`validate_warnings.go:379`: `path+".subcommands."+subName`), and every sibling warning in the
file emits that form — `%s.subcommands.%s` at lines 442, 804 and 809. So the shipped text is
`interaction.grp.subcommands.leaf`.

I followed the second clause. The parenthetical is invocation syntax (`dva run grp leaf`); the
key path is what the reader greps for in `dva.yml`, which is the document a config warning is
about. Emitting invocation syntax *here alone* would put a third path format in one output
stream. Converting all of them is a separate, wider change, not a side effect of this fix.

### A second bug found while building the inheritance fixture

`hasExecutionTarget` did not count `DefaultArgs`. It should, and this is not theoretical —
measured, not reasoned:

```
$ cat dva.yml
interaction:
  argsonly:
    default_args: "echo reached-via-default-args"
$ dva run argsonly
reached-via-default-args                                                   exit=0
```

The mechanism is `exec.buildCommandLine`: in shell mode (the default) it joins `cmd + " " + args`
and hands the result to `sh -c`, so when `cmd` is empty the args *become* the entire shell line.
The predicate therefore disagreed with the runtime in the false-positive direction — it would
have told an author their node cannot run while `dva run` was running it.

That was harmless while the predicate only ever gated the parent-shape warning (a node with
subcommands and only `default_args` is a shape nobody writes). The leaf warning makes it reachable
for any node, so fixing it is load-bearing for criterion 2's "or the fix turns valid configs
noisy" — same category of risk as inheritance, different mechanism. The `args_only` test case is
the standing guard.

### Corpus, per TASK-128's standard

Both binaries (the "before" one built from `git archive HEAD` into an isolated tree) run
`config validate` against all 19 YAML files under `examples/`, each copied into its own directory
alongside its siblings so relative module paths still resolve:

**19 files compared, 19 at delta=0, 0 with any change in either direction.** Warning counts are
unchanged file by file (`applications.yml` 2→2, `basic.yml` 1→1, … `stack-source.yml` 2→2). The
repo's own dogfood `dva.yml` is likewise 0→0, byte-identical.

An all-zeros result is exactly what a broken harness produces, so the same harness was run against
the t165 fixture as a positive control: **before=1 after=3, delta=+2**. The comparison can detect a
change; the corpus genuinely has none. That is the expected outcome — a shipped example containing
a command that does nothing would itself be the bug.

### Falsification

Each revert was diffed against a saved copy of the original file to confirm it was a *complete*
revert before running anything — an incomplete one produces a passing test that looks like a weak
falsification rather than a failed one (the lesson recorded in TASK-158's F3).

| # | revert | result |
|---|---|---|
| F1 | restore the `if len(Subcommands) == 0 { return }` early return | `TestWarnUnreachableCommands` fails — 2 warnings expected, 1 produced; the `dead_leaf` substring is absent |
| F2 | drop `\|\| c.DefaultArgs != ""` from `hasExecutionTarget` | fails on the *negative* assertion — `args_only` warns, 3 warnings where 2 were expected |
| F3 | drop the `\|\| inherited.callable` disjunct | fails **two** tests at once: `TestWarnUnreachableCommands` (`inherits_target` warns) **and** `TestInteractionWarningsRecurseIntoNestedSubcommands` |

F3 is the strongest of the three. Breaking a second, independently-motivated test means removing
that disjunct reintroduces precisely the inheritance false-positive TASK-128 fixed for the parent
shape — the guard is not decorative, and the leaf shape inherits its necessity.

### Criterion 5 — `dva run` stays exit 0, and why

It stays 0 **in this task**, and the runtime half is filed as
[TASK-173](../todo/173-a-node-with-nothing-to-run-runs-sh-c-empty-and-reports-success.md).

The defect is real and is the TASK-118 family: `exec.buildCommandLine` with an empty command and
no args returns `["sh", "-c", ""]`, and `sh -c ""` genuinely succeeds having done nothing. Nothing
is being swallowed — a caller checking `$?` simply cannot distinguish it from a run that worked.

Deferred rather than done here for two reasons:

1. **Different compatibility surface, so a different measurement.** The 19-config corpus above
   bounds a validator change. An exit-code change on `dva run` is bounded by every caller's
   scripts, which is the kind of survey TASK-171 did for `dva clean` and which this task did not
   perform. Shipping the exit-code change on this task's evidence would be claiming a measurement
   that was never taken.
2. **It needs a design decision, not a guard.** The check would live on
   `runner.ResolvedCommand`, a different type from `config.Command`, so it means a second
   "nothing to run" predicate that must agree with `hasExecutionTarget` forever. That two-copy
   drift is what TASK-128 and TASK-146 both warn about, and the `DefaultArgs` gap found above is a
   live demonstration of a single predicate already drifting from the runtime it describes.

Meanwhile CI is not left without a gate: `validate --strict` (`internal/cli/validate.go:173`)
already fails on any non-empty `semanticWarnings`, so as of this change a config containing a dead
leaf fails strict validation with no further work. The hole that remains is narrow — someone who
runs the command without validating first.

### Gates

| gate | result |
|---|---|
| `make test` | pass (`-race -cover`, all packages ok; `internal/config` 68.3%) |
| `gofmt -l internal/ cmd/ tools/` | clean |
| `go vet ./...` | exit 0 |
| `golangci-lint run ./internal/config/...` | 0 issues |
| `make lint` | **blocked, pre-existing** |

`make lint` fails inside `mise exec` on a `tools/doccheck` typecheck error caused by a go1.26.4
mise GOROOT against a go1.26.5 tool build — the drift already recorded before this task, and
`GOTOOLCHAIN=local make lint` fails the same way. Invoking `golangci-lint run ./...` directly
bypasses it and reports 16 issues (10 errcheck, 5 modernize, 1 staticcheck), **none** in
`validate_warnings.go` or `validate_warnings_test.go`. Running the identical command against the
unmodified pre-change tree returns the same 16 — this change adds no lint findings. The
deterministic components of the target (vet, gofmt) are green above.

### Changed files

- `internal/config/validate_warnings.go` — `hasExecutionTarget` counts `DefaultArgs`;
  `warnUnreachableCommands` inverted to report both unreachable shapes.
- `internal/config/validate_warnings_test.go` — `TestWarnUnreachableCommands` rewritten to
  substring assertions over a fixture carrying `dead_leaf` (positive) plus `inherits_target` and
  `args_only` (negative controls).
