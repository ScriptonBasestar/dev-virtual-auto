---
id: TASK-095
title: "`mergeInteraction` drops `child.Subcommands`, so subcommands nested three deep never expand — including two in the project's own shipped example"
type: fix
priority: P2
effort: S
status: done
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/runner/interaction_tree.go — mergeInteraction (~:165-245) never copies child.Subcommands; expandInto (~:93-99) recurses on the merged value"
verified-at: 2026-08-03T13:20:00+09:00
archived-at: 2026-08-03T13:20:00+09:00
verification-summary: |
  Fix is present and load-bearing: internal/runner/interaction_tree.go:262 assigns the CHILD's
  Subcommands map, with the comment at :255-261 recording why it cannot be in the struct literal.
  Live binary on examples/full-stack.yml lists 5 rails* rows (was 3) and resolves
  `dva run rails db migrate --explain` to `Command: db:migrate` with argv empty, service `web`
  and RAILS_LOG_TO_STDOUT inherited across two merge levels.
  Depth is unbounded, not merely deeper: a scratch depth-4 fixture validates rc=0, lists all
  4 keys, and runs `echo L4` — so the "explicit limit or expand" criterion resolves to expand.
  Non-vacuity re-proved independently, not taken from the task file: HEAD sources archived to
  scratch with line 262 deleted → 5 of 5 tests in interaction_depth_test.go fail with the exact
  pre-fix values (3 keys, `bundle exec rails` + argv [migrate], depth-4 count 2).
  `go test ./internal/runner/ -run Interaction`: 8 top-level / 32 with subtests, 0 failures.
  Both "Left open" items have since closed: TASK-101 is in tasks/done/ and deliberately rewrote
  the characterization test to TestInteractionDefaultArgsStopAtACommandOverride (documented at
  interaction_depth_test.go:165-175), and TASK-078's gofmt drift is gone — `gofmt -l` on both
  files is clean, task archived.
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

but `mergeInteraction` never assigned `Subcommands` — `grep 'Subcommands'` inside that function
returned nothing. So `merged.Subcommands` was always nil, the recursion terminated one level early,
and nothing below depth 2 was ever added to the tree.

(That nil was also load-bearing in an unfortunate way: were `merged.Subcommands` set to the
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
which is the same silent-loss shape as [TASK-085](../_archive/085-interaction-steps-silently-drop-compose-keys.md)
and [TASK-094](094-kubectl-runner-discards-steps.md).

Whether depth is bounded deliberately is worth checking during the fix: if there is a reason to
stop at 2, the config must be rejected at validate time rather than silently truncated.

## Acceptance criteria

- [x] Third-level subcommands expand | verify: `dva ls` on `examples/full-stack.yml` must list `rails db migrate` and `rails db seed`; print the full count of `rails*` rows (3 today)
- [x] They are runnable, not just listed | verify: `dva run "rails db migrate" --explain` must resolve to `db:migrate`; print the resolved command
- [x] Arbitrary depth, or an explicit limit | verify: a 4-level fixture either expands or fails `dva validate` with a message naming the depth — silent truncation is not acceptable
- [x] No infinite recursion | verify: `make test` must complete; a merge that carries the parent's Subcommands through would hang here
- [x] The merge still inherits what it did before | verify: `go test ./internal/runner/ -run Interaction` — print the number of tests selected
- [x] Not vacuous | verify: human — revert the merge hunk and confirm the new `rails db migrate` assertion fails
- [x] Full suite passes | verify: `make test`

## Resolution

One line in `mergeInteraction`, taking the **child's** map rather than the parent's, with a comment
recording why it is not in the struct literal above it:

```go
merged.Subcommands = child.Subcommands
```

Regression coverage in `internal/runner/interaction_depth_test.go` (5 tests), whose fixture mirrors
`examples/full-stack.yml:156-181` so the assertions name the same commands the example ships.

### Before / after

| measurement | before | after |
| --- | --- | --- |
| `dva ls` rows matching `rails*` (`examples/full-stack.yml`) | 3 | **5** |
| `InteractionTree.List()` keys for that fixture | 3 | **5** |
| `dva run rails db migrate --explain` | `Command: bundle exec rails`, argv `[migrate]` | **`Command: db:migrate`**, argv empty |
| `dva run rails db seed --explain` | `Command: bundle exec rails`, argv `[seed]` | **`Command: db:seed`** |
| depth-4 fixture (`l1 l2 l3 l4`) keys | 2 | **4** |
| `go test ./internal/runner/ -run Interaction` | 3 top-level / 21 with subtests | **8 top-level / 26 with subtests**, 0 failures |
| `make test` | pass | pass |

The "before" row for `dva run rails db migrate` is the sharper statement of the defect than the
task title: the invocation did not fail. `Find` fell back to the shortest matching prefix,
resolved `rails db` — which has no `command:` and so inherited `bundle exec rails` — and passed
`migrate` as argv. The user got `bundle exec rails migrate`, a command Rails does not have, with
exit status from Rails rather than from dva.

### Non-vacuity

`git archive HEAD | tar -x` into a scratch tree gives the pre-fix source exactly; the new test
file was copied in unchanged and run there.

| probe | result |
| --- | --- |
| new tests against pre-fix source | **4 of 5 fail** — `key count = 3, want 5`; `command = "bundle exec rails", want "db:migrate"`; `subcommand count = 0, want 2` |
| the 5th (`…DefaultArgsInheritIntoSubcommands`) | passes both ways — correct, it characterizes behaviour this fix does not change |
| `merged.Subcommands = parent.Subcommands` (the hazard) | **`panic: test timed out after 30s`** — confirms the comment's claim that carrying the parent's map does not terminate |
| fix present in working tree / absent in probe tree | `grep -c 'merged.Subcommands'` → 2 / 0 |

### Depth is unbounded

The open question in the Problem section resolves to the first branch: there was no deliberate
limit, only the missing assignment. `Find` already tried progressively shorter key prefixes for
arbitrary `argv`, so it needed no change once `expandInto` produced the keys. A depth-4 fixture
expands and runs; no validate-time depth message was added because there is no depth to name.

### Left open

`default_args` inherits into a subcommand that replaces `command:` outright, so
`dva run rails console` executes `console server -p 3000 -b 0.0.0.0`. Measured at depth 2 against
the pre-fix binary, so it is pre-existing — this fix only lets depth 3 reach the same code. Filed
as [TASK-101](../done/101-default-args-inherit-into-subcommands-that-replace-the-command.md) and
pinned by `TestInteractionDefaultArgsInheritIntoSubcommands` so fixing it cannot silently change
what depth-3 commands run.

`internal/runner/interaction_tree.go` reports gofmt drift, in the `ResolvedCommand` comment
alignment at :15-18. Pre-existing — `git show HEAD:` of the same file drifts identically — and
owned by TASK-078, so it was left alone rather than reformatted inside a behaviour fix.

## Related

- [TASK-096](096-manifest-static-commands-undercounts.md) and
  [TASK-097](../done/097-interaction-usage-mishandles-keys-with-spaces.md) — the other two defects in the
  same flat `parent + " " + child` key space. 097 in particular is the reason this key encoding is
  worth revisiting rather than patching three times.
- [TASK-101](../done/101-default-args-inherit-into-subcommands-that-replace-the-command.md) — the
  merge-semantics defect this fix surfaced but did not cause.
