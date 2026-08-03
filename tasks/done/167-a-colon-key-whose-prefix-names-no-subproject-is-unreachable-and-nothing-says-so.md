---
id: TASK-167
title: "A colon key whose prefix names no subproject is unreachable, and every surface says it is fine"
type: bug
priority: P2
status: done
effort: M
created-at: 2026-08-03T15:55:00+09:00
source: "TASK-137 implementation — measuring its criterion-5 control case showed the control is broken too"
depends-on: [TASK-137]
scope: "dva repo — internal/cli/run.go (the colon split), internal/config/{reserved,validate_warnings}.go"
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

## Decision: route it, except behind a reserved prefix

**Chosen: option 1, narrowed.** `run.go` looks the literal key up in `c.Interaction` before
splitting, and skips the split when that key exists — *unless* its prefix is a reserved
command, in which case the split proceeds and the key stays unroutable exactly as today.

Measured first, against `bin/dva`, because the task file's table understates the problem:

| key | `dva config validate` | `dva <key>` |
|---|---|---|
| `mytool:fast` | rc 0, silent | rc 1, ``subproject `mytool` not found`` |
| `:build` | rc 0, silent | rc 1, ``command `build` not recognized`` |
| `app-sub:cmd` | rc 0, silent | rc 1, ``subproject `app-sub` not found`` |
| `app:build` | **rc 1**, reserved conflict + advice | rc 1, ``subproject `app` not found`` |
| *control:* `engine:test` w/ `subprojects.engine` | rc 0 | rc 0, runs |

The last-but-one row is the one that decided it. At **runtime** the reserved case is not
handled specially at all — `dva app:build` dies with the same generic
``subproject `app` not found`` as `mytool:fast`. The reserved/free distinction exists only
in `validate`. So "the reserved case is already handled" was never true of routing; it was
true of diagnosis.

### Why not option 2 (warn and leave routing alone)

Because DVA's own advice generates the keys it would warn about. `RenameSuggestion`
(`reserved.go:116`) tells the author of `app:sub:cmd` to write `app-sub:cmd` — and
`app-sub:cmd` is row three of that table, silently unroutable. Under option 2 the tool
would answer a hard error with a rename to a form it then warns about. The comment at
`reserved.go:107-112` already records that this happened once and was measured.

Option 2 also permanently reserves `:` for subprojects on the strength of one function's
unconditional `SplitN`. Nothing in the schema, the docs, or the config model says the colon
belongs to subprojects; `schema.json` accepts these keys, `dva ls` lists them, and
`dva manifest` prints `usage_example: dva mytool:fast`. Every surface but routing already
treats them as ordinary commands. Option 1 makes routing agree with the surfaces rather than
making four surfaces agree with routing.

### Why the reserved prefix stays excepted

Plain option 1 would make `app:build` run. `dva config validate` **rejects** that config with
rc 1 today, so routing it would ship a config that validate calls a hard error and the runtime
executes happily — trading a uniform failure for a disagreement between two surfaces, which is
the TASK-137 shape this task exists to stop repeating.

The exception also keeps `unroutable_reason` honest. It reads *"prefix is a reserved DVA
command"*, which stays a true and complete description of what is now the only unroutable
class. TASK-137's four tests in `unroutable_namespace_test.go` need no change, and
`TestUnroutableKeyFailsBothInvocationForms` — which drives `runCmd.RunE` with the key unsplit
— is what will catch it if the exception is ever dropped.

### The ambiguity this introduces

Literal-first means a parent declaring `interaction: {"engine:test": ...}` *and*
`subprojects: {engine: ...}` now runs its own command, and the subproject's `engine:test`
becomes unreachable through that spelling. Nothing detects that today because it cannot happen
today. A warning is part of this task's scope, not a follow-up: shipping the routing change
without it would create a fresh silent shadowing while closing another.

## The fixture set this has to cover

Found while reviewing TASK-137, all measured against `bin/dva`:

| key | validate | `dva <key>` | note |
|---|---|---|---|
| `mytool:fast` | rc 0, silent | rc 1, ``subproject `mytool` not found`` | the headline case |
| `:build` | rc 0, silent | rc 1, ``command `build` not recognized`` | leading colon — `schema.json`'s interaction-key pattern permits it, and `UnroutableNamespacePrefix` guards with `idx <= 0`, so nothing covers it. The empty prefix mangles `cmdName` in `run.go` rather than producing a subproject lookup, so the error text differs — the same family, a different message to get right. |
| `app-sub:cmd` | rc 0, silent | rc 1, ``subproject `app-sub` not found`` | reachable from the reserved case: it is what the pre-fix `ConflictAdvice` told authors of `app:sub:cmd` to rename to. TASK-137 fixed the advice (`RenameSuggestion` now drops every colon), so this arrives only if an author writes it directly. |

The `:build` row is the reason the criterion below says "names the key" rather than "names
the prefix" — there is no prefix to name.

## Result

`config.LiteralKeyWins` (`reserved.go:129`) answers the routing question in one place, and
`run.go:32` consults it before splitting. Measured against `bin/dva`:

| key | before | after |
|---|---|---|
| `mytool:fast` | rc 1, ``subproject `mytool` not found`` | **rc 0**, runs |
| `:build` | rc 1, ``command `build` not recognized`` | **rc 0**, runs |
| `app-sub:cmd` | rc 1, ``subproject `app-sub` not found`` | **rc 0**, runs |
| `app:build` | rc 1, ``subproject `app` not found`` | rc 1, unchanged |
| *control:* `engine:test` w/ `subprojects.engine` | rc 0, runs child | rc 0, runs child |

`warnLiteralKeyShadowsSubproject` covers the ambiguity the change introduces, and only that
shape: a colon key whose prefix *is* a declared subproject. It stays silent on the ordinary
case (`mytool:fast`, prefix naming no subproject), which is now simply a working command —
warning there would tell authors their correct config is suspect.

### The escape hatch in the warning was wrong until it was executed

The message first read ``use `dva --project engine test` to reach the subproject``. Run
against the binary:

```
dva --project engine test        rc=1   ERROR: unknown command "test" for "dva"
dva run --project engine test    rc=0   ran-CHILD-subproject
```

`--project` is registered on `runCmd`, so it only parses after an explicit `run`; the bare
form's rewrite in `cli.Execute` does not look past a leading flag. The shorter spelling reads
better and does not work. `ConflictAdvice`'s comment already sets the bar this violated —
*"advice that names a command which refuses is worse than no advice — the reader trusts it
and stops"* — so the fix is the message, and the test compares the warning **whole** rather
than by substring, because a `Contains` check on the first clause passes while the advice
rots.

### Surfaces needed no change

Criterion 3 turned out to be already satisfied, in the direction that matters:
`dva manifest --json` printed `usage_example: dva mytool:fast` all along, with no
`unroutable` mark. It was not wrong about the key — routing was. After the change the
manifest's claim is simply true. The reserved fixture still carries `unroutable` and its
`unroutable_reason`, and still omits `usage_example`.

## Tests

Five, in two packages.

`internal/cli/literal_colon_key_test.go` drives `runCmd.RunE` with the key **unsplit** — the
same entry point TASK-137 uses, and for the same reason: the colon has to be split (or not)
by the code under test.

Every fixture there declares `shell: false` and points `command:` at a sentinel binary that
cannot exist. Both halves are load-bearing, and finding out why cost a run:

- `ExecReplace` panics under `go test` unless it is crossing a subprocess boundary
  (TASK-144) — `syscall.Exec` would replace the test binary and report the package `ok`
  whatever ran. The guard sits *after* `exec.LookPath` on purpose, so a command that cannot
  resolve stops at `command not found` instead. That is the seam these tests use.
- `ShellEnabled()` defaults to **true**, so `command: echo x` becomes `sh -c "echo x"`; `sh`
  resolves, execution reaches the guard, and the first draft panicked on every case. With
  `shell: false` the sentinel is argv[0] itself.

The sentinel is distinct per key, so the error text names *which* interaction resolved. That
is the whole claim under test — an assertion that stopped at "no error", or even at "some
error", would pass against a `run.go` that resolved the key and then ran something else.

Falsified by deleting `!config.LiteralKeyWins(c, cmdName)` from `run.go:32`:

```
--- FAIL: TestFreePrefixColonKeyRunsTheLiteralKey/free_prefix
    literal_colon_key_test.go:106: `dva run mytool:fast` failed with "subproject `mytool` not found. Available: "; want an error naming "dva-resolved-mytool-fast", which is the only proof the literal key resolved rather than being split into a subproject reference
--- FAIL: TestFreePrefixColonKeyRunsTheLiteralKey/leading_colon,_no_prefix_at_all
    literal_colon_key_test.go:106: `dva run :build` failed with "command `build` not recognized! Run 'dva ls' to see available commands"; want an error naming "dva-resolved-leading-colon", ...
--- FAIL: TestFreePrefixColonKeyRunsTheLiteralKey/the_form_RenameSuggestion_produces
    literal_colon_key_test.go:106: `dva run app-sub:cmd` failed with "subproject `app-sub` not found. Available: "; want an error naming "dva-resolved-app-sub-cmd", ...
```

Line 106 is the sentinel assertion, not the `err == nil` guard — the check that carries the
claim is the one that fires, which is what TASK-166 learned to verify rather than assume. All
three pre-fix failure modes are visible in the output, including `:build`'s distinct one.

The reserved test and the subproject control both still pass under that revert. Correct:
neither depends on the predicate, and the control's job is to show the change did *not* touch
the path `run.go:31` was written for.

## Acceptance criteria

- [x] The decision above is recorded in this file with its reasoning before any code
      changes.
- [x] A colon key whose prefix names no subproject either runs, or draws a diagnostic that
      names the key and the way out. It does not stay silently green. — it runs; measured
      rc 0 for all three shapes.
- [x] `dva manifest` and `dva ls --json` agree with whatever routing does — if the key
      cannot run, `usage_example` is omitted the way TASK-137 omits it; if it can, it is
      present and correct. — already true; see "Surfaces needed no change".
- [x] A declared subproject's namespace syntax still routes. `dva run engine:test` against
      a config with `subprojects: {engine: ...}` must be unaffected — this is the case
      `run.go:31` was written for. | verify: `go test ./internal/cli/ -run TestSubprojectNamespaceStillRoutes`
- [x] Prove the gate fails on reverted code | verify: revert the change, run the new test,
      paste the failure. — pasted above.
- [x] TASK-137's four tests in `internal/cli/unroutable_namespace_test.go` still pass, or
      are updated together with an explanation of why the mark's meaning changed. — pass
      unchanged; the mark's meaning narrowed from "a subset of the unroutable keys" to "the
      whole class", which `UnroutableNamespacePrefix`'s comment now records.
- [x] `make test` and `make lint` exit 0. — `make test` green; lint via `go vet ./...` +
      `gofmt -l` (both clean) and `make doc-check` OK. `make lint`'s golangci step is
      blocked by the GOTOOLCHAIN drift noted across this queue, not by this change.

## Related

- [TASK-137](../done/137-manifest-advertises-the-unroutable-namespaced-form-with-no-mark.md)
  — the reserved-prefix branch of the same routing behaviour; the source of this file.
- [TASK-165](165-a-leaf-interaction-with-nothing-to-run-draws-no-warning-and-exits-0.md) —
  another declared-but-unrunnable node that the validator does not mention. Same family:
  the config parses, the surfaces list it, nothing runs.
