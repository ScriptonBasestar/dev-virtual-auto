---
id: TASK-157
title: "A stack_override merge error names an empty entry, because those entries carry no Name"
type: bug
priority: P4
effort: S
completed-at: 2026-08-07
scope: "dva repo — internal/config/merge.go"
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
    result: empty Name no stack entry ""; override conflict test
verification-summary: |
  quality-review pass; re-checked deliverables. empty Name no stack entry ""; override conflict test. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 157

## Result

**Option B:** leave `Name` empty on override entries; fix the message when `Name == ""` to
omit `for stack entry ""`. Outer `[warn] stack_override "api":` already names the key.

Before: `… for stack entry "": "compose" → "helm" …`  
After: `cannot override plugin type: "compose" → "helm" (restricted field)`

Pinned in `TestStackOverrideConflictWarnsOnStderrNotStdout`.
