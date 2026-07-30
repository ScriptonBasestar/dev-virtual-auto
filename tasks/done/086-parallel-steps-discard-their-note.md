---
id: TASK-086
title: "`note:` is printed on the sequential provision path and silently discarded on the parallel one"
type: fix
priority: P3
effort: S
status: done
created-at: 2026-07-31T00:00:00+09:00
completed-at: 2026-07-31T08:20:00+09:00
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
[TASK-083](083-a-step-without-run-announces-work-it-never-does.md) had to add an inert-step notice
to at `compose.go:464`) also never reads `Note`.

## Why this is P3 and not P2

Nothing executes wrongly and no exit code lies — only an operator message is lost. It is filed
because it is the same silent-drop family as
[TASK-083](083-a-step-without-run-announces-work-it-never-does.md) and
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

- [x] A note on the parallel path is printed | verify: `dva provision parallelbatch` on the fixture — grep the note marker, print the count, expect >=1 — **`PAR-NOTE-VISIBLE` 0 → 1; also 1 under `--dry-run`, which is a separate branch of the same function**
- [x] The sequential path is byte-identical to before | verify: `human — same fixture, diff the sequential profile's output against the pre-change capture` — **captured before the edit, `diff` empty, 130 bytes / 8 lines both times**
- [x] The parallel batch still executes | verify: same run — the sibling `run:` marker count must stay 2, so a note fix cannot be confused with a broken batch — **`PAR-CONTROL-RAN` 2 before, 2 after**
- [x] `compose.go`'s loop prints it too | verify: `grep -c '\.Note' internal/cli/compose.go` — currently 0, must be non-zero — **1 — but the grep alone would have been vacuous; see Resolution, the runtime evidence needed `DVA_HOOK_DEPTH=1`**
- [x] Covered by a test that fails without the fix | verify: `go test ./internal/cli/ -run TestParallelBatchPrintsNote` — **2 subtests (executing, dry-run), plus 3 sibling tests in `provision_note_test.go`**
- [x] Not vacuous | verify: `human — revert the executeParallelBatch hunk alone and confirm only that subtest fails` — **3 probes, one per hunk; each failed exactly its own test and nothing else (see table below)**
- [x] Full suite passes | verify: `make test` — **all packages ok under `-race`; `internal/cli` coverage 60.9% → 61.4%**

## Resolution

One shared renderer, three call sites:

```go
func writeNote(w io.Writer, note string) {
	if note == "" {
		return
	}
	fmt.Fprintln(w)
	for _, line := range strings.Split(note, "\n") {
		fmt.Fprintf(w, "    %s\n", line)
	}
	fmt.Fprintln(w)
}
```

The `io.Writer` is the whole reason this works on the parallel path: `executeParallelBatch`
accumulates each step into its own `bytes.Buffer` and prints the buffers in order after
`wg.Wait()`, so writing the note into `&buf` keeps concurrent steps from interleaving
mid-line. Placed after the `IsInert()` early return and **before** the `dryRun` branch — a
note describes the step, so it shows whether or not the commands are going to run, which is
why `--dry-run` prints it too.

`executeProvisionStep`'s inline block was replaced by a `writeNote(os.Stdout, …)` call
rather than left as a fourth copy; the blank-line-either-side, four-space rendering is
preserved exactly, which is what the byte-identical diff confirms.

| | before | after |
| --- | --- | --- |
| `dva provision sequential` | note shown, 130 bytes | **identical, 130 bytes** |
| `dva provision parallelbatch` | label shown, note dropped (0) | note shown (1) |
| `dva provision parallelbatch --dry-run` | note dropped (0) | note shown (1) |
| `DVA_HOOK_DEPTH=1 dva build --mode nativemode` | note dropped (0) | note shown (1) |
| sibling `run:` control | 2 | **2 on every row** |

### Non-vacuity: three hunks, three probes

Reverting each hunk on its own failed exactly one test and left the rest green, so no test
is passing because of a different hunk:

| hunk reverted | failed | stayed green |
| --- | --- | --- |
| `writeNote(&buf, s.Note)` in `executeParallelBatch` | `TestParallelBatchPrintsNote` (both subtests) | the other 3 |
| `writeNote(os.Stdout, step.Note)` in `compose.go` | `TestNativeBuildLoopPrintsNote` | the other 3 |
| `writeNote(os.Stdout, step.Note)` in `executeProvisionStep` | `TestSequentialAndParallelNotesAgree` | the other 3 |

### The compose.go criterion was nearly a vacuous pass

`grep -c '\.Note' internal/cli/compose.go` went 0 → 1 as the criterion asked, and the first
runtime check appeared to confirm it — `dva build --mode nativemode` printed the note. It
was not this fix printing it. The note came out **two-space indented on stderr**, which is
`hooks.go`'s rendering, not `writeNote`'s four-space stdout one.

`wrapWithHooks` wraps `buildCmd` and its replace phase triggers on `len(ic.Replace) > 0` —
the identical condition as compose.go's native branch — so the wrapper handles the command
and never calls `original`. compose.go's loop is reachable only when the wrapper's recursion
guard trips, i.e. `dva build` invoked from inside a hook step. Setting `DVA_HOOK_DEPTH=1`
produced the four-space stdout note (count 1 on stdout vs 0 through the normal path), which
is the evidence that this fix's line is the one running.

Filed as [TASK-093](../todo/093-native-build-loop-is-shadowed-by-the-hook-wrapper.md) — the shadowing
and the two divergent renderings are a structural problem beyond this task's scope, which is
`executeParallelBatch` and the compose.go loop, not which of the two should exist.

### Found while fixing, not fixed here

`hooks.go:160-167` is a **fourth** copy of the note rendering, and it prints the note
*after* the step's commands rather than before, on stderr. Not folded into `writeNote` here
because this task's Left open section already scoped the stream question out; it is now
carried by TASK-093 alongside the shadowing.

`internal/cli/compose.go` is gofmt-dirty at HEAD (4 hunks, all far from this change) and was
left alone — [TASK-078](../todo/078-nine-files-do-not-satisfy-gofmt-and-nothing-checks.md) owns that decision.

## Left open

- **Note-before-run ordering** — now filed as
  [TASK-089](089-note-suppresses-run-on-the-interaction-path-only.md), with a correction to what
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
- [TASK-083](083-a-step-without-run-announces-work-it-never-does.md) — the fix whose call-site
  audit surfaced both.
