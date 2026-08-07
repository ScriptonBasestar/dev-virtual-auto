---
id: TASK-155
title: "A comment counts the [warn] sites to justify a stream choice, and the count is already wrong"
type: chore
priority: P4
effort: S
created-at: 2026-08-03T14:45:00+09:00
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
    result: merge.go no rotting warn census; stderr warn test
verification-summary: |
  quality-review pass; re-checked deliverables. merge.go no rotting warn census; stderr warn test. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 155: Stop shipping a census in a comment

## Acceptance criteria

- [x] `merge.go` keeps the stream rationale and loses the census.
- [x] Stream invariant remains in `TestStackOverrideConflictWarnsOnStderrNotStdout`.
- [x] Measured count at fix time (informational only).
- [x] Sweep for similar census comments.
- [x] `make test` exits 0.

## Result

Comment now points at the stderr rationale + the test, not a rotting site count.

**Measured 2026-08-07:**

```
rg -n '\[warn\]' --glob '*.go' cmd internal tools | rg -v '_test.go' | wc -l
# → 24
```

**Sweep** (`Of the N|N production|exactly N` in production comments under `internal/`/`cmd/`):
matches were almost entirely test `Fatalf` strings and one provision auto-fallback comment —
**0** additional production census-in-comment claims like the merge.go one. Stale subset of
that pattern: **1** (this site), fixed.
