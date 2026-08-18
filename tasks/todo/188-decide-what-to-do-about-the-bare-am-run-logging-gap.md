---
id: TASK-188
title: "Decide what to do about the 638 bare `am run` log entries that record no flow name"
type: chore
priority: P3
effort: XS
created-at: 2026-08-18T15:24:47+09:00
source: "config-damage investigation — usage.log cannot attribute interactive runs to a flow"
scope: "decision only — no dva code; possible upstream issue against agent-mesh"
status: todo
---

# Task 188: Close or accept the attribution gap

## Summary

Investigating whether past `dva-improve` runs had silently rewritten valid configs ended
in an evidence gap that cannot be closed from this repository. `am`'s `usage.log` records
`am run` with no flow name when the flow was chosen interactively — 638 such entries
between 2026-07-03 and 2026-08-18. Named entries exist only for `dva-discover` (×8,
read-only). So the question "did an improve run alter a config that was already valid" has
no answer in the record, and never will for runs already made.

Two things are now true and neither is a fix for the past. The gate defects that made those
runs plausible are fixed (`63ee185`), and every future run snapshots the config first
(`4ec336b`). The gap is strictly historical.

The logging itself belongs to agent-mesh, not dva. The decision is whether to raise it
upstream — an interactive run recording no flow name makes any later audit impossible, for
any consumer, not just this repo — or to accept the gap and stop spending on it.

This card exists so the question is closed deliberately rather than forgotten.

## Completion Criteria

- [ ] The decision is recorded with its reasoning | verify: human — the outcome (raise upstream / accept) and why is written in this card or under `tasks/decision/`
- [ ] If raised upstream, the issue is linked | verify: human — issue URL recorded
- [ ] If accepted, the card states what evidence would exist for future runs instead | verify: human — read the recorded decision

## Technical Notes

- Precedent that this class of damage is real, not hypothetical:
  `tasks/_archive/135-*` records `fix_version` unconditionally `sed -i`-rewriting `version:`
  in every config it touched.
- Nothing in dva can reconstruct the missing attribution retroactively.
