---
id: TASK-142
title: "dva validate prints ✅ on stdout while the warnings that qualify it go to stderr"
type: bug
priority: P3
effort: S
created-at: 2026-08-03T13:00:00+09:00
completed-at: 2026-08-07
scope: "dva repo — internal/cli/validate.go"
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
    result: validateNoticeWriter human stdout; ValidateWithoutJSON PASS
verification-summary: |
  quality-review pass; re-checked deliverables. validateNoticeWriter human stdout; ValidateWithoutJSON PASS. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 142: Put the verdict and its qualifiers on the same stream

## Result

### Rule

**Human path:** notices that qualify the ✅ verdict share **stdout** with it.  
**`--json`:** stdout is reserved for the single document; notices stay on **stderr** so
prose cannot corrupt the machine document (TASK-088 contract kept).

`validateNoticeWriter()` encodes that split.

### Byte counts (stack.*.order fixture)

| | before (task) | after |
|--|---------------|--------|
| stdout | 21 (`✅…`) | ~993 (warnings + verdict) |
| stderr | ~837 | **0** |

### JSON

`dva validate --json 2>/dev/null | jq -s 'length'` → **1** (unchanged document shape).

### Tests

`TestValidateWithoutJSONIsUnchanged` updated for co-located human notices.
`make test` exit 0.
