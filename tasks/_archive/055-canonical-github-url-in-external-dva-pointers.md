---
id: TASK-055
title: "External DVA pointers (ce-plugin, prmpt) must carry the canonical GitHub URL"
type: chore
priority: P3
effort: XS
created-at: 2026-07-22T00:00:00+09:00
completed-at: 2026-08-07
depends-on: [TASK-054]
scope: "Cross-repo: claude-ce-plugin + ~/workflow/prmpt"
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
    result: GitHub canonical URL in prmpt dogfood README/entry
verification-summary: |
  quality-review pass; re-checked deliverables. GitHub canonical URL in prmpt dogfood README/entry. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 055: Canonical GitHub URL in external DVA pointers

## Result

- **ce-plugin:** already satisfied (prior evidence commit `61f525e` / skillref clean).
- **prmpt:** `~/workflow/prmpt/packages/dva/dogfood/README.md` and `entry.md` now carry:
  `https://github.com/ScriptonBasestar/dev-virtual-auto/tree/master/workflows/dva-dogfood`

Unblocked by TASK-054 ownership + pointer rewrite (workflow path, not devenv).
