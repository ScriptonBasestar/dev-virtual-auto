---
id: TASK-152
title: "The collision warning says only the first is reachable, and the second still runs"
type: bug
priority: P2
status: todo
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

- [ ] The warning distinguishes the two shapes. Cross-entry says what is actually lost —
      the command does not appear in `dva ls`, and which spelling reaches which
      declaration. Intra-entry keeps "unreachable", because there it is true.
- [ ] Both messages are reproduced in the task's Resolution with the invocation that proves
      them, not paraphrased.
- [ ] `interaction_collision_warning_test.go` covers a cross-entry config, asserting the
      new wording. The existing intra-entry case stays.
- [ ] A test asserts the cross-entry loser still executes — that behaviour is the reason
      the wording has to change, so it should be pinned before the wording is.
- [ ] Decide whether the cross-entry case should stay a warning at all, or become an error.
      Two declarations both live, one invisible, is arguably worse than one shadowed. If it
      stays a warning, say why in the Resolution.
- [ ] `make test` exits 0.

## Notes

Distinct from TASK-137, which is about reserved-name namespace prefixes in the manifest —
a different surface. TASK-104's own fix (a literal key that spells a composite key deleting
one command) is intact; this is only the sentence describing the result.
