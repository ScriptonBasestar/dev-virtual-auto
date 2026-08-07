---
id: TASK-081
title: "`dva show` names plans and interactions but never a stack entry"
type: fix
priority: P4
status: done
effort: S
completed-at: 2026-08-07
scope: "internal/cli/show.go help strings"
---

# Task 081

## Result (reopen close)

Product work (stack section, JSON, tests) already landed. Remaining criterion: remove
hand-written section enumerations from `show` Short/Long. Both now describe omission rules
without listing sections. Verify:

```
sed -n '14,28p' internal/cli/show.go | grep -cE 'environments \(--env\)|stack entries, plans, commands'
# → 0
```
