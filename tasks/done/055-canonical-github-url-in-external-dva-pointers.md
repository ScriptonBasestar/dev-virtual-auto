---
id: TASK-055
title: "External DVA pointers (ce-plugin, prmpt) must carry the canonical GitHub URL"
type: chore
priority: P3
status: done
effort: XS
created-at: 2026-07-22T00:00:00+09:00
completed-at: 2026-08-07
depends-on: [TASK-054]
scope: "Cross-repo: claude-ce-plugin + ~/workflow/prmpt"
---

# Task 055: Canonical GitHub URL in external DVA pointers

## Result

- **ce-plugin:** already satisfied (prior evidence commit `61f525e` / skillref clean).
- **prmpt:** `~/workflow/prmpt/packages/dva/dogfood/README.md` and `entry.md` now carry:
  `https://github.com/ScriptonBasestar/dev-virtual-auto/tree/master/workflows/dva-dogfood`

Unblocked by TASK-054 ownership + pointer rewrite (workflow path, not devenv).
