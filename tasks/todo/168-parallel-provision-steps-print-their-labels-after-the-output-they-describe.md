---
id: TASK-168
title: "Parallel provision steps print their labels after the output they describe"
type: bug
priority: P3
status: todo
effort: M
created-at: 2026-08-03T16:15:00+09:00
source: "TASK-140 — found while evaluating whether executeParallelBatch's buffering could be reused on the interaction path"
depends-on: []
scope: "dva repo — internal/cli/provision.go, internal/exec/exec.go"
---

# Task 168: Buffer the *commands'* output, not just dva's decorations

## Problem

`executeParallelBatch` (`internal/cli/provision.go`) gives each concurrent step its own
`bytes.Buffer` and flushes the buffers in declaration order after the batch joins. That
looks like it serialises the batch's output. It does not: the buffer receives only the
lines dva itself writes — the `[i/n] label`, the note, the `$ command` echo. The commands
run through `internal/exec`, which hardcodes `c.Stdout = os.Stdout`
(`internal/exec/exec.go:75`), so every child process writes straight to the terminal,
outside the buffer and outside dva's ordering.

The result is that the labels arrive *after* the output they are labelling. Measured
2026-08-03 with `bin/dva` v0.1.44, two parallel steps each emitting five lines:

```
  ⚡ Running 2 steps in parallel...
BETA-1                       ← children, interleaved, unlabelled
ALPHA-1
ALPHA-2
BETA-2
BETA-3
ALPHA-3
ALPHA-4
BETA-4
ALPHA-5
BETA-5
  [1/2] alpha                ← the labels, arriving after their own output
    $ sh -c 'for i in 1 2 3 4 5; do echo ALPHA-$i; ...'
  [2/2] beta
    $ sh -c 'for i in 1 2 3 4 5; do echo BETA-$i; ...'
```

A reader cannot attribute any of the ten lines to a step. With one failing step in a batch
of four, the failure output is somewhere above four labels, unmarked. The buffering is
doing the opposite of its apparent job: without it the labels would at least *precede*
their commands' output.

## Why this is not TASK-086

TASK-086 fixed note *rendering* on this path — which lines dva composes and in what order.
It never touched where the child processes write, so it could not have caught this. Any
future task quoting "TASK-086 already paid for that lesson" as evidence that concurrent
output is handled should read this file first; TASK-140's criterion 4 did exactly that and
was wrong.

## Direction

The fix has to reach `internal/exec`, which is the part that makes this more than a
provision.go change: `Stdout`/`Stderr` need to be injectable so a parallel batch can hand
each step a buffer, while every sequential caller keeps `os.Stdout` and its streaming
behaviour. Note that buffering a child's output also *delays* it — a long-running parallel
step would go silent until the batch joins, which is a real cost for `provision` and worth
weighing against prefixing each line with its step label instead.

Both options are open. Prefixing preserves streaming and attributes every line; buffering
gives clean per-step blocks but hides progress. Record which and why.

## Acceptance criteria

- [ ] Every line a parallel batch emits is attributable to the step that produced it, by
      position or by prefix.
- [ ] No label appears after the output it describes.
- [ ] Sequential paths are unchanged — `dva provision` on a profile with no `parallel:`
      step, and every `dva run` interaction, still stream to the terminal as they do now.
      State the measured before/after for one of each.
- [ ] `internal/exec`'s existing callers are not forced to pass a writer they do not need.
- [ ] A test fails without the change, and its `-run` pattern is proven to match a real
      test name (an unanchored pattern matching zero tests still exits 0).
- [ ] `make test` and `make lint` exit 0.

## Related

- [TASK-140](../done/140-interaction-steps-ignore-parallel-while-the-schema-advertises-it.md)
  — where this was found. TASK-140 chose *not* to reuse `executeParallelBatch` on the
  interaction path partly because of this defect: copying the model would have propagated it.
- [TASK-086](../_archive/086-parallel-steps-discard-their-note.md) — note rendering on the
  same function; adjacent, not this. Its title says it: the *note* was discarded, not the
  output.
