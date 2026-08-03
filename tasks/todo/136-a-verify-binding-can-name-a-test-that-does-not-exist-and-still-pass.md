---
id: TASK-136
title: "A task's verify binding can name a test that does not exist and still exit 0"
type: chore
priority: P3
status: todo
effort: S
created-at: 2026-08-03T12:10:00+09:00
source: "TASK-069 finalize verification — its criterion 3 binding runs zero tests and passes"
depends-on: [TASK-069]
scope: "dva repo — tasks/_archive/069-*.md, plus whatever checks task files (Makefile link-check target, internal/doccheck)"
---

# Task 136: Make a task's `verify:` binding fail when it tests nothing

## Problem

`go test ./pkg/ -run TestThatDoesNotExist` prints `no tests to run` and **exits 0**. A
task acceptance criterion bound to such a command reports a pass while executing nothing
— the exact vacuous-verdict shape this repo's verification culture exists to prevent, and
the same failure mode as counting an empty grep as a clean sweep.

Found in TASK-069, criterion 3:

```
verify: `go test ./internal/config/ -run TestMigrateLegacyCompose`
```

No test named `TestMigrateLegacyCompose` exists anywhere in the repo. The behaviour it
claims to cover is genuinely tested — `internal/config/migrate_test.go` holds
`TestMigrateAutoInferredCompose`, `TestMigrateNestedCompose`,
`TestMigrateFlatComposePluginDropsPluginKey`, `TestMigrateDuplicatesTags`,
`TestMigratePreservesEverythingElse`, `TestMigrateLeavesModernConfigByteIdentical`,
`TestMigrateRefusesAmbiguousEntry`, all passing — so this is a broken binding, not a
coverage hole.

## Blast radius, measured

Swept every `-run` binding in `tasks/` on 2026-08-03 against the 994 `func Test*`
definitions in the repo. Go's `-run` is an unanchored regex, so a binding is only vacuous
when **no** test name matches it as a pattern, not merely when the exact name is absent.

- 57 distinct `-run` patterns appear across the task corpus.
- 56 match at least one real test.
- 1 matches zero: `TestMigrateLegacyCompose`.

So the corpus is nearly clean today. The gap is that nothing would have told us — the
count came from an ad-hoc sweep during a finalize pass, not from any check the repo runs.

## Acceptance criteria

- [ ] TASK-069's criterion 3 binding names a pattern that matches real tests
      (`-run TestMigrate` matches 7) — corrected in place in `tasks/_archive/`.
- [ ] A check fails when a task's `-run` pattern matches zero test functions. Extend
      whatever already walks task files rather than adding a new tool.
- [ ] The check is proven non-vacuous: it flags a deliberately planted bad binding, in the
      style of `TestGeneratorCorpusURLsDetectsPlantedDefects`.
- [ ] The check prints the denominator it swept (patterns examined), so an empty result
      cannot read as a pass.
- [ ] `make test` exits 0.

## Notes

Editing a file already in `tasks/_archive/` is deliberate here: the binding is the
archived record's own evidence, and leaving it broken means the next person to re-run it
gets a green light for nothing. Fix the string, do not reopen the task.

The `-run` regex semantics matter for the checker: match each pattern against the set of
declared test names, do not compare for equality, or 56 correct prefix bindings will be
reported as failures.
