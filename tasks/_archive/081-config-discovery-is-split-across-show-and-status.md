---
id: TASK-081
title: "`dva show` names plans and interactions but never a stack entry"
type: fix
priority: P4
effort: S
completed-at: 2026-08-07
scope: "internal/cli/show.go help strings"
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
    result: show.go no section enum; stackEntryView present
verification-summary: |
  quality-review pass; re-checked deliverables. show.go no section enum; stackEntryView present. Shared: make test OK; make doc-check OK (mise go 1.26.4).
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
