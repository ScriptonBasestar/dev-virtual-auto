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
quality-review: pass
quality-reviewed-at: 2026-08-19T13:56:31+09:00
verified-at: 2026-08-19T14:18:58+09:00
archived-at: 2026-08-19T14:18:58+09:00
quality-review-evidence: |
  - kind: automated
    command-or-step: "grep -rq 'backups/dva' docs/  (AC1 verify binding)"
    result: exit 0 — the binding is scoped to docs/ and matches docs/50-improve-flow-backup-and-restore.md alone. USAGE.md names the directory too (:973, :977, :978) but sits at the repository root, outside this grep. Corrected after archival: this line first read "docs/50 ... and USAGE.md both name the directory", crediting the binding with a match it cannot make
  - kind: manual
    command-or-step: "AC2 — read docs/50 for snapshot-selection rule"
    result: pass — fixed-width zero-padded timestamp makes lexical order == chronological order; the multi-run case (newest snapshot already contains the first run's damage) is stated with the settling `diff`
  - kind: automated
    command-or-step: "AC3 re-run on a fresh fixture from examples/basic.yml: dva validate intact -> overwrite with invalid config -> cp \"$(ls -1 backups/dva/*.bak | tail -1)\" dva.yml -> dva validate"
    result: exit 0 -> exit 1 -> exit 0; the documented one-liner restores a config DVA accepts
  - kind: automated
    command-or-step: "scoped-change check: backup_paths/backup_marker/backup_config/prune_backups present in both dva-improve.yaml and dva-improve-guided/30-configure.yaml"
    result: all four steps present in both flows, matching the doc's implementation table
  - kind: automated
    command-or-step: "make doc-check"
    result: OK — 250 markdown checked, 541 links, 0 broken, 0 oversized (docs/50 is 5628 B, under the 10240 B gate)
verification-summary: |
  quality-review pass, re-observed at disposition. `grep -rq 'backups/dva' docs/` exit 0; the
  restore section named by the review is still in place — docs/50 `## 복원 절차` at :42 with the
  timestamped `cp` at :51 and the newest-snapshot one-liner at :65, and `make doc-check` exit 0
  resolves the inbound link. Deliverables unchanged since review.
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
