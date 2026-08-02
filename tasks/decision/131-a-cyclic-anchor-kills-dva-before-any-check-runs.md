---
id: TASK-131
title: "A cyclic YAML anchor kills dva before any check runs, and the crash is ours rather than yaml.v3's"
type: decision
priority: P2
status: decision
effort: M
created-at: 2026-08-02T00:00:00+09:00
scope: "internal/config/config.go — InteractionCommand.UnmarshalYAML:383-411, loadFile:932; internal/config/validate_warnings.go — MaxSubcommandDepth:762, calculateSubcommandDepth. No fix applied; this task decides which defense."
---

# Task 131: Decide how a self-referencing anchor is rejected

## The blind spot

A `dva.yml` whose `interaction:` tree contains a self-referencing anchor terminates the process
with `fatal error: stack overflow` during config load. Every entry point that reads config is
affected, and none of them get to run a single check.

```yaml
interaction:
  loop: &loop
    command: echo hi
    subcommands:
      self: *loop
```

| command | exit | stderr |
|---|---|---|
| `dva validate` | **2** | `runtime: goroutine stack exceeds 1000000000-byte limit` |
| `dva doctor` | **2** | same |
| `dva run loop` | **2** | same |

~508 stderr lines and `...2340902 frames elided...` — both are samples of one run, not constants.

Two properties make this worse than an ugly error message:

1. **It is a `fatal error:`, not a `panic:`** — grepped for `^(panic|fatal error):`, matched
   `fatal error: stack overflow`. Go's runtime throws it; it is not panicked. **No `recover()`
   anywhere up the stack can contain it**, so the defense cannot be a deferred recover in
   `loadFile` and must happen *before* decode.
2. **DVA already has a check for exactly this shape and it can never run.**
   `MaxSubcommandDepth = 5` (`validate_warnings.go:762`) works correctly — on a deep-but-acyclic
   fixture `dva validate` prints `interaction.alpha: nested 6 levels deep (max 5)` and exits 0.
   It is a post-decode check, so on the input that most needs a depth bound the process is already
   dead.

## The mechanism

The recursion cycle, read off the trace (`(*InteractionCommand).UnmarshalYAML` appears **7 times**
in the un-elided head alone):

```
(*decoder).mappingStruct
  → (*decoder).unmarshal
    → (*Node).Decode                                  ← resets the cycle guard here
      → (*InteractionCommand).UnmarshalYAML           ← internal/config, config.go:383
        → (*decoder).callUnmarshaler
          → (*decoder).unmarshal
            → (*decoder).mappingStruct  → ...
```

yaml.v3 **does** guard against cyclic anchors (`decode.go:534-538`):

```go
if d.aliases[n] {
    failf("anchor '%s' value contains itself", n.Value)
}
d.aliases[n] = true
```

but that set lives on the decoder, and `(*Node).Decode` in `yaml.go` opens with `d := newDecoder()`
— and `newDecoder` does `d.aliases = make(map[*Node]bool)` (`decode.go:346`). So every re-entry
through a custom unmarshaler hands the walk a **fresh, empty** alias set. The guard is walk-scoped
by design (note the `delete(d.aliases, n)` after recursing); it was never built to survive
re-entry. It therefore never fires, and the recursion is unbounded.

The trigger is one field. `InteractionCommand.UnmarshalYAML` decodes into a local `plain` alias
under this comment (`config.go:388-389`):

```go
// Decode all non-command fields using the tag-based alias.
// We use a plain alias that has no UnmarshalYAML to avoid recursion.
```

True of the outer struct. False of the field that matters — `plain.Subcommands` at `:403` is
`map[string]*InteractionCommand`, still the custom-unmarshaler-bearing type. Dropping a method
from a type does not drop it from that type's fields, so `node.Decode(&p)` re-enters
`UnmarshalYAML` immediately. The comment names the hazard it does not actually avoid.

## Why this is ours, not upstream

Three shapes, one document, one library version (v3.0.1), each in its own process:

| decoded into | result |
|---|---|
| `yaml.Node` | `err=<nil>`, DocumentNode, 1 child — **survives** |
| plain struct, no `UnmarshalYAML` anywhere | `yaml: anchor 'loop' value contains itself` — **clean error** |
| custom `UnmarshalYAML` whose `plain` alias keeps the custom field type | **stack overflow** |

The library's guard works. DVA's unmarshaler is what defeats it. And `gopkg.in/yaml.v3 v3.0.1` is
the newest published version (checked against the module proxy: only `v3.0.0` and `v3.0.1` exist),
so there is no upgrade to wait for regardless.

The `yaml.Node` row is the important one: it proves a **pre-decode Node-tree walk is feasible**,
because parsing the same bytes into a `Node` does not recurse into user types at all.

**Blast radius: 1 of 7.** `internal/` has 7 `UnmarshalYAML` implementations; checking each struct
body for a field of its own type, only `InteractionCommand` is self-referential
(`Subcommands map[string]*InteractionCommand`). It is reachable from `interaction:`
(`config.go:22`) and nested `subcommands:` (`:323`) — not from `stack:` or `applications:`.

## What this corrects in TASK-125

[TASK-125](../done/125-three-interaction-warnings-stop-at-depth-1.md) §"Why the walker needs no
cycle guard" (`:130-142`) recorded three things that measurement contradicts:

| TASK-125 said | measured |
|---|---|
| "the **panic** trace" | `fatal error: stack overflow` — not a panic; `recover()` is not an option |
| "with **no `internal/config` frame**" | **18** `internal/config` frames; `(*InteractionCommand).UnmarshalYAML` is named in the trace and is the frame driving the recursion |
| "it is **upstream** of every check in this file" | a plain struct on the same version returns a clean error; the crash needs DVA's own unmarshaler |

**Its conclusion still holds.** `eachInteractionNode` and `calculateSubcommandDepth` (verified
unguarded — no depth cap, no visited set) really are safe today, because decode never yields a
cyclic `Config`. The reasoning was wrong; the result was right.

That distinction is not bookkeeping — it is a constraint on this task. Two of the options below
preserve "decode never yields a cycle"; any option that instead *tolerates* shared anchors would
invalidate TASK-125's conclusion and require adding cycle guards to both walkers in the same
change. **Whoever picks the fix owns that coupling.**

## Options

- **A — Pre-decode `yaml.Node` cycle scan in `loadFile` (`config.go:932`).** Parse to a `Node`
  (proven to survive), walk for a node reachable from itself, reject with an error naming the
  anchor and its YAML path. Type-independent: covers all 7 unmarshalers and any self-referential
  type added later. Cost: a second parse per config file, plus a new walk to maintain.
- **B — Retype `plain.Subcommands` to a shadow type without `UnmarshalYAML`.** Restores yaml.v3's
  own guard, so the clean `anchor 'loop' value contains itself` error comes for free with no new
  code path. Cost: the custom unmarshaler's *only* job is `command:` polymorphism — scalar vs
  sequence (`config.go:437-461`) — so a naive shadow type silently drops list-form `command:`
  inside `subcommands:`. It would have to re-implement that, and nothing currently proves it wrong
  if it doesn't.
- **C — Thread a depth counter through the unmarshaler.** `UnmarshalYAML(node *yaml.Node) error`
  has a fixed signature, so depth has to live off-band — a package-level var (not
  concurrency-safe) or a receiver field set by the parent. Bounds the damage without diagnosing
  the cause, and reports a depth error for what is really a cycle.
- **D — Accept and document.** It takes a hand-written self-referencing anchor to trigger; no user
  reaches it by accident.

## Recommendation: A

A is the only option that is not tied to one type. B is cheaper and fixes today's single instance,
but it leaves the next self-referential type to rediscover this from a stack trace, and it trades a
crash for a possible silent loss of list-form `command:` in subcommands. A is also the only option
that can name the anchor and its path, which is the difference between an error a user can act on
and one they can only report.

A and B both preserve TASK-125's conclusion. C weakens the diagnosis. D leaves a
`fatal error` reachable from a config file.

## The cost that needs deciding with it

Whatever is chosen needs a test asserting a **clean error** on the cyclic fixture — and per
TASK-116's standard, one that fails when the fix is reverted. That test cannot simply call the
loader in-process today: an unfixed loader takes the test binary down with it rather than failing
an assertion, so the red half has to run the fixture through a subprocess. After the fix the
in-process form works, which means the mutation check and the shipped test are not the same shape.

`internal/config/` currently has **no test at all** for cyclic or self-referential anchors, so
there is nothing to extend and nothing to break.

## Related

- [TASK-125](../done/125-three-interaction-warnings-stop-at-depth-1.md) — `:130-142` is the record
  this corrects; its conclusion survives, its three stated pieces of evidence do not.
- [TASK-128](../done/128-the-recursion-was-right-the-nodes-it-walked-were-not.md) — the last time a
  comment in this area described a property the code did not have.
