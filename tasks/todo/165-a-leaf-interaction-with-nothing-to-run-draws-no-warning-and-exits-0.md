---
id: TASK-165
title: "A leaf interaction with no execution target draws no warning and runs to a silent exit 0"
type: bug
priority: P3
status: todo
effort: S
created-at: 2026-08-03T15:40:00+09:00
source: "TASK-128 finalize verification — pre-existing coverage boundary, not introduced there"
scope: "dva repo — internal/config/validate_warnings.go warnUnreachableCommands, hasExecutionTarget at :363"
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

- [ ] A leaf with no execution target draws a semantic warning naming its full path
      (`interaction.grp.leaf`), in the same form the existing unreachable warning uses.
- [ ] Inheritance is respected: a leaf that inherits a runnable `command` from an ancestor must
      **not** warn. `hasExecutionTarget` deliberately ignores inheritance — whatever calls it
      here has to account for that, or the fix turns valid configs noisy.
- [ ] Prove the gate fails on reverted code | verify: revert the new condition, run the new
      test, paste the failure.
- [ ] Re-run the shipped corpus and report warnings added per config, including the zeros.
      TASK-128's standard was a per-config diff showing what changed in both directions; the
      denominator is 19 YAML files under `examples/`.
- [ ] Decide and record whether `dva run` on such a node should also stop being exit 0. A
      command that runs nothing and reports success is the family closed in
      [TASK-118](../_archive/118-a-health-check-that-never-passes-is-still-exit-0.md); if it
      stays 0, say why.
- [ ] `make test` and `make lint` exit 0.

## Related

- [TASK-146](146-dry-run-shows-a-blank-command-for-a-steps-only-interaction.md) — same visible
  blank, opposite cause: there the node *has* a target (`steps:`) and Explain cannot see it.
  Here the node has none. A fix to either must not paper over the other.
- [TASK-162](162-a-command-inherited-through-a-merge-key-is-dropped-and-the-run-exits-0.md) —
  the third way to reach a silent exit-0 run, from the parser side.
