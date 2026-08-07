---
id: TASK-174
title: "`--explain` names the parent's command for a subcommand that runs a script or steps"
type: bug
priority: P3
status: done
effort: S
completed-at: 2026-08-07
scope: "dva repo — internal/runner/runner.go Explain"
---

# Task 174

## Result

**Decision: teach Explain (text + JSON), do not stop Command inheritance in merge.**

Stopping inheritance when the child has script/steps would empty `Command` for compose argv
paths that still gate on it, and would re-implement form precedence next to `classifyForm`.
Explain already uses `classifyForm` for the text switch; JSON now uses the same form pick so
`command` is only set for `formCommand`. Scripted/step children report empty `command` plus
`script` / `script_file` / `steps`. Description-only children still inherit and show the parent
command.

Corpus: examples do not assert `--explain` command fields in CI; behavioural tests cover the
shapes. `make test` exit 0.
