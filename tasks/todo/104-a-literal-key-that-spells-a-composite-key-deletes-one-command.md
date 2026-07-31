---
id: TASK-104
title: "A declared interaction key that spells a composite key silently deletes one of the two commands, and which one survives changes between runs"
type: fix
priority: P3
effort: M
status: todo
created-at: 2026-07-31T11:05:00+09:00
scope: "internal/runner/interaction_tree.go:76-107 — List and expandInto write both a declared key and a synthesized parent+space+child key into one map[string]*ResolvedCommand"
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

Execution is *not* affected: `Find` calls `expand(name, entry)` on the single entry it looked up,
so the two never share a map there. The defect is confined to the listing surfaces — which is what
makes it easy to miss and what makes the two documents disagree with each other.

## Why P3

Nothing runs wrongly and no exit code lies. It needs a config that declares a key spelling another
key's path, which no file in `examples/` does (0 of 19 have a space in any interaction key). But
the failure mode is the bad kind: silent, nondeterministic, and it makes `dva ls` and `dva
manifest` — the two surfaces [TASK-076](../done/076-manifest-advertises-the-one-invocation-that-cannot-reach-the-interaction.md)
went to the trouble of reconciling — disagree with each other run to run.

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

A is the smallest thing that stops the silence. **Decision needed.**

## Acceptance criteria

- [ ] The collision is not silent | verify: on the fixture above, `dva validate` or `dva ls` must name both declarations; print the message
- [ ] Determinism | verify: run the chosen listing command 20 times and print the count of distinct outputs — must be 1
- [ ] No command disappears without a word | verify: `dva manifest --format json \| jq '.dynamic_commands \| length'` — print it next to the number of declared commands
- [ ] Colliding-free configs are untouched | verify: compare `dva manifest` across all `examples/*.yml` before and after; print the number of files compared and the number differing (must be 0)
- [ ] Not vacuous | verify: human — revert the fix and confirm the new assertion fails on the collision fixture
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-097](../done/097-interaction-usage-mishandles-keys-with-spaces.md) — found while measuring it. 097
  fixed the *rendering* for space-containing keys and added `ResolvedCommand.Path`, which is the
  structure option B would key on. After 097 whichever entry survives gets a correct
  `usage_example` — `dva 'rails console'` reaches the literal, `dva rails console` reaches the
  subcommand — so the two are coherent; only the disappearance remains.
- [TASK-095](../done/095-third-level-subcommands-never-expand.md) — the other defect in this flat
  key space.
