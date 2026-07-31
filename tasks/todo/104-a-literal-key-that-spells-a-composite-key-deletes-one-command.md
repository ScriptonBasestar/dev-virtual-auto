---
id: TASK-104
title: "A declared interaction key that spells a composite key silently deletes one of the two commands — and when both live under one parent, `dva run` executes a different one between runs"
type: fix
priority: P2
effort: M
status: todo
created-at: 2026-07-31T11:05:00+09:00
scope: "internal/runner/interaction_tree.go:96-115 — expandInto writes result[name] unconditionally; reached by both List (listing) and expand (execution, via Find)"
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

- [ ] Execution is deterministic | verify: `dva run a b c --explain` 20× on the intra-entry fixture; print the count of distinct `Command:` lines — must be 1
- [ ] Listing is deterministic | verify: `dva manifest --format json` 20× on both fixtures; print the count of distinct outputs — must be 1 each
- [ ] The collision is not silent | verify: `dva validate` must name both colliding declarations and the key they share; print the message
- [ ] No command disappears without a word | verify: `dva manifest --format json \| jq '.dynamic_commands \| length'` — print it next to the number of declared commands, for both fixtures
- [ ] Collision-free configs are untouched | verify: compare `dva manifest` across all `examples/*.yml` before and after; print the number of files compared and the number differing (must be 0)
- [ ] Not vacuous | verify: human — revert each part separately and confirm the determinism assertion and the report assertion fail independently
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-097](../done/097-interaction-usage-mishandles-keys-with-spaces.md) — found while measuring it. 097
  fixed the *rendering* for space-containing keys and added `ResolvedCommand.Path`, which is the
  structure option B would key on. After 097 whichever entry survives gets a correct
  `usage_example` — `dva 'rails console'` reaches the literal, `dva rails console` reaches the
  subcommand — so the two are coherent; only the disappearance remains.
- [TASK-095](../done/095-third-level-subcommands-never-expand.md) — the other defect in this flat
  key space.
