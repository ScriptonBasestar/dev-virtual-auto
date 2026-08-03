---
id: TASK-097
title: "`interactionUsage` treats any space in a key as subcommand nesting, so a legal interaction name containing a space gets a `usage_example` that fails when run"
type: fix
priority: P3
effort: M
status: done
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/cli/list.go:123-136 — strings.Cut(k, \" \"); the flat key space built by internal/runner/interaction_tree.go:97"
verified-at: 2026-08-03T13:40:00+09:00
archived-at: 2026-08-03T13:40:00+09:00
verification-summary: |
  Deliverables exist and behave as claimed. internal/runner/interaction_tree.go:17 carries
  `Path []string`, set at :155 by expandInto; internal/cli/list.go:145 interactionUsage takes
  *ResolvedCommand and reads cmd.Path — no strings.Cut on the key remains anywhere in list.go
  or manifest.go. Live fixture through ./bin/dva (v0.1.44): all 7 emitted usage_examples run
  at exit=0 and each prints its own marker, including `dva 'my task'` and `dva 'my task' sub`,
  the two rows that exited 1 before. `go test ./internal/cli/ -run 'Usage|Manifest'` selects
  14 real tests (verified by name), ok. Non-vacuity re-derived from scratch on a scratchpad
  copy of HEAD with interactionUsage reverted: the two reachability rows and three quoting rows
  fail, both control tests pass. Corpus re-measured today: 17 example configs, 15 declaring
  dynamic commands, 77 dynamic commands, 0 usage_examples acquiring a quote — the change is
  inert on shipped examples. Repo left untouched (`git status --porcelain` empty).
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
[TASK-095](../_archive/095-third-level-subcommands-never-expand.md) trips over. Worth deciding whether to fix
the symptom here or to carry the nesting depth alongside the key so neither task has to guess.

## Options

- **A — carry the shape explicitly.** Have `List()` return keys with their path (or a `nested`
  flag) so no consumer has to re-derive it from the string. Fixes 095 and this together.
- **B — quote when the name is atomic.** Look the key up in `c.Interaction` first: an exact hit
  means a declared top-level name, so emit `dva run "<k>"`. Smaller, and does not touch 095.
- **C — reject spaces in interaction keys at validate time.** Narrows the schema; a breaking change
  for any config that uses one today.

## Acceptance criteria

- [x] The emitted usage actually runs | verify: for a `"my task"` fixture, the string in `usage_example` must succeed when executed verbatim; print the exit code
- [x] Nested keys are unaffected | verify: `dva manifest` on a subcommand fixture must still emit `dva <parent> <sub>`; print both entries
- [x] `ls` and `manifest` still agree | verify: `go test ./internal/cli/ -run 'Usage|Manifest'` — print the number of tests selected
- [x] Shadowing detection survives | verify: a reserved-name parent must still set `shadowed_by_builtin`
- [x] Not vacuous | verify: human — revert the fix and confirm the new assertion fails on the space fixture
- [x] Full suite passes | verify: `make test`

```
1  usage_example = dva 'my task'   exit=0  marker=RAN-MY-TASK
2  rails console -> dva rails console      build fast -> dva run build fast     (both unchanged)
3  go test ./internal/cli/ -run 'Usage|Manifest'  -> 14 tests selected, ok
4  build -> shadowed_by_builtin=build;  build fast -> shadowed_by_builtin=build
5  HEAD + the new tests: 2 rows fail as unreachable, 4 control rows pass (table below)
6  make test -> 0 FAIL; cli 63.4%, runner 52.3%
```

Criterion 2 is ticked with one honest exception. `rails console` and `build fast` are byte-identical
before and after, but a subcommand whose *own* name holds a space now renders quoted:
`rails db migrate` moved from `dva rails db migrate` to `dva rails 'db migrate'`. Both forms run —
`Find` re-joins its arguments with a space, so the unquoted form was never broken — so this is a
rendering change, not a fix. It is the same rendering rule applied consistently, and it shows the
reader where the segment boundary actually is.

## Decision: A

**A — carry the shape explicitly**, because B is provably incomplete. B ("look the key up in
`c.Interaction`; an exact hit means a declared top-level name") repairs `my task` and cannot
repair `my task sub`: that composite key is never in `c.Interaction`, so the lookup misses and the
old space-cut still runs. Measured on a fixture with both:

| key | advertised before | ran? | advertised after | ran? |
| --- | --- | --- | --- | --- |
| `my task` | `dva my task` | **exit 1** | `dva 'my task'` | exit 0 |
| `my task sub` | `dva my task sub` | **exit 1** | `dva 'my task' sub` | exit 0 |

Half a fix on a defect whose whole content is "the emitted string does not work" is worse than
none — it moves the failure to the less obvious row.

C was rejected as a breaking change to a schema pattern that has admitted `\s` since it was
written, for an input class no shipped example uses; nothing is gained by making legal configs
illegal when the renderer can simply be correct.

The deciding argument for A over a smarter derivation is [TASK-101](101-default-args-inherit-into-subcommands-that-replace-the-command.md)'s:
a consumer that re-derives what a producer already knows is a second source of truth, and the two
drift. `expandInto` knows the segments exactly — it is the code that joins them — so it now hands
them over rather than making the reader guess them back.

## Resolution

Three changes.

1. `ResolvedCommand` gains `Path []string`, and `expandInto` takes the path instead of the joined
   name (`internal/runner/interaction_tree.go`). `Name` is unchanged — still the space-joined form,
   still the map key — so nothing that reads it moved.
2. `interactionUsage` takes the `*ResolvedCommand` instead of the key string and reads `Path`.
   Both call sites already had the command in hand (`list.go` and `manifest.go` each do
   `cmd := commands[k]` on the line above), so no caller had to look anything up.
3. `shellJoin` quotes a segment only when it contains whitespace. Every other character the key
   pattern admits — `\w`, `-`, `.`, `:`, `/` — is inert to a shell, and quoting all of them would
   turn every example `dva ls` prints into a literal.

The old `strings.Cut(k, " ")` is gone; nothing re-derives the segment boundary any more.

### Not vacuous

HEAD `8ae8da5` plus the new tests, unmodified (`git archive HEAD | tar -x`, test files copied in):

```
--- FAIL: TestEveryAdvertisedUsageReachesItsOwnEntry
    "my task" advertises "dva my task", which reaches nothing
    "my task sub" advertises "dva my task sub", which reaches nothing
--- FAIL: TestUsageQuotesOnlyWhatNeedsIt          (3 rows)
--- PASS: TestShadowingSurvivesTheKeyChange       ← control, must not move
--- PASS: TestLsAndManifestStillAgree             ← control, must not move
```

The runner tests do not fail at HEAD, they do not compile — `cmd.Path undefined` — which is the
honest reading for a test of a field that did not exist.

### Blast radius

Every `examples/*.yml` rendered through both binaries and compared: **19 files scanned, 89 dynamic
commands compared across the 17 that declare any, 0 differing**. No shipped example has a space in
an interaction key (count: 0), so the change is inert on the whole corpus — it only moves output
for the input class that was broken.

### Tests

- `internal/cli/interaction_key_space_test.go` — `TestEveryAdvertisedUsageReachesItsOwnEntry` does
  not compare strings: it splits each emitted `usage_example` the way a shell would and replays
  `root.go`'s routing plus `Find`, then asserts the resolved command is *the same key* the example
  was printed under. Reaching some other command reads as a failure, which a substring assertion
  would have missed. `TestUsageQuotesOnlyWhatNeedsIt` pins the rendering separately, because the
  reachability test also passes for over-quoted forms.
- `internal/runner/interaction_path_test.go` — `Path` is exact at depths 1–3 including a segment
  containing a space, and `TestSiblingPathsDoNotAlias` guards the `append` trap: three children of
  one parent, run with `-count=5`, so a shared backing array would show up as an intermittent
  failure rather than never.

## Left open

- A declared key that spells another key's composite path silently deletes one of the two
  commands, nondeterministically — filed as
  [TASK-104](../done/104-a-literal-key-that-spells-a-composite-key-deletes-one-command.md) with the
  20-run measurement. Found while probing this fix; `Path` is the structure its option B would
  key on.
- `gofmt` runs the doc comment formatter, which reads a pair of straight single quotes as legacy
  Go doc syntax for a closing typographic quote. A comment documenting the `'\''` shell escape is
  silently rewritten by `make fmt`. Cost one round trip here; worth knowing before someone
  documents shell quoting again.
- `internal/runner/interaction_tree.go` still fails `gofmt` — it was already in
  [TASK-078](../_archive/078-nine-files-do-not-satisfy-gofmt-and-nothing-checks.md)'s set at HEAD (the struct
  comment alignment, untouched by this change), so it is left for that task rather than mixed in.

## Related

- [TASK-076](../_archive/076-manifest-advertises-the-one-invocation-that-cannot-reach-the-interaction.md)
  — **the direct predecessor, and not a duplicate.** 076 fixed this same function for the
  *reserved-name* case: `usage_example: "dva build"` when the built-in takes the bare form. Its
  fix landed in the branch below the `strings.Cut`, via `ShadowedByBuiltin`. A key containing a
  literal space never reaches that branch — it is caught by the `nested` test first and returned
  before any shadowing check runs. Same function, same symptom, different input class.
- [TASK-095](../_archive/095-third-level-subcommands-never-expand.md) — the other defect in the same flat key
  space; option A fixes both.
- [TASK-096](../_archive/096-manifest-static-commands-undercounts.md) — the other manifest-correctness defect.
