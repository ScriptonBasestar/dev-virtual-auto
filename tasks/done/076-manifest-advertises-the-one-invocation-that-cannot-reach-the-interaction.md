---
id: TASK-076
title: "`dva ls` and `dva manifest` advertise a conflicting interaction unmarked, and the manifest prints the one invocation that cannot reach it"
type: fix
priority: P2
effort: M
status: done
created-at: 2026-07-30T00:00:00+09:00
scope: "internal/cli — manifest.go, list.go; internal/config — reserved.go, validate.go; tests; USAGE.md"
---

# Task 076: Do not advertise an interaction that the advertised command will not run

## Problem

An `interaction:` key colliding with a built-in was described four different ways, none a
superset of the others. `dva manifest` — the LLM-facing document — emitted `build` under both
`static_commands` and `dynamic_commands`, the latter carrying `usage_example: "dva build"` with
no marker and exit 0. `usage_example` promises that running it invokes the entry it sits inside,
and `dva build` was the one form that did not. `dva ls` had the same shape for a human reader,
and the load-time warning called the interaction ignored, which is false.

## Resolution

`cli.Execute` rewrites `dva <name>` → `dva run <name>` only when name is not a built-in, so
the accurate statement is **shadowing**, not being ignored — the declaration survives and
`dva run <name>` reaches it. Two predicates now carry that:

- `config.ShadowedByBuiltin(name, cmd)` — does the bare form reach this interaction?
- `config.ConflictAdvice(name)` — what happened, and the way out.

`ValidateReservedCommands` calls the first and `Validate` calls the second, so the validator and
the surfaces that describe a conflict cannot disagree again.

### The kinds of conflict, measured against `bin/dva` 0.1.44

| `interaction:` key | `validate` | bare `dva <key>` | `dva run <key>` |
| --- | --- | --- | --- |
| `build` + `command:` (hookable) | 1 | built-in runs | **interaction runs** |
| `status` + `command:` (not hookable) | 1 | built-in runs (`DVA v0.1.44`) | **interaction runs** |
| `build` + `replace:` (hook form) | **0** | dispatches to the hook, not the compose build | interaction runs |
| `build fast` (sub of either parent) | as parent | built-in runs, `fast` ignored | **subcommand runs** |
| `app:build` (reserved prefix) | 1 | not a built-in → 1 | `subproject 'app' not found` → 1 |

Advice per row comes from `ConflictAdvice`'s three branches; its `app-build` rename was executed
and runs.

Three findings the problem statement lacked:

**The hook exemption.** A hookable built-in declaring `before`/`replace`/`after` is not a
conflict — `dva build` provably dispatches to the hook, not the default build. Keying the mark off
`IsReservedCommand` alone, as the original fix shape proposed, would have stamped
`shadowed_by_builtin` on correct hook declarations whose `usage_example: "dva build"` is right.
The predicate asks "does the bare form reach this?", which also excludes `app:build` for free.

**A namespaced key is reachable by nothing.** Found while writing the new warning, which offered
`dva run app:build` — exit 1. The old wording was accidentally *correct* here and the accurate
rewrite wrong. `ConflictAdvice` branches on it and does not print the failing form in full: a
machine consumer scanning for a `dva run …` form would lift it out of the negation.

**Subcommands are shadowed independently of their parent.** Tree keys are space-separated
(`build fast`), so the whole-key check missed them and the first fix still printed
`usage_example: "dva build fast"` — that runs the built-in with an argument. A hook-form parent
does *not* rescue its children either: measured, `dva build fast` dispatches to the parent's
`replace:` hook and ignores `fast`.

### Surfaces

- `manifest.go` — `usage_example` and `shadowed_by_builtin` both come from the shared
  `interactionUsage`. The field is set **only** when shadowed, so its presence is the signal, and
  its value names a real `static_commands` key (asserted in the same document).
- `list.go` — the one source for both surfaces. The text row is marked in all three shapes
  `printTable` produces (detailed, plain-with-description, plain).
- `reserved.go` — `FormatConflictWarnings` emits one sorted clause per conflict. It detailed
  `conflicts[0]` only, a map-iteration artifact: the same config named a different command run to
  run, and the kinds do not share advice.
- `USAGE.md` — the "**충돌** — 무시됨" rows became the validate outcome plus a "도달하는 호출"
  column. `충돌은 에러가 아니라 경고` was false in both halves and is gone; the doc now states
  that `validate`/`config validate` exit 1 while `ls`/`manifest`/`run` exit 0 on that config.
  Its hook example also used `- run:` with no `step:`, which `validate` rejects though the runner
  executes it — the repo's only two such lines, now `step:`/`run:` pairs like `examples/`
  ([TASK-083](../todo/083-a-step-without-run-announces-work-it-never-does.md)).

## Evidence

Mutation checks — a not-contains assertion passes trivially if the path is never reached:

| mutation | tests that must fail | result |
| --- | --- | --- |
| `interactionUsage` returns `dva <k>` unconditionally | the 4 mark tests (6 subtests) | FAIL; the 3 leave-alone tests passed |
| drop the hook exemption from `ShadowedByBuiltin` | both hook tests | FAIL; also 2 pre-existing config tests |
| namespaced branch offers `dva run <key>` again | both advice tests | FAIL |
| non-hookable branch reverts to the old wording | both advice tests | FAIL |
| `FormatConflictWarnings` details `conflicts[0]` only | the stability test | FAIL |
| drop the nesting branch from `interactionUsage` | both subcommand subtests | FAIL, mark tests unaffected |

Each restore verified with `diff -q` reporting identical. Every invocation named in a new message
or doc sentence was executed: `dva run build|status|build fast`, `dva status`, `dva build` on four
hook fixtures (`step:`-only, `run:`-only, `step:`+`run:`, bare), `dva app-build`, `validate`,
`config validate`, `ls`, `manifest`.

## Non-goals

- Which command wins is unchanged; built-in shadowing is designed behaviour.
- The conflicting entry stays in `ls`/`manifest`. A user who declared it needs to see dva received
  it; silence would be worse than a wrong label.
- No `--json` error envelope — [TASK-079](../todo/079-json-flag-does-not-cover-failures.md).

## Acceptance criteria

- [x] A reserved-name interaction carries a machine-readable mark in the manifest | verify: `go test ./internal/cli/ -run TestManifestMarksReservedInteraction`
- [x] Its `usage_example` is the form that reaches it | verify: `go test ./internal/cli/ -run TestManifestUsageExampleReachesTheInteraction`
- [x] A non-reserved interaction is unchanged | verify: `go test ./internal/cli/ -run TestManifestLeavesNonReservedInteractionAlone`
- [x] A hook-form reserved name is NOT marked and keeps the bare form | verify: `go test ./internal/cli/ -run 'TestManifestDoesNotMarkHookFormReservedName|TestLsTextLeavesHookFormUnmarked'`
- [x] A subcommand of a reserved parent is marked, under both parent shapes | verify: `go test ./internal/cli/ -run TestSubcommandOfReservedParentIsMarked`
- [x] `dva ls --json` exposes the same mark | verify: `go test ./internal/cli/ -run TestLsJSONMarksReservedInteraction`
- [x] `dva ls` text output marks the row | verify: `go test ./internal/cli/ -run TestLsTextMarksReservedInteraction`
- [x] Advice names no invocation that refuses | verify: `go test ./internal/config/ -run TestConflictAdviceNamesOnlyInvocationsThatWork`
- [x] The warning is stable and covers every conflict | verify: `go test ./internal/config/ -run TestFormatConflictWarningsIsStableAndCoversEveryConflict`
- [x] The mark tests are not vacuous | verify: `human — the 6 mutations above; each failed only the tests that assert what it broke`
- [x] No message here claims the interaction is ignored | verify: `/usr/bin/grep -c 'will be ignored' internal/config/reserved.go` — print the count, expect 0
- [x] No message here names a config filename | verify: `/usr/bin/grep -c 'conflict in dva.yml' internal/config/validate.go` — print the count, expect 0
- [x] The doc no longer calls the conflict a warning | verify: `/usr/bin/grep -c '에러가 아니라 경고' USAGE.md` — print the count, expect 0
- [x] The documented severity matches the binary | verify: `human — validate 1, config validate 1, ls/manifest/run 0, all executed on the conflict fixture`
- [x] Full suite passes | verify: `make test`
- [x] Binary builds | verify: `make build`

## Left open

- **`app:build` is still advertised unmarked.** `manifest` prints `usage_example: "dva app:build"`
  for a key no invocation reaches — this task's defect, in the branch the non-goals excluded.
  `ShadowedByBuiltin` is correctly false: nothing shadows it, it is unroutable, so the surfaces
  need a third state rather than a reuse of this one.
- **`validate` is fatal while the load path proceeds.** `ls`/`manifest`/`run` exit 0 on a config
  `validate` rejects — invalid and running at once. Accidental rather than chosen; USAGE.md
  documents it as-is.
- **`USAGE.md` is 729 lines / 27.6KB** vs the 500-line / 10KB standard — over before this change
  too (714 / 26.3KB). Splitting it is its own task.

## Related — the loop that should have caught this

`ref-evaluation.md`'s `lifecycle_boundary` surface only discovers services owned by two of stack,
plans, applications, interaction — never `interaction:` against the **built-in command namespace**,
so dva's answer here was never scored. Same gap as
[TASK-082](../decision/082-the-dogfood-loop-cannot-score-an-absent-section.md); adding the reserved
set as an owner changes `case_manifest_hash`, so stage 60 must treat the next run as a cross-run
promotion.

## Related

- [TASK-074](074-app-subcommands-answer-an-absent-section-three-ways.md) — the absent-section
  half of the same problem; shares the "no config filename in the message" constraint.
- [TASK-073](073-version-error-blames-the-config-for-a-build-defect.md) — precedent for the
  mutation check on not-contains assertions.
- [TASK-067](067-version-field-rule-stated-three-incompatible-ways.md) — precedent for one rule
  stated in mutually incompatible ways across code and docs.
