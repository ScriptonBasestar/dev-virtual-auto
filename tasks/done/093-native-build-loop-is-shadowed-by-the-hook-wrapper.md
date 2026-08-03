---
id: TASK-093
title: "`compose.go`'s native build loop is shadowed by the hook wrapper — same trigger condition, different rendering, reachable only from inside a hook"
type: fix
priority: P3
effort: M
status: done
created-at: 2026-07-31T08:15:00+09:00
scope: "internal/cli/compose.go:457-481 (buildCmd native branch) vs internal/cli/hooks.go:63-64,91-170 (runHookSteps replace phase)"
---

# Task 093: two implementations, one trigger, and the wrapper always wins

## Problem

`buildCmd`'s native branch runs `interaction.build.replace` steps when:

```go
if ic, ok := c.Interaction["build"]; ok && len(ic.Replace) > 0 {   // compose.go:458
```

`wrapWithHooks` wraps `buildCmd` unconditionally (`root.go:120-130`) and its replace phase
fires on the *same* condition:

```go
if len(ic.Replace) > 0 {                                          // hooks.go:63
    return runHookSteps(e, c, "replace", cmdName, ic.Replace)     // ← original() never called
}
```

So whenever compose.go's branch would apply, the wrapper has already handled the command
and `original` — the RunE containing that branch — is not invoked. The only way in is the
wrapper's recursion guard (`hooks.go:34`): with `DVA_HOOK_DEPTH>0`, i.e. `dva build`
invoked from inside another hook step, the wrapper defers and compose.go's loop runs.

The two implementations do not agree, so which one you get changes the output.

## Evidence (measured on 0.1.44 after TASK-086)

One fixture, `modes.nativemode.build: native` plus a two-item `interaction.build.replace`
(a `note:`-only step and an `run: echo BUILD-CONTROL-RAN` sibling as the control):

| invocation | printed by | indent | stream | note position |
| --- | --- | --- | --- | --- |
| `dva build --mode nativemode` | `runHookSteps` | 2 spaces | **stderr** | **after** the commands |
| `DVA_HOOK_DEPTH=1 dva build --mode nativemode` | compose.go loop | 4 spaces | **stdout** | **before** the commands |

```
$ dva build --mode nativemode 2>/dev/null | grep -c BUILD-NOTE-VISIBLE
0                       # the note went to stderr — the wrapper's copy
$ DVA_HOOK_DEPTH=1 dva build --mode nativemode 2>/dev/null | grep -c BUILD-NOTE-VISIBLE
1                       # compose.go's copy, on stdout
```

`BUILD-CONTROL-RAN` appears twice on both paths, which is the control that rules out "one
of them simply did not run": both execute the batch, they only disagree about the message.

The stream split is the same one TASK-086 recorded in its Left open section — `hooks.go`
writes this class of message to stderr, `provision.go` to stdout — now with a case where
**one command** produces both answers depending on nesting depth.

## Why it is P3

Nothing executes wrongly and no exit code lies. It matters because a reader of
`compose.go:457-481` cannot tell that the code is shadowed, so a fix applied there (as
TASK-086's third call site was) does not change what users normally see. That is a
maintenance trap, not a runtime defect.

## Proposed fix

Decide which of the two is canonical rather than repairing both:

- **A — delete the compose.go branch** and let the wrapper own `replace` entirely. Smallest
  diff. Requires confirming the nested case genuinely wants hook semantics; the recursion
  guard exists to stop hooks re-entering, so running the *replace steps again* at depth may
  be the wrong behaviour anyway.
- **B — keep both, share the body.** `runHookSteps` and the compose.go loop are the same
  algorithm (label → inert → compose keys → shell → note) with different writers. Extract
  one executor taking an `io.Writer` and a label format, the way `writeNote` (TASK-086) did
  for the one piece they now share.

Either way, settle the stream question for hook messages — that decision is currently made
four times independently.

## Non-goals

- Not changing `writeNote` or the sequential provision rendering. TASK-086 established it as
  the reference and its output is pinned byte-for-byte by
  `TestSequentialAndParallelNotesAgree`.
- Not changing the recursion guard itself.

## Acceptance criteria

- [x] One code path handles `interaction.build.replace` | verify: `grep -c 'ic.Replace' internal/cli/compose.go internal/cli/hooks.go` — print both counts; under A compose.go must be 0
- [x] The normal invocation is unchanged | verify: `dva build --mode nativemode` on the fixture — `BUILD-CONTROL-RAN` count must stay 2 and the note must still appear on some stream
- [x] The nested invocation agrees with it | verify: `DVA_HOOK_DEPTH=1 dva build --mode nativemode` — the note must land on the same stream with the same indent as the line above, diff the two captures
- [x] Covered by a test | verify: `go test ./internal/cli/ -run 'TestNativeBuildLoopPrintsNote|TestHook'`
- [x] Full suite passes | verify: `make test`

Criterion 1's verify was written for option A and reads "compose.go must be 0". B was chosen, so
the honest count is **compose.go 3, hooks.go 2** — and all three compose.go lines are the handover,
not an implementation:

```
hooks.go:63    if len(ic.Replace) > 0 {
hooks.go:64        if err := runHookSteps(e, c, "replace", cmdName, ic.Replace); err != nil {
compose.go:464     // ...the same len(ic.Replace) > 0 condition...        ← comment
compose.go:473     if ic, ok := c.Interaction["build"]; ok && len(ic.Replace) > 0 {
compose.go:474         return runHookSteps(e, c, "replace", "build", ic.Replace)
```

The measure that answers what the criterion meant is `grep -c 'RunCommands()'`: **compose.go 0**,
hooks.go 1. There is one executor.

## Resolution

**Option B, by reuse rather than extraction.** The native branch's 34-line loop is gone; it now
calls `runHookSteps(e, c, "replace", "build", ic.Replace)` — the function the wrapper already uses.
Nothing was extracted because nothing needed to be: the wrapper's executor was already the more
complete of the two.

A was rejected on measurement. Deleting the branch outright leaves `case "native":` with only its
`return fmt.Errorf("mode %q build=native but no interaction.build.replace defined", mode)` — which
is false precisely when `replace` *is* defined, the only case that reaches it. A nested `dva build`
would fail with a message contradicted by the config in front of it.

### The part this task filed as cosmetic was not

The recorded evidence held exactly — the note lands on stderr via the wrapper and on stdout via
compose.go's copy. What the original measurement did not check is `--dry-run`:

| invocation, at `2e6b89f` | did the step execute? |
| --- | --- |
| `dva build --dry-run --mode nativemode` | no — described only |
| `DVA_HOOK_DEPTH=1 dva build --dry-run --mode nativemode` | **yes — the command ran** |

`consumeDryRunFlag` runs *before* the recursion guard in `wrapWithHooks`, so `dryRun` was correctly
set to true and then simply never read: compose.go's copy had no dry-run branch at all. Observed
with a `run: touch SIDE-EFFECT-HAPPENED` step, so "executed" is a file on disk rather than a
reading of the output. This is a dry run that writes, which is a runtime defect, not a maintenance
trap — the P3 in the frontmatter is left as filed but the rating was wrong.

### Measured after

| | stdout | stderr | note | control | dry-run honoured |
| --- | --- | --- | --- | --- | --- |
| `dva build --mode nativemode` | 17B | 206B | stderr | 2 | yes |
| `DVA_HOOK_DEPTH=1 dva build --mode nativemode` | 17B | 206B | stderr | 2 | yes |

The two stderr captures are **byte-identical** (206B), which is the criterion's "diff the two
captures" answered directly rather than by eye. Delegation also brings `compose_up`/`compose_exec`/
`compose_run` steps to the nested path — the deleted copy implemented none of them.

### Tests

- `internal/cli/native_build_delegation_test.go` (new) — `TestNativeBuildDelegatesToTheHookExecutor`
  asserts the hook executor's label reaches both paths, that nothing renders on stdout, and that
  the two stderr captures are equal; `TestNativeBuildHonoursDryRunWhenNested` asserts on the marker
  file, with the normal path as the control that was already correct.
- `TestNativeBuildLoopPrintsNote` (TASK-086's) rewritten, not deleted: same guarantee — a `note:`
  on the native build path is visible and the sibling step still runs — retargeted from stdout at
  four-space indent to stderr at two. Its name keeps "Loop" because two task files name it in
  their verify commands.

Against HEAD's `compose.go`, the new tests fail on exactly the nested rows (`nested (0B)` — the old
path wrote nothing to stderr) while every normal-path control passes, and the dry-run row fails
with the marker file present.

`make test` green; `internal/cli` coverage 63.3%.

## Left open

- The stream question is **not** settled, only reduced from four independent decisions to three:
  `hooks.go` writes hook messages to stderr, `provision.go` to stdout. Unifying them is a wider
  change than this task — `writeNote` indents four spaces and `runHookSteps` two, so routing hook
  notes through `writeNote` would visibly reindent every hook in every config, and this task's
  non-goals forbid touching `writeNote`. Still open as
  [TASK-088](../_archive/088-validate-json-covers-only-the-failure-it-does-not-produce.md)'s and
  TASK-086's shared observation.
- A failing step's error message changed from `native build failed: %w` to
  `hook replace:build step '<label>' failed: %w`. Strictly more informative — it names the step —
  but it is a user-visible string that nothing pinned.

## Related

- [TASK-086](../_archive/086-parallel-steps-discard-their-note.md) — found while measuring its third
  call site. Its `grep -c '.Note' internal/cli/compose.go` criterion passed, but the runtime
  evidence for it had to come from the `DVA_HOOK_DEPTH=1` path, which is what exposed the
  shadowing.
- [TASK-088](../_archive/088-validate-json-covers-only-the-failure-it-does-not-produce.md) — also carries the
  `hooks.go` stderr vs `provision.go` stdout observation.
