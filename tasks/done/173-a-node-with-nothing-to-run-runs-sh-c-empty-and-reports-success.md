---
id: TASK-173
title: "an interaction with nothing to run executes `sh -c \"\"` and reports success"
type: bug
priority: P3
status: done
effort: S
created-at: 2026-08-03T20:40:00+09:00
completed-at: 2026-08-07
scope: "dva repo — internal/runner, internal/cli/run.go"
---

# Task 173: Decide what `dva run` does when the resolved node has nothing to execute

## Acceptance criteria

- [x] Decide where the check belongs; record why.
- [x] `dva run` fails naming the node and what to add (exit ≠ 0).
- [x] `--explain` succeeds and says nothing to run.
- [x] Inherited execution targets still run.
- [x] Compatibility surface measured (including zero).
- [x] `make test` exits 0.

## Result

### Decision: CLI resolve step, predicate on `ResolvedCommand`

The guard sits in `run.go` **after** `InteractionTree.Find` and **before** `NewRunner.Execute`,
not inside each runner. Reasons:

1. **Inheritance is already applied** on `ResolvedCommand` — a second walk of parents at the
   CLI would re-implement merge rules.
2. **One gate for all runners** — local/compose/kubectl all reached `sh -c ""` / empty exec
   through the same empty-command path; three runner-side guards would drift.
3. **`--explain` stays diagnostic** — it runs before the guard and prints
   `Command: (nothing to run — …)` without failing.

`ResolvedCommand.HasExecutionTarget` is the post-merge twin of
`InteractionCommand.hasExecutionTarget` (TASK-165). Drift is controlled by field parity
comments (hooks intentionally omitted: not executed on the runner path).

### Before / after

```
# description-only leaf
dva run lone          # before: exit 0, silent; after: exit 1, names "lone"
dva run lone --explain
# after:
# Command: (nothing to run — add command:, script:, script_file:, steps:, service:, pod:, or default_args:)
# exit 0
```

### Compatibility surface

Scanned `examples/`, `workflows/`, `agent-mesh-flows/`, `.github/`, `Makefile` for scripted
`dva run` of description-only interactions: **0** (no in-repo automation depends on exit 0
for empty leaves).

### Verification

```
go test ./internal/runner/ ./internal/cli/ -count=1
make test
```
