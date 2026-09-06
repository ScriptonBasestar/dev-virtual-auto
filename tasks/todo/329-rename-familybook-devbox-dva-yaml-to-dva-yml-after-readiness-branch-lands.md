---
id: TASK-329
title: "Rename familybook devbox dva.yaml to dva.yml after readiness branch lands"
type: chore
priority: P3
effort: S
exec-tier: standard
status: todo
created: 2026-09-06
---

## Summary

familybook-devbox still uses the legacy `dva.yaml` filename, which `dva validate` accepts with a warning (TASK-304). The rename to `dva.yml` is blocked until the readiness-contract branch `readiness-dva-yml` (3538cf2) is integrated by a human, since agent integration cannot touch `.gz-git/readiness/*`. After that lands, rename the file in a task worktree, confirm `dva validate` runs without the legacy-name warning, and integrate via `branch-integrate --target develop`.

## Completion Criteria

- [ ] familybook readiness branch (3538cf2) is integrated by a human first | verify: human — origin/develop of familybook-devbox contains the readiness-dva-yml commit
- [ ] dva.yaml is renamed to dva.yml and validate passes without the legacy-name warning | verify: `test -f /Users/archmagece/mydevbox/familybook-devbox/dva.yml`
