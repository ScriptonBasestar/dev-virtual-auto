---
id: TASK-183
title: "The improve flows take a snapshot but offer no way to restore it"
type: feature
priority: P1
effort: M
created-at: 2026-08-18T15:24:47+09:00
source: "4ec336b — backup landed with no restore path"
scope: "dva repo — agent-mesh-flows/dva-improve.yaml, agent-mesh-flows/dva-improve-guided/, docs/"
status: todo
---

# Task 183: Give the snapshot a way back

## Summary

`4ec336b` made both write paths copy the config to
`tmp/dva-improve-backups/<name>.<timestamp>.bak` before an agent edits it. Nothing reads
that directory back. A user whose config was rewritten badly has to discover the path,
pick the right timestamp, and `cp` by hand — none of which is written down anywhere.

A backup delivers its value only at restore time. Until that half exists the feature
reliably does one thing: consume disk.

The open question is where restore belongs. A flow step is wrong — restoring is something
a user decides *after* seeing a bad result, not part of the run that produced it. The
plausible homes are a documented one-liner, or a `dva` subcommand that lists snapshots and
restores one. Prefer the smallest thing that works; a subcommand is only justified if
listing and choosing by timestamp is genuinely awkward by hand.

## Completion Criteria

- [ ] A restore procedure is documented and names the backup directory | verify: `grep -rq 'dva-improve-backups' docs/`
- [ ] The procedure states which snapshot to pick when several exist | verify: human — read the section; timestamp ordering is explicit
- [ ] Restoring a snapshot over an edited config yields a config DVA accepts | verify: human — copy a `.bak` over `dva.yml`, run `dva validate`, expect exit 0

## Technical Notes

- Snapshot naming: `<config name>.<YYYYmmdd-HHMMSS>.bak`, written by `backup_config`
  (`op: copy`) in both `dva-improve.yaml` and `dva-improve-guided/30-configure.yaml`.
- The snapshot captures the **working tree**, not `HEAD` — that is the whole point, since
  `git checkout --` already covers the committed case by discarding the uncommitted one.
- Depends on nothing; blocks nothing. TASK-184 (retention) decides how long a snapshot
  stays restorable, so the two should agree on the directory contract.
