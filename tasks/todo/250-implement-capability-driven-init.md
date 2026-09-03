---
id: TASK-250
title: "Implement evidence-based init for people and agents"
type: feature
priority: P1
effort: L
exec-tier: standard
created-at: 2026-09-01T19:26:00+09:00
source: "TASK-249 decision"
scope: "init discovery and generation, plan preset integration, skills/workflows, fixtures, usage documentation"
status: todo
depends-on: [TASK-244, TASK-249]
---

# Task 250: implement capability-driven init

## Summary

Generate only verified, self-contained plans through one canonical path shared by people and agents.

## Problem

`init` must offer a useful shared starting point without inventing capabilities or turning measured
plan names into a schema-level vocabulary.

## Completion Criteria

- [x] Implement TASK-249's compose-only, native-only, hybrid, no-discovery, and public argv/help compatibility outcomes for all five templates, four flags, `config init`, and the top-level alias using one canonical generation path | verify: `/usr/bin/grep -Eq '^func TestInitPublicSurfaceCompatibility\(' internal/cli/init_test.go && go test ./internal/cli -count=1`
- [x] Every generated plan contains a verified self-contained entry closure; absent evidence omits the plan instead of emitting an empty or placeholder plan | verify: `go test ./internal/cli -count=1`
- [x] Generated configurations pass config validation, merged show, explicit lifecycle selection, and the decided bare lifecycle default behavior | verify: `make test-integration`
- [x] Existing config files are never overwritten implicitly; preview/idempotence and conflicting discovery behave exactly as TASK-249 decided | verify: `go test ./internal/cli -count=1`
- [x] Generated output does not immediately trigger D6/D7 and never authors `local-infra`, `local-dev`, or `full-stack`; those names survive only when already declared by the user | verify: `/usr/bin/grep -Eq '^func TestInitDoesNotAuthorRejectedPlanLabels\(' internal/cli/init_test.go && go test ./internal/config ./internal/cli -count=1`
- [x] Human CLI and agent skill/workflow consume the same canonical preset/generator and generated projections remain reproducible | verify: `make check-generate`
- [x] Usage docs explain evidence, omissions, editing, and default selection without presenting corpus frequency as a contract | verify: `make doc-check`
- [x] Repository gates pass | verify: `make lint && make test && make test-integration && make check-generate && make commit-check`

## Non-goals

- No migration of existing devboxes.
- No fixed archetype labels in schema or validation.
- No guessed native command or runner.

## Decision Record (2026-09-03/04, implementation)

**Discovery outcome design.** `internal/cli/init_scaffold.go` adds
`classifyDiscovery(dir)`, which inspects a directory for two independent kinds of
verified evidence — Compose files and language manifests (`Gemfile`, `package.json`,
`requirements.txt`/`Pipfile`/`pyproject.toml`, `go.mod`) — and classifies the result
into four outcomes:

- **compose-only**: unchanged existing behavior, generates the Compose stack entry.
- **native-only**: a language manifest exists but no Compose file. No `stack:` entry
  is generated — DVA never invents a native `run`/`build` command (the card's
  explicit non-goal) — instead `generateNativeOnlyConfigIn` writes a self-contained,
  comment-only scaffold explaining exactly what evidence was insufficient and how a
  human adds a native runner by hand.
- **hybrid**: both exist. The Compose evidence wins for the generated stack (it is
  verified and executable); the native manifest is reported but stays informational
  only, for the same reason as native-only.
- **no-discovery**: neither exists. No file is written (this is the pre-existing
  behavior, now named and covered by an explicit branch/test rather than falling out
  of an `if`/`else` by accident).

All four outcomes, all 5 templates (`minimal`, `rails`, `node`, `python`, `go`), all 4
flags (`--recursive`, `--devcontainer`, `--all`, template selection), `config init`,
and the top-level `init` alias are exercised by
`TestInitPublicSurfaceCompatibility` in `internal/cli/init_test.go`.

**native-only/hybrid are not TASK-249's "incomplete/conflicting" case — they are
evidence-backed distinct outcomes.** TASK-249's Recommended direction says
conflicting/incomplete discovery should preview only and write nothing; the
no-discovery outcome above is that case. native-only and hybrid are deliberately
*not* treated the same way, and that is not a gap against TASK-249: this card's own
completion criterion 1 requires discovery-evidence fixtures for
"compose-only, native-only, hybrid, and no-discovery" to each define "generated
entries/plans" — i.e. native-only and hybrid are expected to produce generated
output, not an omission, because each has independently verified evidence (a
language manifest, a Compose file, or both) rather than the absence or contradiction
of evidence. "Incomplete/conflicting" in TASK-249's sense is the no-discovery case
(nothing verified) and the true-conflict case is not reachable here: Compose
evidence and native-manifest evidence are never contradictory, only additive, so
hybrid always has a well-defined resolution (prefer the verified, executable Compose
stack) rather than an ambiguous one requiring a human decision. There is
consequently no preview/dry-run mechanism to build here — `scaffoldDvaYml` either
writes a fully generated file from verified evidence, or returns a plain error and
writes nothing; it never stages output for review before writing.

**Single-plan default — no new logic needed.** TASK-249 decided: one plan per
verified closure, bare/implicit default for a single plan, explicit `default_plan`
only when evidence justifies 2+ independent plans. The existing generator already
never emits a `plans:` block, and bare lifecycle commands (`dva up` with no plan
argument) already default to "start every declared stack entry" when no `plans:`
exist — this was true before this task and required no change. `classifyDiscovery`
identifies at most one closure per directory (Compose *or* native, never two
independently verified stacks), so `default_plan` emission never triggers for this
generator today; `USAGE.md` documents this explicitly rather than presenting
multi-plan support as built. If a future task teaches discovery to find multiple
independent verified closures in one directory, `default_plan` selection will need
its own follow-up — out of scope here since no such evidence path exists yet.

**Criterion 3 coverage — what "merged show" and "explicit lifecycle selection" mean
here.** `internal/integration` cannot import `internal/cli` (its generator functions
are unexported and the package boundary is intentional), and no existing integration
test spawns the compiled `dva` binary. Added
`TestGeneratedInitConfigResolvesBareLifecycleDefault` in
`internal/integration/init_generated_config_test.go`, which loads a config shaped
exactly like this generator's compose-only output (one verified stack entry, no
`plans:`) through `config.Load` + `Validate()`, and asserts
`DefaultPlanSource() == "none"` (the bare-lifecycle-runs-every-stack-entry path) and
`stack.compose.default_runner == "compose"` (an explicit, evidence-backed lifecycle
selection, not a guess). This is deliberately narrower than a full CLI-driven
`dva show`/`dva up` exercise — that would require spawning the built binary, which no
existing integration test does — but it directly pins the config-level contract
criterion 3 names (validates, resolves the decided bare default, keeps the runner
selection explicit) using the same `config.Load`/`Validate`/`DefaultPlanSource`
machinery `dva show`/`dva up` themselves rely on. Recorded here rather than silently
treating the pre-existing `make test-integration` pass as sufficient, since before
this task no integration-level test exercised generator-shaped output at all.

**Byproduct bug fix.** Running `Validate()` against generated output (for the new
tests above and in `TestInitPublicSurfaceCompatibility`) surfaced a pre-existing
latent bug: the `rails` template's `console:` interaction key and the `go` template's
`run:`/`build:` keys collide with `internal/config/reserved.go`'s reserved-command
list, which `ValidateReservedCommands` hard-rejects. No prior test ran `Validate()`
against these two templates' generated output, so this was never caught. Fixed by
renaming to `rails-console:`, `dev:`, and `build-app:` in `internal/cli/init.go`
(descriptions/commands/service fields unchanged). Grepped `internal/cli/*_test.go`
for the old key names to confirm nothing else depended on them.

**Census owner/cadence/change-threshold (criterion 9 in the source card).** This item
is TASK-249's completion criterion 9, not one of TASK-250's 8 listed criteria above
— TASK-249 (`decision-status: decided`, `needs-human: true`) explicitly keeps that
criterion, and its siblings 1/2/3/5/6/7/9/10, open as separate human-judgment work
even after its D8-scope decision record ("`decision-status: decided`는 사람이 답할
방향 질문이 끝났다는 뜻이지, 카드가 완료됐다는 뜻이 아니다"). It is marked
`verify: human` there specifically because a bare corpus count without a named
owner/cadence/revision-threshold is insufficient, i.e. it calls for a judgment this
implementation pass should not make unilaterally. Left untouched in
`tasks/todo/249-redesign-capability-driven-init.md`; flagging for a separate,
explicit human decision rather than writing one into a `needs-human` card as a side
effect of an unrelated implementation task.

**Out of scope / untouched.** The `am` (agent-mesh) preset corpus surface
(`agent-mesh-flows/`, `internal/cli/library_reference.txt`, TASK-233's
`local-infra` default) is unmodified — D8 binds only the Go `dva init` generator per
TASK-249's Decision Record. `USAGE.md`'s pre-existing size (1454 lines before this
task's ~21-line addition) is unchanged in kind; `make doc-check` passes because
`tools/doccheck/policy.go` exempts it as a lookup manual — splitting it is out of
scope here.
