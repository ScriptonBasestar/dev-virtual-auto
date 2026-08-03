---
id: TASK-174
title: "`--explain` names the parent's command for a subcommand that runs a script or steps"
type: bug
priority: P3
status: todo
effort: S
created-at: 2026-08-03T21:05:00+09:00
source: "TASK-149 — found while pinning the --json arguments key; the sibling field the same fix left wrong"
depends-on: [TASK-149]
scope: "dva repo — internal/runner/interaction_tree.go mergeInteraction, runner.go Explain"
---

# Task 174: A plan that names a command the subcommand will not run

## Problem

`mergeInteraction` copies the parent's `Command` onto every child, and only a child's own
`command:` overwrites it. A child that declares `script:`, `script_file:` or `steps:` therefore
keeps `merged.Command` pointing at the parent — and `Explain` reports it.

Measured on `dc0f2bf`:

```yaml
interaction:
  rails:
    command: "bundle exec rails"
    default_args: "-e development"
    subcommands:
      scripted:
        script: |
          echo "scripted child ran"
```

| | reports / does |
|---|---|
| `dva run rails scripted --explain --json` | `"command": "bundle exec rails"` |
| `dva run rails scripted` | prints `scripted child ran` |

The plan names a command that never runs. [TASK-149](../done/149-default-args-inheritance-is-documented-only-in-the-schema.md)
fixed the neighbouring `arguments` key — that child now correctly reports no arguments — but
deliberately stopped there, so the plan is currently half-right: no arguments, wrong command.

This is [TASK-146](146-dry-run-shows-a-blank-command-for-a-steps-only-interaction.md)'s
mirror image. There a top-level steps-only interaction had *no* command and `Explain` printed a
blank line; the fix taught the text branch to say "step-driven" instead. Here the node is a
*child*, so it has a non-empty inherited `Command` and never reaches that branch — the TASK-146
wording is bypassed by the very inheritance that causes the problem.

## Acceptance criteria

- [ ] Decide where to cut it: stop `mergeInteraction` inheriting `Command` when the child
      declares `script:`/`script_file:`/`steps:`, or keep the field and teach `Explain` to
      report what will actually run. Say which, and what the other one would have broken.
      Verify: `human — the decision and its reasoning are in the Result section`
- [ ] Both `Explain` branches agree — the text plan and `--json` must not disagree about what
      runs, since TASK-146 changed only the text branch and this is where that shows.
      Verify: `go test ./internal/runner/ -count=1`
- [ ] The TASK-146 wording is reached for a *child* steps-only node, not only a top-level one.
- [ ] A container child (`description:` only) still inherits the parent's command — that is the
      rule TASK-101 established and it must survive.
      Verify: `go test ./internal/runner/ -run ExplainJSON -count=1`
- [ ] Corpus: run `dva ls --json` over `examples/` and report how many resolved commands change
      their reported `command`, including zero.
- [ ] `make test` exits 0.

## Notes

If the chosen cut is "stop inheriting Command", check `DetectRunnerType` first: it reads
`Service` and `Pod`, not `Command`, so runner selection should be unaffected — but
`docker_compose.go:composeArguments` gates its whole command block on `cmd != ""`, so an
emptied `Command` changes the compose argv for such a child. Measure that before choosing.

## Related

- [TASK-175](175-kubectl-runner-drops-script-and-script-file-and-runs-the-inherited-command.md)
  — the same inherited-`Command` field, but consumed by the kubectl runner at execution rather
  than at report time, which makes it a runtime defect rather than a reporting one.
