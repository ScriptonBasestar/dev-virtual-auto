---
id: TASK-095
title: "`mergeInteraction` drops `child.Subcommands`, so subcommands nested three deep never expand — including two in the project's own shipped example"
type: fix
priority: P2
effort: S
status: todo
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/runner/interaction_tree.go — mergeInteraction (~:165-245) never copies child.Subcommands; expandInto (~:93-99) recurses on the merged value"
---

# Task 095: the shipped example declares two commands that cannot be reached

## Problem

`expandInto` recurses through the **merged** command:

```go
for subName, subEntry := range entry.Subcommands {
    merged := mergeInteraction(entry, subEntry)
    t.expandInto(name+" "+subName, merged, result)
}
```

but `mergeInteraction` never assigns `Subcommands` — `grep 'Subcommands'` inside that function
returns nothing. So `merged.Subcommands` is always nil, the recursion terminates one level early,
and nothing below depth 2 is ever added to the tree.

(That nil is also load-bearing in an unfortunate way: were `merged.Subcommands` set to the
*parent's* map instead, `expandInto` would recurse forever. Any fix has to copy the child's map
specifically, not carry the parent's through the merge.)

## Measured

`examples/full-stack.yml:165-181` declares `rails` → `db` → `migrate`/`seed`. Copied to a
scratch dir and run against `bin/dva` at `2e0cfd6`:

```
$ dva ls | grep rails
rails          # Run Rails commands
rails console  # Start Rails console
rails db       # Database related commands
```

`rails db migrate` and `rails db seed` are absent. Worse than absent: `rails db` is listed as
though it were callable, but it carries only a `description:` and no command — it is a pure
container node whose entire purpose is the two children that were dropped.

This is the project's own example file, which `internal/config/examples_test.go` loads — so the
example is validated for *parsing* while the thing it demonstrates does not work.

## Why it is P2 rather than cosmetic

Three-level nesting is a documented feature that silently degrades. There is no error, no
warning, and `dva validate` exits 0 — the user's declaration is accepted and then discarded,
which is the same silent-loss shape as [TASK-085](../done/085-interaction-steps-silently-drop-compose-keys.md)
and [TASK-094](094-kubectl-runner-discards-steps.md).

Whether depth is bounded deliberately is worth checking during the fix: if there is a reason to
stop at 2, the config must be rejected at validate time rather than silently truncated.

## Acceptance criteria

- [ ] Third-level subcommands expand | verify: `dva ls` on `examples/full-stack.yml` must list `rails db migrate` and `rails db seed`; print the full count of `rails*` rows (3 today)
- [ ] They are runnable, not just listed | verify: `dva run "rails db migrate" --explain` must resolve to `db:migrate`; print the resolved command
- [ ] Arbitrary depth, or an explicit limit | verify: a 4-level fixture either expands or fails `dva validate` with a message naming the depth — silent truncation is not acceptable
- [ ] No infinite recursion | verify: `make test` must complete; a merge that carries the parent's Subcommands through would hang here
- [ ] The merge still inherits what it did before | verify: `go test ./internal/runner/ -run Interaction` — print the number of tests selected
- [ ] Not vacuous | verify: human — revert the merge hunk and confirm the new `rails db migrate` assertion fails
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-096](096-manifest-static-commands-undercounts.md) and
  [TASK-097](097-interaction-usage-mishandles-keys-with-spaces.md) — the other two defects in the
  same flat `parent + " " + child` key space. 097 in particular is the reason this key encoding is
  worth revisiting rather than patching three times.
