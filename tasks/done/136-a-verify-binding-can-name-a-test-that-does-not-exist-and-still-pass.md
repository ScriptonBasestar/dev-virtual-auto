---
id: TASK-136
title: "A task's verify binding can name a test that does not exist and still exit 0"
type: chore
priority: P3
status: done
effort: S
created-at: 2026-08-03T12:10:00+09:00
source: "TASK-069 finalize verification — its criterion 3 binding runs zero tests and passes"
depends-on: [TASK-069]
scope: "dva repo — tasks/_archive/069-*.md, plus whatever checks task files (Makefile link-check target, internal/doccheck)"
---

# Task 136: Make a task's `verify:` binding fail when it tests nothing

## Problem

A `go test` invocation whose `-run` pattern matches no test prints `no tests to run` and
**exits 0**. A task acceptance criterion bound to such a command reports a pass while executing nothing
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

Swept every `-run` binding in `tasks/` on 2026-08-03. Go's `-run` is an unanchored regex,
so a binding is only vacuous when **no** test name matches it as a pattern, not merely
when the exact name is absent.

- 57 distinct `-run` patterns appear across the task corpus.
- 56 match at least one real test.
- 1 matches zero: `TestMigrateLegacyCompose`.

So the corpus is nearly clean today. The gap is that nothing would have told us — the
count came from an ad-hoc sweep during a finalize pass, not from any check the repo runs.

> **Superseded.** Every number above is wrong, and the reason is the point of this task:
> they came from a shell sweep, not a check. `doccheck` measures 127 binding sites (not 57
> patterns) against 1051 selectable tests (not 994), and 4 sites matched zero (not 1). See
> the Result.

## Acceptance criteria

- [x] TASK-069's criterion 3 binding names a pattern that matches real tests
      (`-run TestMigrate` matches 11, not the 7 estimated above) — corrected in place in
      `tasks/_archive/` | verify: `make doc-check`
- [x] A check fails when a task's `-run` pattern matches zero test functions. Extend
      whatever already walks task files rather than adding a new tool | verify: `go test ./tools/doccheck/ -run TestRunPatterns_flagsPlantedDefects`
- [x] The check is proven non-vacuous: it flags a deliberately planted bad binding, in the
      style of `TestGeneratorCorpusURLsDetectsPlantedDefects` | verify: `go test ./tools/doccheck/ -run 'TestRunPatterns_(flagsPlantedDefects|failsWhenTestNamesVanish|sweepsTheRealCorpus)'`
- [x] The check prints the denominator it swept (patterns examined), so an empty result
      cannot read as a pass | verify: `make doc-check`
- [x] `make test` exits 0 | verify: `make test`

## Notes

Editing a file already in `tasks/_archive/` is deliberate here: the binding is the
archived record's own evidence, and leaving it broken means the next person to re-run it
gets a green light for nothing. Fix the string, do not reopen the task.

The `-run` regex semantics matter for the checker: match each pattern against the set of
declared test names, do not compare for equality, or 56 correct prefix bindings will be
reported as failures.

## Result

`tools/doccheck` gained a fourth check. Every `go test` command written in inline code has its
`-run` pattern resolved against the tests actually declared in the tree; a pattern selecting
none fails the gate. `make doc-check` now prints `test_funcs_found`, `run_patterns` and
`unmatched_run` alongside the link counters.

```
test_funcs_found:    1056 (from 155 _test.go files)
run_patterns:        127
unmatched_run:       0
```

(The first run of the check, before any of the fixes below, read `1051 (from 154 files)` /
`125` / **`4`**. The denominators grew by this task's own test file and its two new `verify:`
bindings.)

### What the sweep found — and the one thing the task file got right by accident

Four sites matched zero tests, not one. Two were real, two were the checker misreading prose:

| Site | Diagnosis | Resolution |
| ---- | --------- | ---------- |
| `tasks/_archive/069:144` | `-run TestMigrateLegacyCompose` — no test ever had that name; the command exits 0 having run nothing | `-run TestMigrate`, which selects 11 real tests in that package, 7 of them exercising a compose shape |
| `tasks/136:18` | this file's own illustration named a test that does not exist, in a span shaped exactly like the defect it describes | stated in prose instead; the fenced `verify:` quote below it was already invisible to the check, since fenced blocks are stripped |
| `tasks/_archive/105:154` | `-run <the new tests>` — a prose placeholder inside a command span | placeholder moved out of the span |
| `tasks/_archive/059:170` | **not a defect** — see below | checker corrected |

The 059 case is the one worth recording. It reads
`-run 'TestCheckSubprojectComposeProjectNames\|TestSameStringSet'`, and the first reading
was that the backslash survives single quotes and reaches Go's regexp as an escaped pipe —
a literal `|`, matching a test named `TestA|TestB`, which is nothing. That reading is right
about shell quoting and wrong about where the text is: the span sits in a markdown table,
and `\|` is the *only* backslash escape GFM processes inside a code span, because it is the
only way to put a pipe in a cell. The reader sees `-run 'TestA|TestB'`, which selects 2
tests and 14 subtests — exactly the "14 subtests PASS" the row claims.

The edit that "fixed" it was written, and would have split the table cell in two. It was
reverted when the rendered form was checked rather than the source bytes. `inTableRow`
now unescapes `\|` on table rows only, so the genuine defect — the same escape *outside* a
table, where nothing unescapes it — is still caught. Both directions are pinned by tests.

### Design notes

- **Test names come from source, not `go test -list`.** Listing needs a build, and would
  miss `internal/integration` behind its `-tags=integration` guard. The regex requires
  `*testing.T` in the signature, which is what excludes `TestMain(m *testing.M)` — `-run`
  cannot select it — and the `isRunnableTestName` filter applies Go's own rule that the
  character after `Test` must not be lower case.
- **Repo-wide, not per-package.** A pattern that matches somewhere but not in the package
  the command names is also a broken binding, but judging that means writing a second
  parser for `go test`'s package-argument grammar (`./...`, bare `./pkg`, several packages,
  `-tags` changing which files compile) inside a documentation checker. Repo-wide catches
  the class this task was filed for; narrowing is additive if it ever proves needed. This
  is a scope decision, not a measurement. Checked on the one binding this task rewrote:
  `-run TestMigrate` selects 11 repo-wide and 11 inside `./internal/config/`, because every
  `TestMigrate*` happens to live there — the two scopes agree. Whether they agree across the
  other 126 sites was not measured.
- **`go test` must appear in the span.** `-run` already has to start a word, which keeps
  `--dry-run` and the prose `re-run` out; requiring `go test` as well keeps the denominator
  to things someone would actually run.

### Falsification

| # | Break | Result |
| - | ----- | ------ |
| F1 | Rename a live binding's target: `tasks/todo/081:132` → `-run TestShowNamesStackEntriesRenamedAway` | `make doc-check` exit 1 naming the file and line; `TestRunPatterns_sweepsTheRealCorpus` FAIL |
| F2 | Point the declaration regex at `*testing.B`, so no test names are found | `test_funcs_found: 0 (from 155 files)`, every binding then present (125) reported, and the vacuity guard fires — never a silent green |

Disjoint paths: F1 exercises the per-pattern verdict, F2 the denominator. F2 is the one that
matters most — a checker that finds no test names would otherwise pass every pattern in the
repo. Both restored via `Edit`, never `git checkout --`, which would have discarded the
uncommitted work in this tree.

### Gates

```
make test          ok — cli 68.6% · config 68.3% · exec 67.0% · lifecycle 62.5% ·
                        output 100% · runner 67.8% · doccheck 72.7% → 83.3%
make doc-check      exit 0 — 229 markdown, 520 links, 127 -run patterns, 0 unmatched
gofmt -l            0 files
go vet ./...        exit 0
```

### Changed

- `tools/doccheck/verifyrun.go` — new: binding extraction, shell unquoting, GFM table-escape
  handling, test-name collection, `-run` match semantics.
- `tools/doccheck/verifyrun_test.go` — new: 7 planted defects, 10 working bindings that must
  not be flagged, 4 non-bindings that must not inflate the denominator, the vacuity guard,
  and the real-corpus floor.
- `tools/doccheck/check.go` — four counters on `Result`, the sweep wired into `Check`, the
  vacuity guard.
- `tools/doccheck/main.go` — the three new counters printed unconditionally; package comment.
- `tasks/_archive/069-*.md`, `tasks/_archive/105-*.md` — corrected bindings.
