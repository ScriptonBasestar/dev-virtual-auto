---
id: TASK-161
title: "Two of the six relocated errcheck exclusions carry a bare _ = and no reason"
type: chore
priority: P4
status: todo
effort: S
created-at: 2026-08-03T15:15:00+09:00
source: "TASK-127 finalize verification — its own criterion, overstated"
depends-on: [TASK-127]
scope: "dva repo — internal/exec/exec.go:166, :175"
---

# Task 161: Name the last two exclusions

## Problem

TASK-127 removed `.golangci.yml`'s `exclusions.presets`, which had been suppressing six errcheck
findings, and moved each suppression to its call site — the point being that a dropped error
should carry its reason next to the code rather than in a config file deciding it elsewhere. Its
criterion states that "the 6 previously-suppressed call sites carry `_ =` with a comment".

Four do. Two do not:

```go
internal/exec/exec.go:166   defer func() { _ = os.Remove(f.Name()) }()
internal/exec/exec.go:175       _ = f.Close()          // WriteString error path
```

Both are bare. The exclusion moved out of the config and stayed unnamed, which is the half of
TASK-127's point that did not land at these two sites.

Both are plausibly benign — a temp-file cleanup, and a Close on a path that is already returning
an error. That is the argument for a one-line comment, not against one: a reader cannot tell a
considered drop from an overlooked one without it, and the neighbouring `f.Close()` twelve lines
below carries three lines explaining exactly why it is *not* dropped.

## Acceptance criteria

- [ ] Both sites carry a reason, in the form the other four use.
- [ ] Re-count: `grep` the six sites TASK-127 relocated and report how many carry a comment,
      before and after. State the denominator.
- [ ] While there, sweep the rest of the non-test tree for bare `_ =` on an error and report the
      count. It is 74 at HEAD across `cmd internal tools`, most of which predate TASK-127 and are
      out of scope — so say which of those are error drops as opposed to ordinary blank
      assignments, and whether that population deserves its own task.
- [ ] `make lint` and `make test` exit 0.

## Notes

TASK-127's prose at lines 73-74 has the matching slip — it enumerates four of the five benign
sites, omitting `exec.go:175`, and calls `provision.go:133` an `fmt.Fprintln` when it is a
`Fprintf`. That file has been corrected inline; this task is only the code half.
