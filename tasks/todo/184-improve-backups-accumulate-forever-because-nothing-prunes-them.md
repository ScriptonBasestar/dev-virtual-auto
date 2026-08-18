---
id: TASK-184
title: "Nothing prunes tmp/dva-improve-backups, so every run adds a file that is never removed"
type: chore
priority: P2
effort: S
created-at: 2026-08-18T15:24:47+09:00
source: "4ec336b — backup_config only copies; no step or command deletes"
scope: "dva repo — agent-mesh-flows/dva-improve.yaml, agent-mesh-flows/dva-improve-guided/30-configure.yaml"
status: todo
---

# Task 184: Decide how long a snapshot is worth keeping

## Summary

`backup_config` copies and never deletes. Each `dva-improve` or guided run leaves one more
timestamped file in `tmp/dva-improve-backups/`, and no step, command, or documented chore
removes any of them. On a config edited repeatedly during a tuning session the directory
grows once per run, indefinitely.

The directory is invisible to git — it carries a `.gitignore` of `*` — which is exactly
why it will not be noticed. Nothing will ever show up in `git status` to prompt a cleanup.

Two policies are defensible: keep the newest N, or drop anything older than N days. Keep-N
is the better default here because it stays bounded regardless of how often the flow runs,
and the value of an old snapshot falls off sharply once the config has been edited past it.

## Completion Criteria

- [ ] A retention policy is applied where the snapshot is written | verify: `grep -q 'id: prune_backups' agent-mesh-flows/dva-improve.yaml`
- [ ] Running the flow past the retention bound removes the oldest snapshot | verify: human — run against a fixture N+1 times, observe the count stop growing and the oldest file gone
- [ ] Pruning never touches anything outside the backup directory | verify: human — the delete is scoped to `tmp/dva-improve-backups/` and matches only `*.bak`
- [ ] Both write paths are covered, not just `dva-improve.yaml` | verify: `grep -q 'id: prune_backups' agent-mesh-flows/dva-improve-guided/30-configure.yaml`
- [ ] Flows still validate | verify: `am validate agent-mesh-flows/dva-improve.yaml`

## Technical Notes

- am's shell policy reads an unquoted bare word as a command name, so any glob or filename
  in the pruning shell must be quoted — see TASK-186 and the `when:` contract comment block
  in `dva-improve-guided.yaml`.
- The criteria bind to a step named `prune_backups` because the obvious binding does not
  work: `grep -qE 'retention|prune|keep'` already exits 0 against `dva-improve.yaml`, where
  `find ... -prune` matches. A criterion that passes before the work starts is worse than
  no criterion. Rename the step freely, but update these bindings with it.
- Deleting files from a flow deserves more care than writing them. Scope the match tightly
  and prefer `op: delete` on an explicit path over a shell `rm` with a pattern.
- The `.gitignore` marker itself must survive pruning; it is what keeps the directory out
  of the target's index.
