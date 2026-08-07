---
id: TASK-154
title: "The (CI) suffix marks one of the five targets CI actually runs"
type: bug
priority: P3
status: done
effort: S
completed-at: 2026-08-07
scope: "Makefile, tools/cilabels, .github/workflows/ci.yml"
---

# Task 154

## Result

**Option A** — label all five; gate with `go run ./tools/cilabels` from `make doc-check`.

| Source | Count | Targets |
|--------|-------|---------|
| `ci.yml` `run: make …` | **5** | build, doc-check, fmt-check, test, test-integration |
| Makefile `(CI)` labels | **5** | same set |

Mismatch fails `make doc-check`. Archive TASK-112 points here for the corrected claim.
