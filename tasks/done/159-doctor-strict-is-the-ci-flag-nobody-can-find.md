---
id: TASK-159
title: "doctor --strict is the CI flag nobody can find"
type: docs
priority: P3
status: done
effort: S
completed-at: 2026-08-07
scope: "USAGE.md, CHANGELOG.md"
---

# Task 159

## Result

- `USAGE.md` doctor section: `--strict` + advisory default prose (sibling form to
  `validate --strict`).
- `CHANGELOG.md` Unreleased: under the same advisory-exit doctor entry.
- Sweep (`doctor`, `config validate`, `provision`, `run`, `up` local flags vs USAGE):
  **0** missing under heuristic (denominator: those five commands' non-global flags).
- `make doc-check`: links_checked > 0, exit 0.
