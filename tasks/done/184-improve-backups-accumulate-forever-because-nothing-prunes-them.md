---
id: TASK-184
title: "Nothing prunes backups/dva, so every run adds a file that is never removed"
type: chore
priority: P2
effort: S
created-at: 2026-08-18T15:24:47+09:00
completed-at: 2026-08-18T21:10:00+09:00
source: "4ec336b — backup_config only copies; no step or command deletes"
scope: "dva repo — agent-mesh-flows/dva-improve.yaml, agent-mesh-flows/dva-improve-guided/30-configure.yaml"
status: done
---

# Task 184: Decide how long a snapshot is worth keeping

## Summary

`backup_config` copies and never deletes. Each `dva-improve` or guided run left one more
timestamped file in `backups/dva/`, and no step, command, or documented chore removed any
of them. The directory carries a `.gitignore` of `*`, so nothing would ever appear in
`git status` to prompt a cleanup — which is exactly why it would never be noticed.

## Resolution

Keep-N, N = 10, applied by a `prune_backups` step that runs straight after the snapshot is
written on both paths. Keep-N over age-based expiry because it stays bounded regardless of
how often the flow runs, and pruning after the write bounds the directory at exactly 10
rather than 11.

Two decisions came out differently than the card anticipated, both from measurement:

**A shell loop, not `op: delete`.** The card's technical note preferred `op: delete` on an
explicit path. Measured against am cb8b4ce, `file/delete` hands its path to a single remove
call: `path: "g/*.bak"` fails with `no such file or directory` on the literal name. It
cannot expand a glob, so it deletes exactly one file per run, and a fifty-file backlog
would need forty runs to reach the bound. `rm` and every other command in the pipeline are
in the am allowlist, and the step runs unprompted under `-y`.

**Sorted on the timestamp field, not on the whole name.** `docs/50` already decided, with
its reason recorded, that the name is canonical and mtime is not — a snapshot copied in
from elsewhere carries an mtime that disagrees with its own name. That decision stands. But
the snippet it shipped sorts on the whole name, and `dva.yaml.*` sorts ahead of `dva.yml.*`
because `a` < `m`, so a project that renamed its config would have its **newest** snapshots
pruned. `sort -t '.' -k 3 -r` reads only the timestamp, which is field 3 for both names and
fixed width, so a text sort of it is chronological.

## Completion Criteria

- [x] A retention policy is applied where the snapshot is written | verify: `grep -q 'id: prune_backups' agent-mesh-flows/dva-improve.yaml`
- [x] Running the flow past the retention bound removes the oldest snapshot | verify: human — run against a fixture N+1 times, observe the count stop growing and the oldest file gone
- [x] Pruning never touches anything outside the backup directory | verify: human — the delete is scoped to `backups/dva/` and matches only `*.bak`
- [x] Both write paths are covered, not just `dva-improve.yaml` | verify: `grep -q 'id: prune_backups' agent-mesh-flows/dva-improve-guided/30-configure.yaml`
- [x] Flows still validate | verify: `am validate agent-mesh-flows/dva-improve.yaml`

## Evidence

The `action` was extracted from the shipped flow by parsing the YAML — not retyped — and
run under `am … -y` against fixtures.

| fixture | before | after |
| --- | --- | --- |
| 13 snapshots | 13 | the newest 10; `20260801/02/03` gone |
| rerun at the bound | 10 | 10, exit 0 |
| 3 snapshots (under the bound) | 3 | 3 |
| no `backups/dva/` at all | — | exit 0, `Done` |
| 5 further runs, one new snapshot each | 10 | 10 every time, newest present |

**Scope.** The same 13-file run left untouched: `.gitignore` inside the directory,
`notes.txt` inside the directory, `backups/outside2.bak` one level up, and `outside.bak` at
the fixture root. `ls` without `-a` never lists the marker, and the grep demands a `.bak`
suffix on top of that; every name passed to `rm` comes from `ls` of the directory the step
has already cd'd into, so no name can hold a slash.

**The sort field is load-bearing.** Ten July `dva.yml` snapshots plus three August
`dva.yaml` ones, the state left by a config rename:

| ordering | keeps |
| --- | --- |
| whole name (`sort -r`, the old doc snippet) | all ten July `dva.yml` — every August snapshot deleted |
| field 3 (shipped) | the three August `dva.yaml` plus the seven newest July ones |

**Gates.** `go run ./tools/flowcheck .` → `OK — no decision-path defects` across 103 shell
fields; `am validate` valid on both files; `make doc-check` OK.

## Technical Notes

- The step is gated on the same flag as `backup_config` on each path
  (`check_config.has_dva_yml` / `backup_paths.has_config`), so the snapshot trio moves as a
  unit and a fresh project with nothing to lose runs no delete at all.
- `cd '…' 2>/dev/null || exit 0` is what makes the missing-directory case a success rather
  than a blocked step; `[ -n "$f" ]` inside the loop absorbs the single blank line `printf`
  emits when nothing is over the bound.
- Every filename and separator is quoted, including the `.` argument to `sort -t`: unquoted
  it is a bare word in command position for am's analyzer, and `.` is the source builtin.
