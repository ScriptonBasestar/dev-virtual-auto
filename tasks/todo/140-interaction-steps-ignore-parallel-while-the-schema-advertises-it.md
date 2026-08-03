---
id: TASK-140
title: "An interaction step marked parallel runs sequentially, and validate calls the config valid"
type: bug
priority: P2
status: todo
effort: M
created-at: 2026-08-03T13:00:00+09:00
source: "TASK-085 finalize verification — the sixth ProvisionItem key, same shape as the five 085 fixed"
depends-on: [TASK-085]
scope: "dva repo — internal/runner/steps.go, internal/config/schema.json"
---

# Task 140: Honour `parallel:` on the interaction path, or refuse it

## Problem

`parallel: true` on an interaction step does nothing. The key parses, validates, and is
then dropped on the floor — the same silent-discard shape TASK-085 fixed for `compose_up`,
`compose_exec`, `compose_run`, `cmd:` and `echo:`.

`Parallel` appears **zero** times in non-test code under `internal/runner/`
(`grep -rn Parallel internal/runner --include='*.go' | grep -v _test.go` → 0), so
`runStepLoop` (`internal/runner/steps.go:20`) has no batching at all. The provision path
does have it: `internal/cli/provision.go:54` `groupParallelBatches(steps)` and `:67`
`executeParallelBatch(...)`.

Measured 2026-08-03 with `bin/dva` v0.1.44, two `sleep 1` steps both marked
`parallel: true`:

```
$ dva run par             # interaction path
  → a
  → b
real 2.02                 # sequential

$ dva provision default   # provision path, same two steps
✅ Provision complete!
real 1.01                 # concurrent
```

Nothing warns. On a config whose only content is that interaction:

```
$ dva validate
✅ dva.yml is valid
rc=0
```

…while `internal/config/schema.json:330-333` actively advertises the key:

```json
"parallel": {
  "type": "boolean",
  "description": "Run this step concurrently with other consecutive parallel steps",
  "default": false
}
```

So the schema promises concurrency, `validate` confirms the file is good, and the runner
takes twice as long as advertised. A user timing a slow interaction has no way to learn
the key was ignored.

## Why this survived TASK-085

TASK-085 enumerated the keys an interaction step drops and fixed five of them by adding
`hasStepKeys`/`runComposeStepKeys` (`internal/runner/step_keys.go:29,42`). `parallel:` is
not a *payload* key — it does not make a step do work, it changes how steps are
scheduled — so it fell outside the guard `len(cmds) == 0 && !hasStepKeys(step)`
(`internal/runner/steps.go:52`) that closes the other five. A sweep of `tasks/todo/`,
`tasks/blocked/` and `tasks/plan/` on 2026-08-03 found 0 files mentioning `parallel`.

## Acceptance criteria

- [ ] Pick a direction and record why: (A) implement batching in `runStepLoop`, reusing
      `groupParallelBatches`/`executeParallelBatch` rather than writing a third scheduler,
      or (B) reject the key on the interaction path so `validate` fails loudly.
- [ ] Under A: the fixture above measures ~1s, not ~2s, for `dva run par` — state the
      measured wall-clock for both paths side by side.
- [ ] Under B: `dva validate` exits non-zero on an interaction step carrying `parallel:`,
      and `schema.json`'s description says where the key is honoured.
- [ ] Whichever direction: output interleaving is handled the way `executeParallelBatch`
      handles it (per-step `bytes.Buffer`), so concurrent steps cannot shred each other's
      lines — TASK-086 already paid for that lesson.
- [ ] A test fails without the change, and its `-run` pattern is proven to match a real
      test name (an unanchored pattern matching zero tests still exits 0).
- [ ] `make test` exits 0.

## Notes

Check the remaining `ProvisionItem` fields against what `runStepLoop` consumes before
fixing this one. This sweep found `parallel` by timing one path, not by diffing the struct
against the runner — a field-by-field comparison is the cheap way to learn whether it is
the last one.
