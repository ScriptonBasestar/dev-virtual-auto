---
id: TASK-152
title: "The collision warning says only the first is reachable, and the second still runs"
type: bug
priority: P2
status: done
effort: S
created-at: 2026-08-03T13:55:00+09:00
source: "TASK-104 finalize verification — the wording, not the fix, is wrong for the cross-entry shape"
depends-on: [TASK-104]
scope: "dva repo — internal/cli/validate.go:40"
---

# Task 152: Say what the collision actually costs

## Problem

When two declarations resolve to the same command name, `dva validate` warns
(`internal/cli/validate.go:40`):

> `interaction.rails.subcommands.console and interaction."rails console" both resolve to
> the command "rails console"; only the first is reachable — rename one`

For the **cross-entry** shape the second clause is false. Measured 2026-08-03 on
`bin/dva` v0.1.44, with both declarations present:

```
$ dva run 'rails console'    # the declaration the warning calls unreachable
RAN-LITERAL-TOPLEVEL

$ dva run rails console      # the winner
RAN-EXPANDED-SUB
```

Both run. `Find` (`internal/runner/interaction_tree.go:56-58`) looks the literal top-level
key up directly, so a quoted invocation reaches the "loser" deterministically. What that
declaration lost is the **listing**, not reachability.

This is the dangerous direction for a diagnostic to be wrong in. An author who trusts the
message deletes a declaration that is still executing for every user who types the quoted
form.

## Scope

Intra-entry collisions are correctly described — there the loser really is unreachable:
`dva run a "b c"` runs `RAN-NESTED-SUB` and nothing else can reach the other declaration.

So the message is right for one shape and wrong for the other, and says the same sentence
for both. The test that pins it,
`internal/cli/interaction_collision_warning_test.go:43-47`, uses an intra-entry config
only — which is why the cross-entry wording was never exercised.

## Acceptance criteria

- [x] The warning distinguishes the two shapes by top-level key: `Winner[0] != Loser[0]` is the
      cross-entry case (each declaration under a different top-level key, each still reachable);
      same key is intra-entry (loser unreachable). Cross-entry says what is lost — the `dva ls`
      listing — and that both still run; intra-entry keeps "only the first is reachable".
- [x] Both messages reproduced in Resolution below, with the invocations that prove them.
- [x] `interaction_collision_warning_test.go` covers a cross-entry config
      (`TestCollisionWarningCrossEntrySaysBothRun`); the existing intra-entry case stays.
- [x] `TestCrossEntryCollisionLoserIsStillReachable` asserts BOTH declarations still execute
      (split reaches the nested, quoted reaches the literal) — pinned before the wording.
- [x] Cross-entry stays a WARNING. See Decision below.
- [x] `make test` exits 0.

## Decision: warning, not error

A cross-entry collision leaves two declarations both reachable and functional; the only thing
lost is that `dva ls` shows one. Promoting it to an error would reject configs that work, for a
problem whose cost is a hidden listing — a cost the (now-accurate) warning already states. So it
stays a warning: inform the author, do not block the config.

## Resolution

The warning now branches on whether the two colliding declarations share a top-level key.

Intra-entry (same top-level key — loser genuinely unreachable):

```
interaction.a.subcommands."b c" and interaction.a.subcommands.b.subcommands.c both resolve to
the command "a b c"; only the first is reachable — rename one
```

Pinned by `TestCollisionWarningNamesBothDeclarations` over `collidingConfig()` (both declarations
under `interaction.a`). `dva run a "b c"` runs `RAN-NESTED-SUB`; nothing reaches the other.

Cross-entry (different top-level keys — both reachable, listing lost):

```
interaction."rails console" and interaction.rails.subcommands.console both resolve to the command
"rails console"; both still run (each by its own spelling), but only the first is listed in
`dva ls` — rename one so both are visible
```

Pinned by `TestCollisionWarningCrossEntrySaysBothRun` and `TestCrossEntryCollisionLoserIsStillReachable`
over `crossEntryCollidingConfig()`. `dva run rails console` → `RAN-EXPANDED-SUB`;
`dva run 'rails console'` → `RAN-LITERAL-TOPLEVEL`. Both run; the loser lost the listing, not
reachability — which is what the message now says.

## Notes

Distinct from TASK-137, which is about reserved-name namespace prefixes in the manifest —
a different surface. TASK-104's own fix (a literal key that spells a composite key deleting
one command) is intact; this is only the sentence describing the result.
