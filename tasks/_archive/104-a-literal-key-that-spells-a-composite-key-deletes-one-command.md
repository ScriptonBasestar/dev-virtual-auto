---
id: TASK-104
title: "A declared interaction key that spells a composite key silently deletes one of the two commands — and when both live under one parent, `dva run` executes a different one between runs"
type: fix
priority: P2
effort: M
status: done
created-at: 2026-07-31T11:05:00+09:00
completed-at: 2026-07-31T11:20:00+09:00
scope: "internal/runner/interaction_tree.go — expandInto writes result[name] unconditionally; reached by both List (listing) and expand (execution, via Find). internal/cli/validate.go — the warning channel that reports it."
verified-at: 2026-08-03T13:55:00+09:00
archived-at: 2026-08-03T13:55:00+09:00
verification-summary: |
  Re-measured against bin/dva v0.1.44 with fresh fixtures in scratchpad, not against the numbers in the task.
  Determinism: intra-entry `run a b c --explain` 20× → 1 distinct Command: line; `manifest` 30× → 1 distinct
  .dynamic_commands on both intra- and cross-entry fixtures. Report: `dva validate` prints the collision naming
  both declarations by their dva.yml paths plus the shared key, rc=0 plain / rc=1 under --strict.
  Counts: intra 4 declared → 3 commands + 1 collision; cross 3 → 2 + 1. Corpus: 19 example files, 18 manifests,
  89 dynamic commands, 0 interaction warnings, 0 space-containing interaction keys (so the corpus cannot regress).
  Non-vacuity re-run mechanically via `go test -overlay` on two modified copies of interaction_tree.go: reverting
  the sort fails only the determinism tests, gutting the report fails only the reporting tests — independent, as claimed.
  Test tallies taken with -v (TASK-144 hazard): runner 47 PASS/0 FAIL, cli 392 PASS/0 FAIL.
---

# Task 104: one map, two key shapes, last writer wins

## Problem

`List()` builds `map[string]*ResolvedCommand` keyed by the space-joined path. Two unrelated
declarations can produce the same string:

```yaml
interaction:
  "rails console":          # a declared top-level key that happens to contain a space
    command: "echo RAN-LITERAL-TOPLEVEL"
  rails:
    command: "echo RAN-RAILS"
    subcommands:
      console:              # expands to the key "rails console"
        command: "echo RAN-EXPANDED-SUB"
```

`expandInto` writes `result[name] = cmd` unconditionally, so whichever declaration the range over
`t.entries` reaches last overwrites the other. Nothing warns.

## Measured (bin/dva at 8ae8da5 + TASK-097)

Both commands exist and are individually reachable — this is not a config error, it is two valid
commands one of which disappears from the listing:

```
$ dva "rails console"    -> rc=0  RAN-LITERAL-TOPLEVEL
$ dva rails console      -> rc=0  RAN-EXPANDED-SUB
```

But the listing holds only one of them, and which one is not stable — 20 consecutive
`dva manifest` runs on the same unchanged file:

| survivor in `dynamic_commands["rails console"]` | runs |
| --- | --- |
| `echo RAN-EXPANDED-SUB` | 19 |
| `echo RAN-LITERAL-TOPLEVEL` | 1 |

`dva manifest --format json | jq -r '.dynamic_commands | keys[]'` returns 2 keys where 3 commands
were declared. Go randomizes map iteration order, so the surviving row changes between runs of the
same binary on the same input — a reader who runs `dva ls` twice can get two different answers.

## Execution IS affected — corrected 2026-07-31

The original filing said "Execution is *not* affected: `Find` calls `expand(name, entry)` on the
single entry it looked up, so the two never share a map there." **That is true only when the two
declarations sit under different top-level entries.** They can also collide *inside one entry*, and
then `Find`'s own `expand` call is the map they collide in:

```yaml
interaction:
  a:
    subcommands:
      "b c":                 # a literal subcommand name containing a space
        command: "echo RAN-LITERAL-SUB"
      b:
        subcommands:
          c:                 # expands to "a b c" as well
            command: "echo RAN-NESTED-SUB"
```

Measured at `39d331e`, same binary, same unchanged file:

| invocation | 20 runs |
| --- | --- |
| `dva run a b c` | 18 × `RAN-NESTED-SUB`, **2 × `RAN-LITERAL-SUB`** |
| `dva run a b c --explain` → `Command:` | 10 × `echo RAN-NESTED-SUB`, **2 × `echo RAN-LITERAL-SUB`** (12 runs) |
| `dva a b c` (bare form) | 10 × `RAN-NESTED-SUB`, **2 × `RAN-LITERAL-SUB`** (12 runs) |

`--explain` is the sharper row: the *resolution* differs, not just the output, so a user who checks
the plan before running can be shown one command and get the other.

`dva validate` exits **0** on both fixtures and says `✅ dva.yml is valid`.

## Why P2 (raised from P3)

Filed as P3 on the belief that only the listing was wrong. With execution included this is
`dva run <name>` executing different code on consecutive invocations of the same binary against an
unchanged file — the failure mode the whole tool exists to prevent, and a direct contradiction of
two of the five core beliefs in `SOUL.md`:

- **신념 2** — "같은 설정과 입력은 같은 실행 순서를 만들어야 한다." Identical config and input must
  produce identical execution. Measured: they do not.
- **신념 3** — "하나의 동작에는 하나의 소유자만 둔다." One owner per behaviour. Two declarations own
  the name `a b c`, and which one owns it is decided by Go's map seed.

It still needs a config that spells one path two ways, which no file in `examples/` does (0 of 19
have a space in any interaction key). That is what keeps it below P1, not the severity of the
outcome.

## Options

- **A — detect and report.** `expandInto` refuses to overwrite: on a collision, record it and let
  `dva validate` fail with both declarations named. Turns a silent drop into a config error the
  author can act on. Does not make both commands listable.
- **B — key the map by path, not by the joined string.** The collision only exists because
  `["rails console"]` and `["rails", "console"]` flatten to one string. `ResolvedCommand.Path`
  (added by TASK-097) already carries the distinction; the map key does not. Both commands become
  listable, at the cost of a key type every consumer has to handle.
- **C — reject spaces in interaction keys at validate time.** Removes the whole input class.
  Breaking for any config using one today, and `schema.json`'s key pattern currently admits `\s`
  in both `interaction` and `subcommands`.

## Decision: A, in two parts

`SOUL.md` decides this one rather than a cost comparison, and it decides against B:

- 신념 3 — "하나의 동작에는 하나의 소유자만 둔다" — says two declarations must not share one name.
  B's whole purpose is to let them, so B contradicts the belief it would be implementing.
- The 이 신념 속의 선택 table — "자동 수정 vs 사용자 통제 → **진단 우선**" — prefers diagnosing over
  resolving on the user's behalf. B resolves; A diagnoses.
- 신념 2 — "암묵적인 추측보다 명시적인 선택과 오류를 선호한다" — prefers an explicit error to an
  implicit guess. Picking a winner by map seed is the implicit guess in its purest form.

B is also **structurally unable to fix the surface that motivated the filing.** `dva manifest`
emits `dynamic_commands` as a JSON object, and JSON object keys are strings — so two commands whose
paths flatten to one string cannot both appear there no matter what Go type the internal map uses.
B would make them distinguishable in memory and still lose one on the way out.

C stays rejected for the reason 097 rejected it: `schema.json` has admitted `\s` in interaction and
subcommand keys since it was written, and no shipped config uses one, so nothing is bought by
making legal configs illegal.

**Part 1 — determinism.** Sort the iteration in `List` and `expandInto` so the same file always
produces the same winner. This alone satisfies 신념 2's "같은 설정과 입력은 같은 실행 순서" clause and
converts an intermittent execution defect into a stable, reproducible one — which is a prerequisite
for anyone debugging it, and the part that must not wait.

**Part 2 — report.** Record the collision and surface it. `dva validate` already has a semantic
warning layer (measured: `[warn] semantic: interaction.a: has subcommands but is not directly
callable`), so the channel exists and only the check is missing.

## Acceptance criteria

- [x] Execution is deterministic | verify: `dva run a b c --explain` 20× on the intra-entry fixture; print the count of distinct `Command:` lines — must be 1
- [x] Listing is deterministic | verify: `dva manifest --format json` 30× on both fixtures; print the count of distinct `.dynamic_commands` — must be 1 each
- [x] The collision is not silent | verify: `dva validate` must name both colliding declarations and the key they share; print the message
- [x] No command disappears without a word | verify: `dva manifest --format json | jq '.dynamic_commands \| length'` — print it next to the number of declared commands, for both fixtures
- [x] Collision-free configs are untouched | verify: compare `dva manifest` across all `examples/*.yml` before and after; print the number of files compared and the number differing (must be 0)
- [x] Not vacuous | verify: human — revert each part separately and confirm the determinism assertion and the report assertion fail independently
- [x] Full suite passes | verify: `make test`

## Resolution

Both parts of decision A, implemented and measured against a baseline binary built from `7b1669e`
(`git archive HEAD`), which is this repo without the fix.

**Part 1 — determinism.** `internal/runner/interaction_tree.go`: `sortedNames()` fixes the iteration
order of both `t.entries` and `entry.Subcommands`, and `expandInto` changes from last-writer-wins to
**first**-writer-wins. Sorting alone would have been enough for determinism — last-writer-wins over a
sorted walk is equally reproducible — but first-writer-wins is what gives the surviving command the
shorter, more specific path and gives the report a stable Winner/Loser to name.

Recursion continues into a *losing* declaration's subcommands: its children sit at longer paths that
need not collide with anything, and skipping them would turn one dropped command into a dropped
subtree. `TestSubcommandsOfALoserStillExpand` pins that.

**Part 2 — report.** `List()` now delegates to a new `ListWithCollisions()`, which is **one walk**,
not a detector running beside the expansion. A second traversal that re-derives what the expansion
already knows is a second source of truth, and the two drift — the lesson from
[TASK-101](../_archive/101-default-args-inherit-into-subcommands-that-replace-the-command.md) and
[TASK-097](../_archive/097-interaction-usage-mishandles-keys-with-spaces.md). `internal/cli/validate.go` renders
each `Collision` through `describeInteractionPath`, which addresses a declaration the way the author
wrote it in `dva.yml` (`interaction.a.subcommands."b c"`) rather than by the flattened name they
never wrote. Emitted under category `interaction_collision`; counts toward `--strict`.

### Measured

| criterion | fixed | baseline (7b1669e) |
| --- | --- | --- |
| `dva run a b c` × 20 (intra-entry) | 20 × `RAN-NESTED-SUB` | 18 × `RAN-NESTED-SUB`, **2 × `RAN-LITERAL-SUB`** |
| `dva run a b c --explain` × 20 | 1 distinct `Command:` line | — |
| distinct `.dynamic_commands` × 30, intra-entry | **1** | 2 |
| distinct `.dynamic_commands` × 30, cross-entry | **1** | 2 |
| `dva validate` on either fixture | names both declarations + the shared key | silent, `✅ dva.yml is valid` |

The message, verbatim:

```
[warn] interaction: interaction.a.subcommands.b.subcommands.c and interaction.a.subcommands."b c"
  both resolve to the command "a b c"; only the first is reachable — rename one
```

Command counts, still short by one per collision — which is the point of part 2, since A diagnoses
rather than resolves:

| fixture | declarations | `dynamic_commands` | collisions reported |
| --- | --- | --- | --- |
| intra-entry | 4 | 3 | 1 |
| cross-entry | 3 | 2 | 1 |

Blast radius across the whole corpus: **19 files compared, 18 producing a manifest** (`modules/main.yml`
yields none standalone), **89 dynamic commands, 0 manifests differing** from baseline, **0 new
`[warn] interaction:` lines**. No shipped example spells one path two ways, which is what kept this
below P1.

### Non-vacuity

Each half reverted separately in its own copy of the tree, full packages run with no `-run` selector:

| probe | `TestList/FindIsDeterministic…` | `TestTheLoserIsMissing…`, `TestCollisionWarningNames…` | `TestCollisionsAreReported` |
| --- | --- | --- | --- |
| A — sorted iteration reverted, report kept | **FAIL** | pass | FAIL |
| B — report gutted, sorting kept | pass | **FAIL** | FAIL |
| control — both parts present | pass | pass | pass |

The two halves fail independently. `TestCollisionsAreReported` fails under both because it asserts
*which* declaration wins, which needs the sort and the report together.

### Two things the probes caught

- **`-run 'Collision'` does not select `TestTheLoserIsTheOneMissingFromTheMap`.** The first probe run
  used that selector and silently skipped two of the tests it was supposed to exercise — the same
  vacuous-selector trap recorded in [TASK-096](../_archive/096-manifest-static-commands-undercounts.md). Both
  probes were re-run against the full packages.
- **The test's own failure message lost the boundary it was testing.** `t.Errorf("winner = %v")` on a
  path prints both `["a","b","c"]` and `["a","b c"]` as `[a b c]`, so probe A's failure read
  `winner = [a b c], want [a b c]`. Changed to `%q`. Exactly the defect this task is about, occurring
  in the report of the report.

### Left open

- `manifest`'s `generated_at` is a second-resolution wall clock, so "distinct whole-manifest outputs
  over N runs" is not a determinism measurement — a loop crossing a second boundary reports 2 either
  way. The criterion above was corrected mid-verification to compare `.dynamic_commands`. Nothing is
  wrong with `generated_at`; it just cannot be part of a reproducibility check.
- Part 2 diagnoses; it does not make both commands listable. That is decision A working as chosen —
  see the SOUL.md reasoning above — not an omission. A config hitting this warning has two commands
  and one name, and the fix is to rename one.
- `gofmt -l` still flags `internal/runner/interaction_tree.go`. Verified pre-existing at HEAD (the
  `ResolvedCommand` struct comment alignment), owned by TASK-078, deliberately not mixed in here.

## Related

- [TASK-097](../_archive/097-interaction-usage-mishandles-keys-with-spaces.md) — found while measuring it. 097
  fixed the *rendering* for space-containing keys and added `ResolvedCommand.Path`, which is the
  structure option B would key on. After 097 whichever entry survives gets a correct
  `usage_example` — `dva 'rails console'` reaches the literal, `dva rails console` reaches the
  subcommand — so the two are coherent; only the disappearance remains.
- [TASK-095](../_archive/095-third-level-subcommands-never-expand.md) — the other defect in this flat
  key space.
