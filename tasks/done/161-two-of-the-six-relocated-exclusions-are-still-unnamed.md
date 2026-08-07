---
id: TASK-161
title: "Two of the six relocated errcheck exclusions are still unnamed"
type: chore
priority: P4
status: done
effort: S
completed-at: 2026-08-07
scope: "internal/exec/exec.go"
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
