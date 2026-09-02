---
id: TASK-267
title: "Repair grammar-independent subproject exposure defects"
type: bug
priority: P1
effort: M
exec-tier: standard
created-at: 2026-09-03T00:20:00+09:00
source: "TASK-259 section 5 items 2, 3, 6 and its Troubleshooting Log"
scope: "subproject usage_example resolution, run.go recovery hint, literal-key documentation claim, and eager-load error masking"
status: todo
depends-on: [TASK-259]
---

# Task 267: repair subproject exposure defects

## Summary

Fix the four defects that [TASK-259](259-discover-qualified-project-addressing.md) confirmed against
source and that its §5 declares "separable from any addressing decision — they are correct under every
option". TASK-263 owns representation and routing; this card owns only what is provably wrong today.

## Problem

Each item below is a disagreement between what a surface prints and what the runtime does.

1. `buildManifestSubprojectCommands` (`internal/cli/manifest.go:536-553`) emits
   `usage_example: dva <project>:<key>` unconditionally and never sets `ShadowedByBuiltin` or
   `Unroutable`. The root path resolves the same field through `interactionUsage`
   (`manifest.go:441-450`) precisely because, as the comment at `manifest.go:437-440` records, the
   unconditional form "for a shadowed key was the one form that provably ran something else — a
   different command with a different description, in the same document, silently". The subproject
   path still carries that defect.

   **This is new code, not a call to an existing helper.** `interactionUsage` and
   `config.ConflictAdvice` (`internal/config/reserved.go:231`) set the standard to meet — every
   invocation they name was executed against the binary — but both take a single *root* key, know
   nothing about subprojects, and never emit a `--project` form. The
   `dva run --project <project> <item>` fallback has to be written here. Scoping this item as
   "route the subproject path through the existing root helper" will not satisfy criterion 1.
2. `internal/cli/run.go:115` tells the user to run `dva ls --project <project>` when a subproject
   command is not found. `lsCmd` registers only `--format`/`-f` and `--detailed`/`-d`
   (`internal/cli/list.go:45-46`); `--project` is registered on `runCmd` alone (`run.go:144`). The
   advertised recovery command exits non-zero with an unknown-flag error.
3. `USAGE.md` and the `LiteralKeyWins` comment claim a parent has no literal `p:item` key of its own.
   The warning that exists for exactly that case proves the claim false.
4. `cli.Execute` (`internal/cli/root.go:195`) discards the `loadConfig()` error and hands control to
   cobra. One `subprojects:` entry with a missing `path` therefore makes the parent's own local
   interactions fail as `unknown command "hello" for "dva"`, masking the real cause. `dva run hello`
   prints the true error, so the masking is specific to the bare form.

## Completion Criteria

- [ ] `manifest.subprojects.*.commands.*.usage_example` is computed against the parent namespace and falls back to `dva run --project <project> <item>` when the `:` form is shadowed; the entry carries the same shadow/unroutable markers the root `dynamic_commands` path already sets | verify: `go test ./internal/cli -count=1`
- [ ] A regression test pins a shadowed subproject key and asserts the emitted `usage_example` invokes that entry rather than the shadowing builtin | verify: `go test ./internal/cli -count=1`
- [ ] `run.go:115` names an invocation that exists; if the chosen repair is to give `ls` a `--project` flag instead, the flag is registered and covered by a test rather than only documented | verify: `go test ./internal/cli -count=1`
- [ ] `USAGE.md` and the `LiteralKeyWins` comment no longer claim a parent cannot own a literal `p:item` key, and cite the warning that disproves it | verify: `make doc-check`
- [ ] A `subprojects:` entry with a missing `path` no longer masks the parent's own local interactions on the bare form; the surfaced error names the failing subproject and its path | verify: `go test ./internal/cli -count=1`
- [ ] Repository gates pass | verify: `make lint && make test && make test-integration && make doc-check && make commit-check`

## Non-goals

- No addressing grammar, alias, registry, or auto-import change — TASK-263 owns representation and routing.
- No owner field or canonical-vs-alias marker on `ls --json`/`manifest` (TASK-259 §5 item 1) and no completion work (item 4); both belong to TASK-263.
- No decision on the two open questions in TASK-259 §5 item 5.
