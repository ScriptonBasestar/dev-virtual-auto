---
id: TASK-169
title: "before/replace/after on a nested subcommand validate clean and never run"
type: bug
priority: P2
status: todo
effort: S
created-at: 2026-08-03T16:30:00+09:00
source: "TASK-140 review — the Result section cited a nested hook as its deepest warning hit; the hit is real, the hook is dead"
scope: "dva repo — internal/config/validate.go, internal/cli/hooks.go"
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

## Acceptance criteria

- [ ] `dva validate` on the fixture above exits non-zero, or warns — pick one and record
      why. The top-level case is an error today, which argues for consistency; the
      `validate_warnings.go:220` precedent argues that a key which has always been inert
      should not start failing configs at upgrade. Note that unlike `parallel:`, this one
      can hide a skipped backup, which is an argument the precedent did not have to weigh.
- [ ] The message names the full path (`interaction.db.subcommands.migrate`), not just the
      leaf, since the leaf name alone (`migrate`) looks hookable-adjacent and the author
      needs to know which nesting is the problem.
- [ ] Hooks nested under a *hookable* top-level name are covered by the same answer —
      `interaction.up.subcommands.fast.before` is equally unreachable, and a fix keyed off
      `IsHookableCommand(leafName)` would wave it through.
- [ ] A test fails without the change, driving `Validate()` rather than the helper, so
      deleting the recursion is caught (the registration gap TASK-140 hit).
- [ ] `make test` exits 0.

## Notes

`eachInteractionNode` in `internal/config/validate_warnings.go:287` already walks exactly
this tree and knows the dotted path at each node — reuse it rather than writing a second
recursion. Check whether `HasHooks()` is called anywhere else that shares the same
top-level-only assumption.
