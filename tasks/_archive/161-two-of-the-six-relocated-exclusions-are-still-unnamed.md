---
id: TASK-161
title: "Two of the six relocated errcheck exclusions are still unnamed"
type: chore
priority: P4
effort: S
completed-at: 2026-08-07
scope: "internal/exec/exec.go"
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
    result: ExecScriptInline best-effort Remove/Close named
verification-summary: |
  quality-review pass; re-checked deliverables. ExecScriptInline best-effort Remove/Close named. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 161

## Result

Both `ExecScriptInline` sites now carry reasons (temp Remove best-effort; Close on Write error
path must not mask WriteString).

**TASK-127 denominator:** 6 sites — after this, both remaining bare ones in exec.go are
commented (6/6 named for that task's set).

**Broader sweep:** `rg '_ = '` non-test under cmd/internal/tools ≈ **55** lines; most are
intentional blank assignments / cleanup, not the six-site errcheck relocation class. Full
population rename is out of scope — no separate task filed unless a future lint requires it.
