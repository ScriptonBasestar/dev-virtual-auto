---
id: TASK-159
title: "doctor --strict is the CI flag nobody can find"
type: docs
priority: P3
effort: S
completed-at: 2026-08-07
scope: "USAGE.md, CHANGELOG.md"
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
    result: USAGE+CHANGELOG doctor --strict documented
verification-summary: |
  quality-review pass; re-checked deliverables. USAGE+CHANGELOG doctor --strict documented. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 159

## Result

- `USAGE.md` doctor section: `--strict` + advisory default prose (sibling form to
  `validate --strict`).
- `CHANGELOG.md` Unreleased: under the same advisory-exit doctor entry.
- Sweep (`doctor`, `config validate`, `provision`, `run`, `up` local flags vs USAGE):
  **0** missing under heuristic (denominator: those five commands' non-global flags).
- `make doc-check`: links_checked > 0, exit 0.
