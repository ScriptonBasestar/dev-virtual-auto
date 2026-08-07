---
id: TASK-169
title: "before/replace/after on a nested subcommand validate clean and never run"
type: bug
priority: P2
effort: S
created-at: 2026-08-03T16:30:00+09:00
source: "TASK-140 review — the Result section cited a nested hook as its deepest warning hit; the hit is real, the hook is dead"
scope: "dva repo — internal/config/validate.go"
status: done
quality-review: pass
quality-reviewed-at: 2026-08-07T18:05:08+09:00
verified-at: 2026-08-07T18:05:08+09:00
archived-at: 2026-08-07T18:05:08+09:00
quality-review-evidence: |
  - kind: test
    command-or-step: make test && make doc-check (mise go 1.26.4)
    result: exit 0; shared suite green
  - kind: recheck
    command-or-step: acceptance criteria re-observed
    result: validateHookPlacement; nested hooks rejected
verification-summary: |
  quality-review pass; re-checked deliverables. validateHookPlacement; nested hooks rejected. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 169: Reject hooks where they cannot run, at every depth

## Problem

`internal/config/validate.go:134-138` rejects hooks on a non-hookable command:

```go
for name, cmd := range c.Interaction {
    if cmd.HasHooks() && !IsHookableCommand(name) {
        return fmt.Errorf("interaction.%s: before/replace/after hooks are only supported on hookable commands (up, down, stop, restart, build, clean, logs)", name)
    }
}
```

It iterates `c.Interaction` only. It does not recurse into `Subcommands`. Moving the same
hook one level down turns a hard validation error into silence — and the hook still does
not run.

Measured 2026-08-03, `bin/dva` v0.1.44:

```yaml
interaction:
  db:
    subcommands:
      migrate:
        before:
          - {step: backup, run: "echo BACKUP-RAN"}
        command: echo MIGRATING
```

```
$ dva validate
✅ dva.yml is valid          # rc 0
$ dva db migrate
MIGRATING                    # rc 0 — BACKUP-RAN never printed
```

Declared as `interaction.migrate.before`, the identical hook is a rc-1 validation failure.

Hooks execute only through `wrapWithHooks` (`internal/cli/hooks.go:20`), wired at
`internal/cli/root.go:129` for the seven hookable built-ins, plus the native-mode build
delegation at `internal/cli/compose.go:521`. Nothing walks `Subcommands` looking for hooks,
so there is no path on which a nested one could fire.

The shape is worse than an inert step. `before: [backup]` on a migrate command reads as a
safety interlock; the config validates, the command exits 0, and the backup did not happen.
Nothing distinguishes that run from one where it did.

## Why this surfaced now

TASK-140's `parallel:` warning walks the interaction tree at every depth, so it reports
`interaction.db.subcommands.migrate.before[0] "backup"` — and its Result section quoted that
as its deepest hit. The hit is real; the location is dead code. Warning that a key inside an
unreachable block is ignored, while saying nothing about the block, is the sharper bug of
the two. TASK-140's Result has been corrected to say so.

## Decision: an error, at every depth, with no account taken of the node's name

**Chosen: error.** `validateHookPlacement` (`validate.go:213`) replaces the top-level loop
and walks the tree with `eachInteractionNode`. A nested hook is rejected unconditionally; the
top-level rule and its message are unchanged.

### Why not a warning

The `warnInertProvisionSteps` precedent — a long-inert key should not start failing configs at
upgrade — rests on a premise that does not hold here, and `warnIgnoredParallelSteps` states it
in its own comment: a dropped `parallel:` "produces exactly the right output and merely takes
twice as long", so the author has something to notice. A skipped `before: [backup]` produces
exactly the right output and no signal.

Measured, and this is what settled it — semantic warnings do not reach the run path at all:

```
$ dva validate          # nested fixture, before the fix
[warn] semantic: interaction.db: has subcommands but is not directly callable; ...
✅ dva.yml is valid     # rc 0

$ dva db migrate
MIGRATING               # rc 0 — and not even the warning above
```

So warn-only would put the notice on the one command the person running a migration does not
run. The config an error breaks at upgrade is a config whose backup was never running; saying
so out loud is the entire value.

Consistency is the second argument, not the first: the identical hook one level up is rc 1
today, and a verdict that flips on nesting depth alone is the disagreement-between-surfaces
shape this queue keeps closing.

### Why the nested rule ignores the node's name

The task's own example needed correcting before it could be measured. Written as stated —
`interaction.up.subcommands.fast.before` with no hooks on `up` itself — the config dies first
with a **reserved command conflict**, because `ShadowedByBuiltin("up", cmd)` holds when the
top-level `up` declares no hooks. It never reaches the hook check. Giving `up` a hook of its
own gets there.

| fixture | before | `dva <cmd>` before |
|---|---|---|
| `interaction.db.subcommands.migrate.before` | validate rc 0 | `dva db migrate` → `MIGRATING`, rc 0 |
| `interaction.up.subcommands.fast.before` (hooked parent) | validate rc 0 | `dva up fast` runs the **parent's** hook, then rc 1 on the positional |
| `interaction.db.subcommands.up.before` | validate rc 0 | `dva db up` → `DB-UP`, rc 0 |

Row three is the one that fixes the rule's shape, and it is not the row the criterion named.
`fast` is not a hookable name, so a check keyed off `IsHookableCommand(leafName)` catches row
two by accident. Row three's leaf is literally called `up` — hookable — and its hook is exactly
as dead. So the nested rule takes no account of the name, and the message says so:
*"whatever the subcommand is named"*.

Row two also shows the shape is worse than inert: `dva up fast` executes the parent's
`before:` and then fails, so the hook that did fire belongs to a command the author did not
invoke.

### Not fixed by making nested hooks run

Considered and declined. That is a runner change, not a validation one, and it would give
`before:` a second meaning at depth before anyone has asked for one. Rejecting where it cannot
run leaves that door open to be opened deliberately.

## Result

```
interaction.db.subcommands.migrate: before/replace/after hooks run only on a top-level
hookable command (up, down, stop, restart, build, clean, logs); a hook nested under a
subcommand never runs, whatever the subcommand is named
```

All three nested fixtures now exit 1 with the path that names the nesting. The top-level
message is byte-identical to before — the walker's path at depth 0 is `interaction.<name>`,
which is what the old inline `fmt.Sprintf` produced. The repo's own `dva.yml` still validates
rc 0, and the examples corpus passes unchanged; no config in the repo declares a nested hook.

`sort.Strings` on the collected problems is not decoration: `c.Interaction` is a map, and a
config with two violations named a different one on each run. First-only reporting matches how
the rest of `Validate` behaves. TASK-128.

## Tests

`internal/config/hook_placement_test.go`, six cases through `Validate()` plus an order-stability
test. Loaded via `Load()` because `Validate()` refuses a Config whose `filePath` is empty and
`filePath` is unexported with no setter.

Falsified twice, because the criterion asks for two different failures:

**Recursion removed** (walker swapped back for the top-level loop) — the three nested rows fail
at `hook_placement_test.go:124`, the `err == nil` line:

```
--- FAIL: TestValidateRejectsHooksWhereTheyCannotRun/nested_under_a_non-hookable_parent
    hook_placement_test.go:124: Validate() = nil; the hook here can never execute, so a clean verdict tells the author their config works when it does not
--- FAIL: .../nested_under_a_hookable_parent
--- FAIL: .../nested_leaf_whose_own_name_is_hookable
--- FAIL: TestHookPlacementErrorIsStableAcrossRuns
    hook_placement_test.go:164: expected an error from a config with two nested hooks
```

The two top-level rows still pass, correctly — neither depends on the recursion.

**Call unwired** (`validateHookPlacement` defined but not called from `Validate`) — the same
three fail *plus* `top-level_non-hookable_is_unchanged`. That fourth failure is the one that
matters here: it is the registration gap TASK-140 hit, where a check worked and was never
wired, and only a test driving `Validate()` rather than the helper can see it.

## Acceptance criteria

- [x] `dva validate` on the fixture above exits non-zero, or warns — pick one and record
      why. — error; see "Why not a warning", which turns on the measurement that semantic
      warnings never reach the run path.
- [x] The message names the full path (`interaction.db.subcommands.migrate`), not just the
      leaf. | verify: `go test ./internal/config/ -run TestValidateRejectsHooksWhereTheyCannotRun`
- [x] Hooks nested under a *hookable* top-level name are covered by the same answer. —
      covered, and extended: the criterion's own example is caught by a leaf-name check
      too, so a third fixture (`db.subcommands.up`) pins the rule that no name is consulted.
- [x] A test fails without the change, driving `Validate()` rather than the helper, so
      deleting the recursion is caught (the registration gap TASK-140 hit). — both
      falsifications pasted above.
- [x] `make test` exits 0. — green; `go vet ./...`, `gofmt -l`, `make doc-check` all clean.

## Notes

`eachInteractionNode` in `internal/config/validate_warnings.go:287` already walks exactly
this tree and knows the dotted path at each node — reuse it rather than writing a second
recursion. Check whether `HasHooks()` is called anywhere else that shares the same
top-level-only assumption.
