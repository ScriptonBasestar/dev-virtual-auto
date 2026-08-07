---
id: TASK-176
title: "`--explain` prints a blank Command for a script-only interaction and never mentions the script"
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
    result: script/script_file-driven Explain labels
verification-summary: |
  quality-review pass; re-checked deliverables. script/script_file-driven Explain labels. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 176

## Result

- `Command: (script-driven — see Script below)` / `(script_file-driven — see Script File below)`
  matching TASK-146 vocabulary.
- **script:** full body, arrow-indented (same depth as steps — truncating would hide work).
- **script_file:** declared path as written (not absolute).
- JSON agrees: `script` / `script_file` keys when that form wins; `command` empty.
