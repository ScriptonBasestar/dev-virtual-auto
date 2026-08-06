---
id: TASK-074
title: "The seven `dva app` subcommands answer an absent `applications:` section three different ways, none of which routes the user anywhere"
type: fix
priority: P2
effort: S
status: done
created-at: 2026-07-30T00:00:00+09:00
scope: "internal/cli — app.go (the empty-applications path in ls/up/stop/down/restart/build/log) + app tests"
verified-at: 2026-08-03T12:15:00+09:00
archived-at: 2026-08-03T12:15:00+09:00
verification-summary: |
  Ran all seven `dva app` subcommands (ls/up/stop/down/restart/build/log, bare and named)
  against a scratch fixture (stack:1, interaction:1, no applications:) outside any git repo.
  All bare forms: exit 0, identical advice line on stderr. All named forms: exit 1, name the
  target, no "not found" language, no config filename. `dva up --dry-run` (the suggested
  route) exits 0. Code inspection confirms noApplications() is the single call site for all
  seven RunE bodies, sortedPlanNames() was extracted into plan_lifecycle.go and is shared by
  three call sites, and no stray "no applications defined" literal remains anywhere in the
  repo (grep -c = 0). Targeted go test run of all 7 acceptance-linked tests (13+13+5+1
  subtests across TestAbsentApplications*) all PASS. Follow-ups TASK-079 and TASK-081 are
  both already done; TASK-082 remains correctly tracked as an open decision task.
---

# Task 074: One answer for the empty `applications:` section, and make it a route

## Problem

A user ran `dva app` in a stack-only devbox project, saw seven subcommands offered, ran
`dva app up`, and got `ERROR: no applications defined in dva.yml`. A dead end: it states what is
missing and stops — no next command, no statement of what the config *does* declare. `dva app`
had just advertised `up`, so the failure reads as a defect in dva rather than as "this project has
no apps, use the stack."

Underneath it, one condition was handled three incompatible ways:

| subcommand | site | behaviour |
| --- | --- | --- |
| `ls` | `app.go:39-42` | stderr `No applications defined in dva.yml`, **exit 0** |
| `up` `stop` `down` `restart` `build` | `app.go:61,78,99,180,219` | `ERROR: no applications defined in dva.yml`, **exit 1** |
| `log` | no guard at all | falls through the status loop to `app.go:359` → `ERROR: application 'myapp' not found`, **exit 1** |

Capital `No` versus lowercase `no`, exit 0 versus exit 1, and `log` asserting something false — it
reported the named app as missing from a set of apps when the set itself did not exist. Five copies
of one literal, one divergent copy, one gap.

`ls` returning exit 0 is the tell that the contract was never decided: whoever wrote `ls` concluded
nothing-to-list is not a failure, and that conclusion never propagated.

## Evidence of the defect (verified 2026-07-30)

Against `bin/dva` 0.1.44, on a `dva.yml` declaring one `stack:` entry and one `interaction:` and no
`applications:` — now `noAppsConfig` in `internal/cli/app_absent_test.go`. `dva validate` printed
`✅ dva.yml is valid`, so none of this was an artefact of a malformed config. Run outside a git
repo so the `.gitignore` warning ([TASK-080](080-gitignore-warning-preempts-every-command.md)) did not interleave:

```
=== dva app ls ===          exit=0   stderr: No applications defined in dva.yml
=== dva app up ===          exit=1   stderr: ERROR: no applications defined in dva.yml
=== dva app up myapp ===    exit=1   stderr: ERROR: no applications defined in dva.yml
=== dva app build ===       exit=1   stderr: ERROR: no applications defined in dva.yml
=== dva app log myapp ===   exit=1   stderr: ERROR: application 'myapp' not found
```

Three facts found while checking what the message *could* point at, each of which constrained the
fix:

1. **`dva ls` is not the discovery command it sounds like.** It lists `interaction:` only —
   verified on the fixture, and `dva ls --json` agrees. A message saying "run `dva ls` to see what
   is declared" would be wrong except when interactions are all there is.
2. **`dva stack` is not the route either.** It has no `ls` (`dva stack ls` prints the parent help
   and exits 0), and `USAGE.md:157` says "`stack`은 선언 저장소이므로 `dva stack up`은 더 이상
   권장 모델이 아닙니다".

## The section this command serves is itself migration-only

`shared-guardrails.md` rule 33: "Legacy `dva app` and `--mode` behavior is migration-only." Rule 27
models native app processes as stack runners selected from plans, rule 2 requires named `plans:` in
new configs, and `ARCHITECTURE.md:78`'s domain-boundary table has no `applications` row.

So a config with no `applications:` is not incomplete — it is the shape the project's own
generation rules require. `dva app` advertised seven subcommands for a section its own guardrails
forbid generating, and the message treated the compliant state as the user's mistake. That put the
routing half of this task above the consistency half.

Same class as [TASK-073](073-version-error-blames-the-config-for-a-build-defect.md), aggravated:
this reader is not confused about a dormant branch, they are stopped in a project where nothing is
wrong. That this path was never walked shows in the same file — the port-conflict error at
`app.go:162` and the `dva init` hint at `root.go:236` both got routes, because someone met them.

## What shipped

`noApplications(c, action, names)` in `app.go`, called by all seven subcommands. Bare invocation
prints to stderr and exits 0; a named target returns an error naming the target and what it would
have done. `log`, which had no guard, got one.

The advice line comes from `absentApplicationsAdvice`, which states what the config declares with
counts and then routes. **The route is not unconditional.** This task's original sketch said route
to `dva up` always; running it on three fixture shapes proved that would be a broken suggestion in
two of them, so the advice branches:

| config declares | route | why not bare `dva up` |
| --- | --- | --- |
| `default_plan` | `dva up <default>` | — |
| plans, no default | `dva up <a\|b>` | bare form errors: `multiple plans configured; specify one` |
| stack only | `dva up` | — |
| interactions only | `dva ls` | bare form prints `Lifecycle: (no entries configured)` and does nothing |
| nothing | no route; says so | there is nothing to route to |

Never names a config file (config is the merge of `modules:` and `subprojects:`, so the file that
would need the `applications:` block is not knowable from the loaded config — the same misdirection
TASK-073 removed). Never routes to `dva stack up` or to another `dva app` subcommand.

`sortedPlanNames(c)` was extracted in `plan_lifecycle.go` because the multi-plan branch needs the
same sorted name list that `requirePlanSelection` and `rejectUnknownPlanArg` already built
separately — adding a third copy of that loop while removing five copies of a literal would have
been the same defect this task exists to fix. Net −4 lines there. Sorting is load-bearing: `c.Plans`
is a map, so an unsorted message differs between runs.

In `up`/`restart`/`build` the guard sits **after** `parseDvaFlags` and the `--dev`/`--docker` name
filter, because those commands set `DisableFlagParsing: true` and the target names are not known
before that. It still precedes `resolveMode`, the mode header, and `suggestProvision`.

## Non-goals (held)

- No `applications:` section was added to any target project. The absence is correct.
- `dva ls` was not changed to list stack entries — separate contract, filed as
  [TASK-081](../todo/081-config-discovery-is-split-across-show-and-status.md).
- No `--json` error envelope — filed as [TASK-079](079-json-flag-does-not-cover-failures.md).
- The port-conflict message at `app.go:162` was not touched. It is the model, not the target.

## Acceptance criteria

> **Retired by the command-surface restructure (`docs/43`).** `dva app` was removed, and
> `app.go` and `app_absent_test.go` went with it, so the five `go test -run` bindings below
> now select nothing. Left as executable commands they would print "no tests to run" and
> exit 0 — a reader checking this task's history would be told it still verifies. The test
> names are kept in the notes so `git log -S` can still find what ran.

- [x] Bare `dva app up` on a config with no `applications:` exits 0 and names a runnable command | verify: `human — was TestAppUpBareOnAbsentApplications; command removed, see note`
- [x] `dva app up NAME` on the same config exits 1 and names NAME | verify: `human — was TestAppUpNamedOnAbsentApplications; command removed, see note`
- [x] `dva app log NAME` no longer reports the app as "not found" | verify: `human — was TestAppLogOnAbsentApplications; command removed, see note`
- [x] No message on this path names a config filename | verify: `human — was TestAbsentApplicationsMessageNamesNoFile; command removed, see note`
- [x] The not-contains assertion is not vacuous | verify: `human — reinstate "in dva.yml" in the helper; the test must FAIL`
- [x] The five duplicated literals are gone | verify: `/usr/bin/grep -c 'no applications defined' internal/cli/app.go` — printed `0`
- [x] All seven subcommands answer through the one helper | verify: `human — read the seven RunE bodies; each empty-section exit goes through the helper`
- [x] The route is the current model, not another legacy command | verify: `human — was TestAbsentApplicationsRoutesToCurrentModel; command removed, see note`
- [x] The suggested command actually works on a plan-less config | verify: `human — dva up --dry-run on the repro fixture, exit 0`
- [x] Full suite passes under -race | verify: `make test`
- [x] Binary builds through the project's own path | verify: `make build`

## Evidence of the fix

End-to-end on the stack-only fixture, and each suggested command then executed verbatim at exit 0:

```
dva app ls         exit=0  no applications declared; this config declares stack (1), interaction (1) — run 'dva up' to start the declared lifecycle
dva app up         exit=0  (same line)
dva app up myapp   exit=1  ERROR: no applications declared, so there is no 'myapp' to start; … run 'dva up' …
dva app log myapp  exit=1  ERROR: no applications declared, so there is no 'myapp' to show logs for; …
```

The two-plan and interaction-only branches were exercised the same way, each on its own fixture.

Both not-contains assertions were mutation-checked, because either passes trivially if the path is
never reached. `app.go` was restored from an untouched copy after each, `diff` confirming identical:

| mutation | result |
| --- | --- |
| reinstate a filename in the advice | 12 subtests FAIL |
| route to `dva app up` | 12 subtests FAIL |
| make the bare form return an error | 6 subtests FAIL |

Green: `make build`, `make test` (`-race -cover`; `internal/cli` 58.1%, `internal/config` 62.7%),
`make test-integration` (17 PASS), `go vet ./...`, `golangci-lint run ./...` → `0 issues.`

## Left open

- **The loop that should have caught this.** `workflows/dva-dogfood/ref-evaluation.md` files a
  surface with no instance under `evaluation.not_applicable_surfaces`, so dva's answer to an absent
  section is never scored. Needs a decision, filed as
  [TASK-082](../decision/082-the-dogfood-loop-cannot-score-an-absent-section.md).
- **The counts are inlined.** `absentApplicationsAdvice` carries `stack (1), interaction (1)`
  itself because no single command lists those sections. Once TASK-081 lands, the message can
  route to that command instead of restating the knowledge — and the bare form still writes to
  stderr ignoring `--json`, like everything else on this path (TASK-079).
