---
id: TASK-093
title: "`compose.go`'s native build loop is shadowed by the hook wrapper — same trigger condition, different rendering, reachable only from inside a hook"
type: fix
priority: P3
effort: M
status: todo
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

- [ ] One code path handles `interaction.build.replace` | verify: `grep -c 'ic.Replace' internal/cli/compose.go internal/cli/hooks.go` — print both counts; under A compose.go must be 0
- [ ] The normal invocation is unchanged | verify: `dva build --mode nativemode` on the fixture — `BUILD-CONTROL-RAN` count must stay 2 and the note must still appear on some stream
- [ ] The nested invocation agrees with it | verify: `DVA_HOOK_DEPTH=1 dva build --mode nativemode` — the note must land on the same stream with the same indent as the line above, diff the two captures
- [ ] Covered by a test | verify: `go test ./internal/cli/ -run 'TestNativeBuildLoopPrintsNote|TestHook'`
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-086](../done/086-parallel-steps-discard-their-note.md) — found while measuring its third
  call site. Its `grep -c '.Note' internal/cli/compose.go` criterion passed, but the runtime
  evidence for it had to come from the `DVA_HOOK_DEPTH=1` path, which is what exposed the
  shadowing.
- [TASK-088](088-validate-json-covers-only-the-failure-it-does-not-produce.md) — also carries the
  `hooks.go` stderr vs `provision.go` stdout observation.
