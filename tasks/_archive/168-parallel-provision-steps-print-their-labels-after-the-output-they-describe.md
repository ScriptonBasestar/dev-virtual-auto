---
id: TASK-168
title: "Parallel provision steps print their labels after the output they describe"
type: bug
priority: P3
effort: M
created-at: 2026-08-03T16:15:00+09:00
completed-at: 2026-08-03T18:35:00+09:00
source: "TASK-140 — found while evaluating whether executeParallelBatch's buffering could be reused on the interaction path"
depends-on: []
scope: "dva repo — internal/cli/provision.go, internal/cli/prefixwriter.go"
status: done
quality-review: pass
quality-reviewed-at: 2026-08-07T18:05:08+09:00
verified-at: 2026-08-07T18:05:08+09:00
archived-at: 2026-08-07T18:05:08+09:00
quality-review-evidence: |
  - kind: test
    command-or-step: make test && make doc-check (mise go 1.26.4)
    result: exit 0; shared suite green
  - kind: recheck
    command-or-step: acceptance criteria re-observed
    result: stepPrefixWriter parallel labels; provision_parallel_output_test
verification-summary: |
  quality-review pass; re-checked deliverables. stepPrefixWriter parallel labels; provision_parallel_output_test. Shared: make test OK; make doc-check OK (mise go 1.26.4).
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

- [x] Every line a parallel batch emits is attributable to the step that produced it, by
      position or by prefix.
- [x] No label appears after the output it describes.
- [x] Sequential paths are unchanged — `dva provision` on a profile with no `parallel:`
      step, and every `dva run` interaction, still stream to the terminal as they do now.
      State the measured before/after for one of each.
- [x] `internal/exec`'s existing callers are not forced to pass a writer they do not need.
- [x] A test fails without the change, and its `-run` pattern is proven to match a real
      test name (an unanchored pattern matching zero tests still exits 0).
- [x] `make test` and `make lint` exit 0. — `make test` does; `make lint` fails on the
      known GOTOOLCHAIN drift, **proven pre-existing** below.

## Result

### The premise in Direction was wrong: this never reaches `internal/exec`

Direction asserts "the fix has to reach `internal/exec`, which is the part that makes this
more than a provision.go change." It does not, and the check is one grep:
`grep -c dvaexec internal/cli/provision.go` → **0**. The two helpers that run a parallel
step's command — `runShellCommand` and `runProvisionCompose` — are local to provision.go and
build their own `exec.Cmd`, hardcoding `os.Stdout` themselves. `internal/exec`'s 20 real
`ExecSubprocess*` callers are all in `lifecycle/` and `runner/`, none on this path.

So criterion 4 ("existing callers are not forced to pass a writer they do not need") is
satisfied by not touching that package at all. `internal/exec/exec.go:75` is still the line
the Problem section quotes, and it is still hardcoded — it just is not what prints a
provision step's output. A future task that wants injectable streams in `internal/exec` for
`lifecycle`/`runner` is unblocked by this one, not done by it.

### Design choice: prefix, and keep the buffer only for `--dry-run`

**Prefixing.** A parallel batch exists precisely because its steps are slow. Buffering the
children would produce tidy per-step blocks, but the terminal stays silent until the batch
joins, so a hung step and a working step look identical for as long as the batch runs —
which is the whole duration a user would want to tell them apart. Prefixing keeps the
streaming the sequential path already has and makes every line attributable, which is what
both of the first two criteria actually ask for.

**`--dry-run` keeps the buffer**, deliberately. No child process runs, so the defect cannot
occur, and a *plan* answering "what will happen" reads better in declaration order than in
goroutine-completion order. `TestParallelDryRunStaysInDeclarationOrder` pins this over 20
repetitions, because a scheduling-order regression passes a single run about half the time.

New file `internal/cli/prefixwriter.go` holds `stepPrefixWriter`: a line-splitting
`io.Writer` that tags each complete line and holds a partial line until its newline arrives.
Labels are padded to a common width by **rune** count, not byte count, so a CJK step name
does not shear the column. Every writer in one batch shares **one** mutex, because they
share one terminal.

`writeNote` was also changed to compose its block into a `strings.Builder` and issue a
single `io.WriteString`. Its three separate `Fprintf` calls took the writer's lock three
times, which let another step's line land between the note's blank lines.

### Measured before/after

Fixture: two `parallel: true` steps (`alpha`, `beta`) emitting five lines each, plus a
sequential `gamma`. Before = `bin/dva` v0.1.44 built from `1b88f8e` in a detached
`git worktree`; after = this change.

**Before** — ten unattributable lines, then the labels (full capture in Problem above).

**After** — every child line names its step, and each label precedes its own output:

```
🚀 Running provision profile: demo

  ⚡ Running 2 steps in parallel...
  [2/3] beta  │     $ for i in 1 2 3 4 5; do echo BETA-$i; sleep 0.05; done
  [1/3] alpha │     $ for i in 1 2 3 4 5; do echo ALPHA-$i; sleep 0.05; done
  [2/3] beta  │ BETA-1
  [1/3] alpha │ ALPHA-1
  [1/3] alpha │ ALPHA-2
  [2/3] beta  │ BETA-2
  [1/3] alpha │ ALPHA-3
  [2/3] beta  │ BETA-3
  [1/3] alpha │ ALPHA-4
  [2/3] beta  │ BETA-4
  [1/3] alpha │ ALPHA-5
  [2/3] beta  │ BETA-5
  [3/3] gamma
    $ echo GAMMA-sequential
GAMMA-sequential

✅ Provision complete!
```

The two steps still interleave — they run concurrently, and forcing an order would be the
buffering this task rejected. What changed is that interleaving is now readable. Note
`[3/3] gamma`, the sequential step in the same profile: unprefixed, exactly as before.

**After, `--dry-run`** — buffered, in declaration order, no prefixes:

```
🔍 DRY RUN — showing execution plan for profile: demo

  ⚡ Running 2 steps in parallel...
  [1/3] alpha
    [dry-run] $ for i in 1 2 3 4 5; do echo ALPHA-$i; sleep 0.05; done
  [2/3] beta
    [dry-run] $ for i in 1 2 3 4 5; do echo BETA-$i; sleep 0.05; done
  [3/3] gamma
    [dry-run] $ echo GAMMA-sequential

🔍 Dry run complete — no commands were executed.
```

**Criterion 3 — one of each, before vs after, diffed byte for byte.**

`dva provision seq` on a profile with no `parallel:` step — **IDENTICAL, byte for byte**,
and non-empty (the note and both steps are present, so this is not a vacuous pass):

```
🚀 Running provision profile: seq

  [1/2] one

    a note on the sequential path

    $ echo SEQ-ONE-A; echo SEQ-ONE-B
SEQ-ONE-A
SEQ-ONE-B
  [2/2] two
    $ echo SEQ-TWO
SEQ-TWO

✅ Provision complete!
```

`dva run greet`, a `runner: local` interaction — **IDENTICAL, byte for byte**, at
`before=35 bytes, after=35 bytes` (`INTERACTION-RAN` / `INTERACTION-LINE-2`). The byte
count is recorded because the first attempt at this measurement produced **0 bytes on both
sides** and reported "IDENTICAL" — a vacuous pass caused by a fixture written with `desc:`
and `local:` instead of `description:` / `runner: local` / `command:`. Two empty strings
compare equal. The count is what caught it.

### Falsifications

Four, each reverting one independent part of the change; the point is that the failures are
disjoint, so each part is load-bearing on its own.

| # | Reverted to | Result |
|---|---|---|
| F1 | children write to `os.Stdout` again | only `TestParallelBatchAttributesEveryCommandLine` fails — `unattributed output line "ALPHA-1" — it names no step` |
| F2 | the `$ …` echo goes back to the buffer | both batch tests fail — `"ALPHA-1" never appeared after its own echo` |
| F3 | `Flush` made a no-op | only `TestStepPrefixWriter/Flush_emits_a_line_with_no_trailing_newline` fails |
| F4 | per-writer mutex instead of one shared mutex | only `TestStepPrefixWriter/concurrent_steps_never_interleave_mid-line` fails — `interleaved line: "123456789"` |

**F4 passed on the first attempt, and that was the finding.** The subtest wrote to a
`strings.Builder`, and `emit` issues exactly one `fmt.Fprintf` per line — a single `Write`
to a Builder cannot be split, so removing the lock changed nothing observable. The test was
proving a property of `fmt`, not of the mutex. It was rewritten around `splitWriter`, which
delivers every `Write` in two pieces with a `runtime.Gosched()` between them, restoring the
property the lock is actually for: an underlying writer that does not deliver a write whole.
`os.Stdout` is exactly that writer once a line exceeds `PIPE_BUF` (4096) and stdout is a
pipe — `dva provision | tee log`. F4 then failed as tabled. Had the vacuous version been
kept, the shared mutex would have looked unnecessary.

### Criterion 5 — the `-run` pattern matches real tests

An unanchored `-run` pattern that matches nothing still exits 0, so the pattern was proven
to bind: the five-test pattern produced **25** combined `=== RUN` / `--- PASS` lines
(`ok github.com/ScriptonBasestar/dva/internal/cli 171.893s`), while the contrast pattern
`TestNoSuchParallelThing` produced **0**.

### Gates

`make test` — **pass**, every package; `internal/cli` ok at 68.6% coverage. `gofmt` clean,
`go vet` ok. The note test was additionally run at `-race -count=15` to confirm the
`writeNote` locking fix, after it passed standalone but failed under the full suite.

`make lint` — **fails, and provably not from this change.** It reports 5 `typecheck` issues,
all of the form `could not import os (… compile: version "go1.26.4" does not match go tool
version "go1.26.5")`, in `tools/doccheck/check.go` — a file this change does not touch.
Running `make lint` in the untouched `1b88f8e` worktree produces the **identical** 5
typecheck issues. This is the known GOTOOLCHAIN drift; TASK-145 recorded the same.

`TestParallelBatchPrintsNote/executing` (TASK-086's guard) had to be updated rather than
left alone: it asserted the note block byte for byte, which on the executing path now
forbids the prefix — i.e. it would forbid the note being attributable to its step on the one
path where several steps print at once. The assertion is now per-mode: exact bytes under
`--dry-run`, and on the executing path a prefixed content line with a blank line either
side. The rendering guarantee it was written to protect is intact.

## Related

- [TASK-140](../done/140-interaction-steps-ignore-parallel-while-the-schema-advertises-it.md)
  — where this was found. TASK-140 chose *not* to reuse `executeParallelBatch` on the
  interaction path partly because of this defect: copying the model would have propagated it.
- [TASK-086](../_archive/086-parallel-steps-discard-their-note.md) — note rendering on the
  same function; adjacent, not this. Its title says it: the *note* was discarded, not the
  output.
