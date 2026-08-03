---
id: TASK-167
title: "A colon key whose prefix names no subproject is unreachable, and every surface says it is fine"
type: bug
priority: P2
status: todo
effort: M
created-at: 2026-08-03T15:55:00+09:00
source: "TASK-137 implementation — measuring its criterion-5 control case showed the control is broken too"
depends-on: [TASK-137]
scope: "dva repo — internal/cli/run.go:31-37 (the unconditional colon split), internal/config/validate_warnings.go, internal/cli/list.go interactionUsage"
---

# Task 167: Route or reject a colon key whose prefix names no subproject

## Problem

`run.go:31` splits **every** interaction key on `:` and reads the prefix as a subproject
reference, before ever asking whether the literal key exists:

```go
if parts := strings.SplitN(cmdName, ":", 2); len(parts) == 2 {
    resolvedProject = parts[0]
    cmdName = parts[1]
}
```

So a key like `mytool:fast`, where `mytool` is not a reserved command and not a declared
subproject, is reachable by nothing. Measured against `bin/dva` with a fixture declaring
only `interaction: {"mytool:fast": {command: echo free}}`:

| surface | result |
|---|---|
| `dva config validate` | `✅ dva.yml is valid` — **rc 0, no warning** |
| `dva ls` | `mytool:fast`, unmarked |
| `dva manifest --json` | `"usage_example": "dva mytool:fast"` |
| `dva mytool:fast` | rc=1 ``ERROR: subproject `mytool` not found. Available:`` |
| `dva run mytool:fast` | rc=1 ``ERROR: subproject `mytool` not found. Available:`` |

This is [TASK-137](../done/137-manifest-advertises-the-unroutable-namespaced-form-with-no-mark.md)
one branch over, and worse on the diagnosis side: there the prefix was a reserved command,
so `ValidateReservedCommands` rejected the config and `ConflictAdvice` explained it. Here
nothing objects at all. The author gets a green validate and a `usage_example` for a
command that cannot run.

TASK-137 discovered this by measuring its own criterion-5 control case, deliberately left
the mark keyed to the reserved prefix — `unroutable_reason` says *"prefix is a reserved DVA
command"*, which would be a false statement about `mytool` — and filed this instead.
`internal/config/reserved.go:93` and `internal/cli/unroutable_namespace_test.go:87` already
point a reader here.

## The decision this task has to make first

Two answers, and they are not both available:

1. **Route it.** Look the literal key up in `c.Interaction` *before* splitting, and only
   fall back to the subproject reading when no such key exists. `mytool:fast` starts
   working; the namespace syntax keeps working for real subprojects.
2. **Reject it.** Leave routing alone and add a semantic warning (and/or a validate error)
   for any colon key whose prefix names neither a subproject nor a reserved command, with
   the same rename advice `ConflictAdvice` gives.

Option 1 has a consequence that must be faced head-on, not discovered later: it would make
`app:build` reachable too, which turns TASK-137's `unroutable` mark into a lie. That is
detected, not silent — `TestUnroutableKeyFailsBothInvocationForms` drives `runCmd.RunE`
with the key *unsplit*, so it fails the moment the split at `run.go:33` starts routing the
literal key. (It called `runSubprojectCommand("app", "build")` with pre-split arguments
until TASK-137's review; in that form it would have passed straight through option 1 and
this citation would have been worthless.) But the fix would then have to also decide what the reserved
prefix means once the colon no longer implies a subproject. Option 2 costs nothing to
TASK-137 and closes the silent-failure hole, but it permanently reserves the colon for
subprojects even though nothing else in the config forces that.

Whichever is chosen, record why the other was not.

## The fixture set this has to cover

Found while reviewing TASK-137, all measured against `bin/dva`:

| key | validate | `dva <key>` | note |
|---|---|---|---|
| `mytool:fast` | rc 0, silent | rc 1, ``subproject `mytool` not found`` | the headline case |
| `:build` | rc 0, silent | rc 1, ``command `build` not recognized`` | leading colon — `schema.json`'s interaction-key pattern permits it, and `UnroutableNamespacePrefix` guards with `idx <= 0`, so nothing covers it. The empty prefix mangles `cmdName` in `run.go` rather than producing a subproject lookup, so the error text differs — the same family, a different message to get right. |
| `app-sub:cmd` | rc 0, silent | rc 1, ``subproject `app-sub` not found`` | reachable from the reserved case: it is what the pre-fix `ConflictAdvice` told authors of `app:sub:cmd` to rename to. TASK-137 fixed the advice (`RenameSuggestion` now drops every colon), so this arrives only if an author writes it directly. |

The `:build` row is the reason the criterion below says "names the key" rather than "names
the prefix" — there is no prefix to name.

## Acceptance criteria

- [ ] The decision above is recorded in this file with its reasoning before any code
      changes.
- [ ] A colon key whose prefix names no subproject either runs, or draws a diagnostic that
      names the key and the way out. It does not stay silently green.
- [ ] `dva manifest` and `dva ls --json` agree with whatever routing does — if the key
      cannot run, `usage_example` is omitted the way TASK-137 omits it; if it can, it is
      present and correct.
- [ ] A declared subproject's namespace syntax still routes. `dva run engine:test` against
      a config with `subprojects: {engine: ...}` must be unaffected — this is the case
      `run.go:31` was written for.
- [ ] Prove the gate fails on reverted code | verify: revert the change, run the new test,
      paste the failure.
- [ ] TASK-137's four tests in `internal/cli/unroutable_namespace_test.go` still pass, or
      are updated together with an explanation of why the mark's meaning changed.
- [ ] `make test` and `make lint` exit 0.

## Related

- [TASK-137](../done/137-manifest-advertises-the-unroutable-namespaced-form-with-no-mark.md)
  — the reserved-prefix branch of the same routing behaviour; the source of this file.
- [TASK-165](165-a-leaf-interaction-with-nothing-to-run-draws-no-warning-and-exits-0.md) —
  another declared-but-unrunnable node that the validator does not mention. Same family:
  the config parses, the surfaces list it, nothing runs.
