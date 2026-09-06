---
id: TASK-328
title: "Run a live dogfood verification round for native entries and composition plans"
type: test
priority: P2
effort: M
exec-tier: standard
created: 2026-09-06
---

## Summary

All devbox migrations so far were verified only with `dva validate` and `--dry-run` lifecycle verbs (agent constraint). This round runs the real verbs against the migrated configs: primeno1's six native entries (gate chain plus `exec`) and the familybook / flow-taskchain composition plans, using `dva up`, `dva status`, and `dva down --purge`, and attaches the exit codes and trimmed output to each project's report under `docs/dogfood/`. Any defect found is promoted to its own card. This is PLAN-006 row 10a; unblocked since TASK-311 (plan logs build scope) landed. Requires a human-operated session because lifecycle verbs beyond `--dry-run` are not permitted for agents.

## Completion Criteria

- [ ] primeno1 native entries complete a real dva up / status / down --purge cycle with output attached to the dogfood report | verify: human — docs/dogfood/primeno1.md contains a 실기동 section with exit codes for up, status, down --purge
- [ ] familybook and flow-taskchain composition plans complete a real up / status / down --purge cycle with output attached | verify: human — docs/dogfood/familybook.md and docs/dogfood/flow-taskchain.md contain a 실기동 section with exit codes
- [ ] PLAN-006 row 10a references this task | verify: `grep -q "10a" /Users/archmagece/mywork/scripton/dva/tasks/plan/006-devbox-dogfood-followup.md`
