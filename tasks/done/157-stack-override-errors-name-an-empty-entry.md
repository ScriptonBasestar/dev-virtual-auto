---
id: TASK-157
title: "A stack_override merge error names an empty entry, because those entries carry no Name"
type: bug
priority: P4
status: done
effort: S
completed-at: 2026-08-07
scope: "dva repo — internal/config/merge.go"
---

# Task 157

## Result

**Option B:** leave `Name` empty on override entries; fix the message when `Name == ""` to
omit `for stack entry ""`. Outer `[warn] stack_override "api":` already names the key.

Before: `… for stack entry "": "compose" → "helm" …`  
After: `cannot override plugin type: "compose" → "helm" (restricted field)`

Pinned in `TestStackOverrideConflictWarnsOnStderrNotStdout`.
