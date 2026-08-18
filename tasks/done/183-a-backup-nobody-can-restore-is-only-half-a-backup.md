---
id: TASK-183
title: "The improve flows take a snapshot but offer no way to restore it"
type: feature
priority: P1
effort: M
created-at: 2026-08-18T15:24:47+09:00
source: "4ec336b — backup landed with no restore path"
scope: "dva repo — agent-mesh-flows/dva-improve.yaml, agent-mesh-flows/dva-improve-guided/, docs/"
status: done
completed-at: 2026-08-18T16:58:00+09:00
---

# Task 183: Give the snapshot a way back

## Summary

`4ec336b` made both write paths copy the config to
`backups/dva/<name>.<timestamp>.bak` before an agent edits it. Nothing reads
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

- [x] A restore procedure is documented and names the backup directory | verify: `grep -rq 'backups/dva' docs/`
- [x] The procedure states which snapshot to pick when several exist | verify: human — read the section; timestamp ordering is explicit
- [x] Restoring a snapshot over an edited config yields a config DVA accepts | verify: human — copy a `.bak` over `dva.yml`, run `dva validate`, expect exit 0

## Resolution

`docs/50-improve-flow-backup-and-restore.md`, linked from USAGE.md's LLM Integration
section. No `dva` subcommand: listing is `ls -1 backups/dva/*.bak` and choosing is reading
a fixed-width timestamp, so a subcommand would wrap two commands a user can already type.
The card asked for the smallest thing that works and this is it.

The part worth writing down turned out not to be the copy command but **which** snapshot to
pick. Each one is the state *before* a run, so after two bad runs the newest snapshot
already contains the first run's damage — the doc says so and shows the `diff` that settles
it. Also stated: the snapshot covers one config file, not the directory moves
`20-transform` makes, so committing before a run is still the real safety net.

### Evidence

Every command in the doc was run verbatim against a fixture built from `examples/basic.yml`:

| step | result |
| --- | --- |
| `dva validate` on the intact config | exit 0 |
| after overwriting with an invalid config | exit 1 |
| `cp backups/dva/<newest>.bak dva.yml` then `dva validate` | exit 0 |
| `cp "$(ls -1 backups/dva/*.bak \| tail -1)" dva.yml` | exit 0 |
| `git add -A` with the marker in place | 0 `backups` entries in the index |
| `ls -1r … \| tail -n +6 \| xargs rm --` keeping 5 of 7 | 5 newest by name remain |

The retention one-liner was corrected before shipping: the first draft used `ls -1t`
(mtime) and `xargs -r`. `-t` contradicts the doc's own claim that the filename timestamp is
canonical, and `-r` is a GNU option — this machine has BSD `/usr/bin/xargs`. The shipped
form sorts by name and was measured to be a no-op on empty input.

## Technical Notes

- Snapshot naming: `<config name>.<YYYYmmdd-HHMMSS>.bak`, written by `backup_config`
  (`op: copy`) in both `dva-improve.yaml` and `dva-improve-guided/30-configure.yaml`.
- The snapshot captures the **working tree**, not `HEAD` — that is the whole point, since
  `git checkout --` already covers the committed case by discarding the uncommitted one.
- Depends on nothing; blocks nothing. TASK-184 (retention) decides how long a snapshot
  stays restorable, so the two should agree on the directory contract.
