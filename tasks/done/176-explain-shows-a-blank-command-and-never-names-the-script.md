---
id: TASK-176
title: "`--explain` prints a blank Command for a script-only interaction and never mentions the script"
type: bug
priority: P3
status: done
effort: S
completed-at: 2026-08-07
scope: "dva repo — internal/runner/runner.go Explain"
---

# Task 176

## Result

- `Command: (script-driven — see Script below)` / `(script_file-driven — see Script File below)`
  matching TASK-146 vocabulary.
- **script:** full body, arrow-indented (same depth as steps — truncating would hide work).
- **script_file:** declared path as written (not absolute).
- JSON agrees: `script` / `script_file` keys when that form wins; `command` empty.
