---
id: TASK-148
title: "dva stack statu offers no suggestion, while dva stat does"
type: bug
priority: P3
status: done
effort: S
created-at: 2026-08-03T13:40:00+09:00
completed-at: 2026-08-07
scope: "dva repo — internal/cli/command_group.go, config, ssh"
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
