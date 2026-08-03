---
id: TASK-148
title: "dva stack statu offers no suggestion, while dva stat does"
type: bug
priority: P3
status: todo
effort: S
created-at: 2026-08-03T13:40:00+09:00
source: "TASK-098 finalize verification — 098's third Left open item, untracked"
depends-on: [TASK-098, TASK-108]
scope: "dva repo — internal/cli/stack.go (NoArgs error path)"
---

# Task 148: Suggest at the subcommand level too

## Problem

Misspell a top-level command and DVA helps:

```
$ dva stat            # rc=1, suggests stop / stack / status
```

Misspell a *sub*command and it does not:

```
$ dva stack statu
ERROR: unknown command "statu" for "dva stack"
rc=1                  # no "Did you mean this?" block at all
```

Both are the same user mistake, one level apart. The difference is structural: cobra's
"Did you mean this?" block is produced by root-level dispatch, while this error comes from
`NoArgs` rejecting an argument — a path cobra's suggestion machinery never reaches.

Re-measured 2026-08-03 on `bin/dva` v0.1.44; both exit 1.

## Why it is still open

TASK-098 recorded three items under "Left open". The first two — unsorted suggestion order
and a duplicate header — were resolved when TASK-108 removed `suggestCommands` and let
cobra own suggestions. This third one was not: cobra owns suggestions now, and cobra does
not suggest here. Both tasks are archived; nothing in `tasks/todo/`, `tasks/blocked/`,
`tasks/decision/` or `tasks/plan/` tracks it.

## Acceptance criteria

- [ ] `dva stack statu` suggests `status`, in the same format and from the same source as
      the top-level suggestion — one mechanism, not a second implementation. TASK-108
      deleted DVA's own suggester deliberately; do not bring it back.
- [ ] The exit code stays 1. TASK-098's whole subject was that this path exited 0; a
      suggestion must not cost that.
- [ ] Every subcommand group with the same `NoArgs` shape behaves the same way — print how
      many were found and checked. `stack` is where it was noticed, not necessarily the
      only one.
- [ ] Exactly one suggestion block is printed, no duplicate header — the defect TASK-108
      fixed at the top level must not reappear one level down.
- [ ] A test covers the misspelled-subcommand case and fails without the change.
- [ ] `make test` exits 0.

## Notes

Check whether cobra's `SuggestFor`/`SuggestionsMinimumDistance` on the parent command
reaches this path before writing anything by hand. `levenshtein` is still in `root.go:391`
and used by `stack.go` and `provision.go`, so the ingredients exist either way.
