---
id: TASK-174
title: "`--explain` names the parent's command for a subcommand that runs a script or steps"
type: bug
priority: P3
effort: S
completed-at: 2026-08-07
scope: "dva repo — internal/runner/runner.go Explain"
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
    result: ExplainScript classifyForm; no parent command leak
verification-summary: |
  quality-review pass; re-checked deliverables. ExplainScript classifyForm; no parent command leak. Shared: make test OK; make doc-check OK (mise go 1.26.4).
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
