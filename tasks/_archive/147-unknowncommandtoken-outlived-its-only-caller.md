---
id: TASK-147
title: "unknownCommandToken has no production caller and a doc comment addressing callers that do not exist"
type: chore
priority: P3
effort: S
created-at: 2026-08-03T13:40:00+09:00
completed-at: 2026-08-07
scope: "dva repo — internal/cli/root.go"
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
    result: unknownCommandToken deleted; rg 0 go hits
verification-summary: |
  quality-review pass; re-checked deliverables. unknownCommandToken deleted; rg 0 go hits. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 147: Retire or re-justify `unknownCommandToken`

## Acceptance criteria

- [x] Decide: delete it (with its test), or name the production caller that justifies
      keeping it. Print `grep -rn unknownCommandToken --include='*.go' .` before and after.
- [x] If deleted, remaining exit-code coverage still stands (token parser was a means).
- [x] `make test` exits 0.

## Result

**Deleted.** No production caller (TASK-108 removed `suggestCommands`). The only test file
named in the task (`stack_exit_code_test.go`) was already gone.

```
# before (source)
internal/cli/root.go  definition + doc

# after
(0 lines in *.go)
```

`levenshtein` retained with its justification comment; `unknownCommandToken` had none.
