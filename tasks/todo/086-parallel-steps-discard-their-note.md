---
id: TASK-086
title: "`note:` is printed on the sequential provision path and silently discarded on the parallel one"
type: fix
priority: P3
effort: S
status: todo
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/cli — provision.go executeParallelBatch, compose.go native build loop"
---

# Task 086: adding `parallel: true` deletes the step's message

## Problem

`note:` is how a provision step says something to the operator without running anything. The
sequential path prints it — `internal/cli/provision.go:125-127`:

```go
if step.Note != "" {
    for _, line := range strings.Split(step.Note, "\n") {
```

`executeParallelBatch` never looks at it. The function is 102 lines and contains **zero**
references to `Note`. `internal/cli/compose.go` contains zero in the entire file. So marking a step
`parallel: true` — a scheduling hint — silently deletes its message.

## Evidence (measured on 0.1.44)

One fixture, two profiles, the same `note:` key:

```
########## CONTROL: note on the SEQUENTIAL provision path ##########
exit=0
  [1/1] a note on the sequential path
    SEQ-NOTE-VISIBLE
>> SEQ-NOTE-VISIBLE count (expect >=1): 1

########## FINDING: note on the PARALLEL provision path ##########
exit=0
  ⚡ Running 2 steps in parallel...
PAR-CONTROL-RAN
  [1/2] a note on the parallel path
  [2/2] a second parallel item so a batch forms
    $ echo PAR-CONTROL-RAN
>> PAR-NOTE-VISIBLE count (0 = note silently dropped): 0
>> PAR-CONTROL-RAN count (proves the batch really executed): 2
```

The second control is the one that makes this readable. `PAR-CONTROL-RAN` appearing **twice**
(echoed command + its output) proves the batch genuinely ran, so the missing note is a dropped
message and not a profile that failed to execute. Note also that the *label* still prints —
`[1/2] a note on the parallel path` — which makes the loss worse than plain silence: the step
looks like it reported, and only its content is gone.

Fixture:

```yaml
version: "0.1.44"
provision:
  sequential:
    - step: "a note on the sequential path"
      note: "SEQ-NOTE-VISIBLE"
  parallelbatch:
    - step: "a note on the parallel path"
      parallel: true
      note: "PAR-NOTE-VISIBLE"
    - step: "a second parallel item so a batch forms"
      parallel: true
      run: "echo PAR-CONTROL-RAN"
```

A second, independent path has the same hole: `compose.go`'s native build loop (the one
[TASK-083](../done/083-a-step-without-run-announces-work-it-never-does.md) had to add an inert-step notice
to at `compose.go:464`) also never reads `Note`.

## Why this is P3 and not P2

Nothing executes wrongly and no exit code lies — only an operator message is lost. It is filed
because it is the same silent-drop family as
[TASK-083](../done/083-a-step-without-run-announces-work-it-never-does.md) and
[TASK-085](085-interaction-steps-silently-drop-compose-keys.md), and because a note is *by
definition* the thing whose entire purpose is to be seen.

## Proposed fix

Print the note in `executeParallelBatch` the way `executeProvisionStep` does, into the per-step
`&buf` the batch already accumulates so interleaving stays safe, and add the same to
`compose.go`'s loop. The multi-line split at `provision.go:127` should be shared rather than
copied a third time — three call sites is where TASK-074's five duplicated literals started.

Worth deciding while here, but **not** in this task's scope: the runners check `Note` *before*
`RunCommands()` and `continue`, so an item carrying both a note and a `run:` prints the note and
never runs the command. That ordering is a separate question about what a two-payload item means;
see Left open.

## Non-goals

- Not changing note *text* or formatting on the sequential path. The two should agree, and the
  sequential one is the reference.
- Not resolving the note-vs-run ordering question. Filing the observation only.

## Acceptance criteria

- [ ] A note on the parallel path is printed | verify: `dva provision parallelbatch` on the fixture — grep the note marker, print the count, expect >=1
- [ ] The sequential path is byte-identical to before | verify: `human — same fixture, diff the sequential profile's output against the pre-change capture`
- [ ] The parallel batch still executes | verify: same run — the sibling `run:` marker count must stay 2, so a note fix cannot be confused with a broken batch
- [ ] `compose.go`'s loop prints it too | verify: `grep -c '\.Note' internal/cli/compose.go` — currently 0, must be non-zero
- [ ] Covered by a test that fails without the fix | verify: `go test ./internal/cli/ -run TestParallelBatchPrintsNote`
- [ ] Not vacuous | verify: `human — revert the executeParallelBatch hunk alone and confirm only that subtest fails`
- [ ] Full suite passes | verify: `make test`

## Left open

- **Note-before-run ordering** — now filed as
  [TASK-089](../done/089-note-suppresses-run-on-the-interaction-path-only.md), with a correction to what
  this entry originally said. Only the two *runners* `continue` after printing a note.
  `executeProvisionStep` does not: it prints the note at `provision.go:125-131` and falls through
  to execution. Measured — `{note, run}` executes under `dva provision` and silently does not
  under `dva run`. So this is not one shared ordering choice needing a decision, it is two paths
  disagreeing, and the provision one is right.
- **`hooks.go` writes to stderr while `provision.go` writes to stdout** for the same class of
  step message. Out of scope here, but it means "where does a note go" has two answers today.

## Related

- [TASK-085](085-interaction-steps-silently-drop-compose-keys.md) — found in the same survey;
  compose keys dropped on the interaction path.
- [TASK-083](../done/083-a-step-without-run-announces-work-it-never-does.md) — the fix whose call-site
  audit surfaced both.
