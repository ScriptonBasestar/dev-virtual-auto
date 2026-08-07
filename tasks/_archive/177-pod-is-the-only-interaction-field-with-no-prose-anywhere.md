---
id: TASK-177
title: "`pod:` appears in no document"
type: chore
priority: P3
effort: S
completed-at: 2026-08-07
scope: "USAGE.md"
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
    result: USAGE service:/pod: prose
verification-summary: |
  quality-review pass; re-checked deliverables. USAGE service:/pod: prose. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 177

## Result

USAGE.md interaction section: `service:` / `pod:` / local table; kubectl forms
(command, list, steps, script, script_file); shebang not honoured in-pod; compose script
falls back to host. `make doc-check` exit 0.
