---
id: TASK-097
title: "`interactionUsage` treats any space in a key as subcommand nesting, so a legal interaction name containing a space gets a `usage_example` that fails when run"
type: fix
priority: P3
effort: M
status: todo
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/cli/list.go:123-136 — strings.Cut(k, \" \"); the flat key space built by internal/runner/interaction_tree.go:97"
---

# Task 097: two different key shapes share one string space, told apart by a guess

## Problem

`interactionUsage` decides what a key *is* by whether it contains a space:

```go
if parent, _, nested := strings.Cut(k, " "); nested {
    if config.IsReservedCommand(parent) {
        return fmt.Sprintf("dva run %s", k), parent
    }
    return fmt.Sprintf("dva %s", k), ""
}
```

`InteractionTree.List()` puts two unrelated shapes into that one flat space:

1. genuine top-level interaction names, and
2. composite keys `parent + " " + subName` synthesized by `expandInto`
   (`interaction_tree.go:97`).

A space is the only discriminator — but it is not one. `schema.json`'s interaction key pattern is
`^[\w\-.:/\s]+$`, which **includes `\s`**, and no Go-level validation rejects an embedded space.
So a declared name containing a space is legal input that gets classified as shape 2.

## Measured

```yaml
interaction:
  "my task":
    command: "echo hi"
```

| invocation | result |
| --- | --- |
| `dva manifest --format json` | `"my task": {"usage_example": "dva my task"}`, no `shadowed_by_builtin` |
| `dva my task` | `ERROR: unknown command "my" for "dva"`, exit 1 |
| `dva run "my task"` | `hi`, exit 0 |

The emitted `usage_example` is the one form that provably does not work. Bare routing
(`root.go:183-207`) looks up `args[0]` — `"my"` — which is not a key; `dva run` (`run.go:27-48`)
does `tree.Find("my", "task")`, which looks up `"my"` for the same reason. Only quoting the whole
name as one token reaches the interaction, and `interactionUsage` never emits that form.

Same wrong value reaches `dva ls` (`list.go:159`), which shares the call.

## Why this is not just a manifest bug

The function's own doc comment says it exists so `ls` and `manifest` agree. They do agree — on the
wrong answer. The root cause is the flat key encoding, which is also what
[TASK-095](095-third-level-subcommands-never-expand.md) trips over. Worth deciding whether to fix
the symptom here or to carry the nesting depth alongside the key so neither task has to guess.

## Options

- **A — carry the shape explicitly.** Have `List()` return keys with their path (or a `nested`
  flag) so no consumer has to re-derive it from the string. Fixes 095 and this together.
- **B — quote when the name is atomic.** Look the key up in `c.Interaction` first: an exact hit
  means a declared top-level name, so emit `dva run "<k>"`. Smaller, and does not touch 095.
- **C — reject spaces in interaction keys at validate time.** Narrows the schema; a breaking change
  for any config that uses one today.

## Acceptance criteria

- [ ] The emitted usage actually runs | verify: for a `"my task"` fixture, the string in `usage_example` must succeed when executed verbatim; print the exit code
- [ ] Nested keys are unaffected | verify: `dva manifest` on a subcommand fixture must still emit `dva <parent> <sub>`; print both entries
- [ ] `ls` and `manifest` still agree | verify: `go test ./internal/cli/ -run 'Usage|Manifest'` — print the number of tests selected
- [ ] Shadowing detection survives | verify: a reserved-name parent must still set `shadowed_by_builtin`
- [ ] Not vacuous | verify: human — revert the fix and confirm the new assertion fails on the space fixture
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-076](../done/076-manifest-advertises-the-one-invocation-that-cannot-reach-the-interaction.md)
  — **the direct predecessor, and not a duplicate.** 076 fixed this same function for the
  *reserved-name* case: `usage_example: "dva build"` when the built-in takes the bare form. Its
  fix landed in the branch below the `strings.Cut`, via `ShadowedByBuiltin`. A key containing a
  literal space never reaches that branch — it is caught by the `nested` test first and returned
  before any shadowing check runs. Same function, same symptom, different input class.
- [TASK-095](095-third-level-subcommands-never-expand.md) — the other defect in the same flat key
  space; option A fixes both.
- [TASK-096](096-manifest-static-commands-undercounts.md) — the other manifest-correctness defect.
