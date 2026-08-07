---
id: TASK-148
title: "dva stack statu offers no suggestion, while dva stat does"
type: bug
priority: P3
effort: S
created-at: 2026-08-03T13:40:00+09:00
completed-at: 2026-08-07
scope: "dva repo — internal/cli/command_group.go, config, ssh"
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
    result: setGroupParentBehavior; SuggestionsFor on groups
verification-summary: |
  quality-review pass; re-checked deliverables. setGroupParentBehavior; SuggestionsFor on groups. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 148: Suggest at the subcommand level too

## Result

`stack` is gone from the CLI surface. The same defect lived on every pure group parent:
leftover args showed help and exited 0 with no suggestions.

**Group parents found and fixed: 2** — `config`, `ssh` (the only commands with children and
no prior RunE). Wired via `setGroupParentBehavior` from `Execute()` after all inits register
children. Suggestions come from **`cmd.SuggestionsFor`** (same source as top-level cobra).

```
$ dva ssh statu     # exit 1, suggests status
$ dva config migrat # exit 1, suggests migrate
```

Exit code stays 1. One "Did you mean this?" block. Tests in `command_group_test.go`.
