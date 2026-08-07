---
id: TASK-154
title: "The (CI) suffix marks one of the five targets CI actually runs"
type: bug
priority: P3
effort: S
completed-at: 2026-08-07
scope: "Makefile, tools/cilabels, .github/workflows/ci.yml"
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
    result: cilabels OK 5 CI targets match Makefile
verification-summary: |
  quality-review pass; re-checked deliverables. cilabels OK 5 CI targets match Makefile. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 154

## Result

**Option A** — label all five; gate with `go run ./tools/cilabels` from `make doc-check`.

| Source | Count | Targets |
|--------|-------|---------|
| `ci.yml` `run: make …` | **5** | build, doc-check, fmt-check, test, test-integration |
| Makefile `(CI)` labels | **5** | same set |

Mismatch fails `make doc-check`. Archive TASK-112 points here for the corrected claim.
