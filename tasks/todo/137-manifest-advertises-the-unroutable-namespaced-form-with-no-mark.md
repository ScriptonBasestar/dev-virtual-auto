---
id: TASK-137
title: "manifest advertises the unroutable namespaced form, and its machine-readable surfaces have no state for it"
type: bug
priority: P2
status: todo
effort: M
created-at: 2026-08-03T12:10:00+09:00
source: "TASK-076 finalize verification — the branch its non-goals excluded, untracked"
depends-on: [TASK-076]
scope: "dva repo — internal/cli/manifest.go, internal/config/reserved.go, internal/cli/ls.go"
---

# Task 137: Give the manifest a state for "unroutable", not just "shadowed"

## Problem

TASK-076 fixed the case where `dva manifest` advertised an invocation that reaches a
built-in instead of the declared interaction: reserved names now carry
`shadowed_by_builtin` and a `usage_example` that actually works.

An interaction whose key uses a **reserved name as a namespace prefix** was excluded by
that task's non-goals, and it is still advertised as if it worked. Reproduced live with
`bin/dva` v0.1.44 against a fixture declaring `interaction: {"app:build": {run: ...}}`:

```
dva manifest → "app:build": { "runner": "Local", "usage_example": "dva app:build" }
dva app:build       → rc=1  ERROR: unknown command "app:build" for "dva"
dva run app:build   → rc=1  ERROR: subproject `app` not found. Available:
```

Both forms fail. `usage_example` names one of them.

`ShadowedByBuiltin` is correctly false — nothing shadows this key, it is simply
unreachable — so the fix is not to reuse that field. The surfaces need a third state.

## What already works

`dva ls` and `dva validate` diagnose it precisely (`internal/config/reserved.go`):

> interaction command namespace prefix 'app' is a reserved DVA command — no invocation
> reaches this key: the bare form is not a built-in, and the run form reads 'app:' as a
> subproject reference, so it fails with subproject 'app' not found. Use a different
> separator (e.g., 'app-build')

So the detection exists and the human-facing text is good. Only the machine-readable
surface — the one an AI agent reads to decide what to run — still advertises the dead
form.

## Acceptance criteria

- [ ] `dva manifest` marks a namespace-prefixed reserved key with a state distinct from
      `shadowed_by_builtin` (e.g. `unroutable: "app"`), carrying the reason.
- [ ] `usage_example` for such a key names no invocation that exits non-zero — either it
      is omitted, or it names the working alternative the warning already suggests.
- [ ] `dva ls --json` exposes the same state as `manifest`.
- [ ] A test proves both invocation forms fail for the marked key, so the mark cannot
      drift away from the behaviour it describes.
- [ ] A non-reserved namespaced key (`build:fast` is reserved, `mytool:fast` is not) is
      left unmarked — the mark is not a blanket rule about colons.
- [ ] `make test` exits 0.

## Notes

TASK-076's own "Left open" section names this and calls it "this task's defect, in the
branch the non-goals excluded". A sweep on 2026-08-03 found no other task tracking it,
which is why this file exists.

Its two sibling left-opens are already handled elsewhere: USAGE.md's size is TASK-106,
and `validate` being fatal while `ls`/`manifest`/`run` proceed on the same config is
documented in USAGE.md as-is.
