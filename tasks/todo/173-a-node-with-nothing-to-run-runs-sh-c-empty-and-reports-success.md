---
id: TASK-173
title: "an interaction with nothing to run executes `sh -c \"\"` and reports success"
type: bug
priority: P3
status: todo
effort: S
created-at: 2026-08-03T20:40:00+09:00
source: "TASK-165 criterion 5 — the runtime half of the same defect, deliberately deferred"
depends-on: [TASK-165]
scope: "dva repo — internal/runner (ResolvedCommand dispatch), internal/exec/exec.go buildCommandLine"
---

# Task 173: Decide what `dva run` does when the resolved node has nothing to execute

## Problem

TASK-165 made `dva config validate` report a node with no execution target and no
subcommands. It deliberately left the runtime alone. Running one still succeeds:

```yaml
interaction:
  grp:
    subcommands:
      leaf: {description: does nothing at all}
  lone:
    description: a top-level leaf with nothing to run
```

| invocation | output | exit |
|---|---|---|
| `dva config validate` | warns on `grp`, `grp.leaf` **and** `lone` (TASK-165) | 0 |
| `dva run grp leaf` | *nothing* | **0** |
| `dva run lone` | *nothing* | **0** |

The mechanism is `exec.buildCommandLine`: with an empty command and no args it returns
`["sh", "-c", ""]` under the default shell mode, and `sh -c ""` exits 0 having done nothing.
So the process really is replaced and really does succeed — this is not an error being
swallowed, it is a successful execution of nothing.

That is the family closed by
[TASK-118](../_archive/118-a-health-check-that-never-passes-is-still-exit-0.md) and, on the
reporting side, by TASK-158. A caller that checks `$?` — which is all a script can check —
cannot tell this from a run that worked.

## Why TASK-165 did not do it

Two reasons, both recorded there:

1. It is an observable exit-code change on `dva run`, and its compatibility surface is not
   the 19-config `examples/` corpus that task measured but every caller's scripts. That is a
   different measurement, of the kind TASK-171 performed for `dva clean`.
2. Doing it right needs a "nothing to run" predicate on `runner.ResolvedCommand`, which is a
   second copy of `config.hasExecutionTarget` on a different type. Two predicates that must
   agree forever is the drift both TASK-128 and TASK-146 warn about, and it is a design
   decision, not a one-line guard.

## Acceptance criteria

- [ ] Decide where the check belongs: the runner (post-merge `ResolvedCommand`, which already
      has the inherited view) or the CLI resolve step. Record why, naming what stops it
      drifting from `config.hasExecutionTarget`.
      Verify: `human — the decision and its reasoning are in the Result section`
- [ ] `dva run` on such a node fails with a message naming the node's path and what to add.
      Verify: `d=$(mktemp -d) && printf 'version: "0.1.44"\ninteraction:\n  lone:\n    description: x\n' > $d/dva.yml && (cd $d && dva run lone); test $? -ne 0`
- [ ] `--explain` on the same node still succeeds and says the node has nothing to run —
      `--explain` is the tool for diagnosing this, so it must not become the thing that fails.
      Verify: `human — before/after output in the Result section`
- [ ] A node that inherits an execution target from an ancestor still runs.
      Verify: `go test ./internal/runner/ -count=1`
- [ ] Measure the compatibility surface before changing the exit code: does anything in
      `examples/`, `Makefile`, `workflows/`, `agent-mesh-flows/`, `.github/` or any shell
      script run an interaction that resolves to nothing? Report the count including zero.
- [ ] `make test` exits 0.

## Notes

`shell: false` behaves differently and is worth checking: `buildCommandLine` returns the args
alone, so an empty command yields an empty argv and `exec.LookPath` is called on nothing.
Measure both modes before choosing the predicate's shape.

## Related

- [TASK-165](../done/165-a-leaf-interaction-with-nothing-to-run-draws-no-warning-and-exits-0.md)
  — the validator half, and the source of the measurements above.
- [TASK-146](146-dry-run-shows-a-blank-command-for-a-steps-only-interaction.md) — a node that
  *does* have a target which `Explain` cannot see. A guard written against `Explain`'s view
  rather than the resolved command would report that working node as empty.
