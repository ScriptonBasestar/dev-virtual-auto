---
id: TASK-031
title: "make test-integration has been red the whole run, and CI never runs it"
type: bug
priority: P1
status: done
effort: S
created-at: 2026-07-17T02:30:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: fresh Phase 1 sweep (running a gate nobody had run)
source-severity: HIGH
---

# Task 031: A Documented Test Gate Is Permanently Red And Structurally Invisible

## Summary

`make test-integration` — documented in `CLAUDE.md` and exposed as an interaction in the repo's
own `dva.yml:51` — **exits 1 at HEAD** and has done so for this entire remediation run. Three
fixture-validation tests fail. CI never runs the target, so nothing reported it.

The gate always returns red, which means it conveys no signal: a genuine new regression is
indistinguishable from the standing failure.

## Evidence

At HEAD `e87d02b`:

```
$ make test-integration
--- FAIL: TestValidateBasicFixture (0.01s)
    validate_test.go:12: basic fixture should validate: schema validation failed in dva.yml:
      - stack.compose: Must not validate the schema (not)
      - stack: Must validate all the schemas (allOf)
--- FAIL: TestValidateFullStackFixture
--- FAIL: TestValidateProvisionProfilesFixture
FAIL
make: *** [test-integration] Error 1     EXIT=2
```

Not caused by this run's commits. Verified by checking out the **pre-run baseline** `73dc094` in
a scratch worktree and running the suite there — identical three failures:

```
$ git worktree add /tmp/dva-baseline 73dc094 && cd /tmp/dva-baseline
$ go test -tags=integration ./internal/integration/...
--- FAIL: TestValidateBasicFixture
--- FAIL: TestValidateFullStackFixture
--- FAIL: TestValidateProvisionProfilesFixture
EXIT=1
```

The breakage predates the run. (An initial hypothesis that TASK-010's schema hardening at
`5729d98` caused it was **disproved** by this baseline check — recorded because the wrong guess
is the reason the check was run at all.)

## Why nobody noticed — the structural half

`.github/workflows/ci.yml` runs exactly three things:

```yaml
- name: Build   ->  make build
- name: Test    ->  make test
- name: Vet     ->  go vet ./...
```

`make test` is `go test -race -cover ./...`, which **cannot** see these tests: they sit behind
`//go:build integration`, and only `make test-integration` passes `-tags=integration`.

So the suite is doubly hidden — behind a build tag, and outside CI. It is a safety net that
cannot catch anything.

This is the same structural theme as TASK-026, where "all 18 examples validate" was green while
shipped examples were unrunnable. There a passing check masked breakage; here an entire gate is
red and unwatched. Both are verification that reports something other than the truth.

## Root cause of the three failures

The fixtures use the **legacy** compose shape at entry level:

```yaml
stack:
  compose:
    order: 10
    files: [compose.yml]        # <-- rejected
    project_name: basic-test    # <-- rejected
```

`schema.json` deliberately rejects exactly this. Its own `stack` description states the intent:

> Named stack declarations. Compose must be declared through `stack.<entry>.runners.compose`;
> legacy `stack.<entry>.compose` and `plugin: compose` shapes are rejected by schema validation.

All three failing fixtures carry the identical shape — one migration, three files:

```
internal/integration/testdata/fixtures/basic/dva.yml             files + project_name
internal/integration/testdata/fixtures/full-stack/dva.yml        files + project_name
internal/integration/testdata/fixtures/provision-profiles/dva.yml  files
```

## Why this needs no product decision (verified, not assumed)

The tension looks like a schema-vs-loader disagreement — `Validate()` rejects the legacy shape
while the loader happily reads it (`TestLoadBasicFixture` **passes** today and asserts
`ComposeProjectName() == "basic-test"`). That raises the fair question of whether the fixture or
the schema is wrong, which would be a decision.

It is not, because **the loader accepts both shapes**. `ComposeConfig()`
(`internal/config/lifecycle_helpers.go:5-21`) checks legacy `e.Compose` first and then falls
back to `runners.compose`:

```go
if e.Compose != nil { return e.Compose }
for name, runnerCfg := range e.Runners {
    if normalizeRunnerName(name) != "compose" { continue }
    if cfg, ok := runnerCfg.(*ComposePluginConfig); ok { return cfg }
}
```

So a migrated fixture still resolves to the same `ProjectName`/`Files`, and the currently-passing
`Load` tests keep passing. The schema's stated intent and the loader's tolerance are compatible:
legacy stays supported in code for back-compat, and the fixtures simply never migrated.

Migration direction verified empirically before filing, with a control:

```
$ dva validate   # current legacy fixture, verbatim copy
  - stack.compose: Must not validate the schema (not)      EXIT=1

$ dva validate   # same fixture migrated to runners.compose
  ✅ dva.yml is valid                                       EXIT=0
```

Note the migration also requires `default_runner: compose`: the schema rejects `runners.compose`
without it. Discovered by probing, not from the docs.

## Severity: HIGH / P1 — with the honest caveat

No user-facing harm: this is a developer gate, and 7+ other integration tests pass. The failing
three are all fixture-validation.

It is still P1 because `make test-integration` **always exits 1**, so the gate's signal is
destroyed rather than degraded — no one can tell a new regression from the standing red. A dead
verification layer is the mechanism by which findings like TASK-026 reach shipped files. Cheap
to fix (three YAML files), high leverage.

## Completion Criteria

- [ ] `make test-integration` exits 0 | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && make test-integration`
- [ ] The three fixtures validate | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && go test -tags=integration ./internal/integration/... -run 'TestValidate'`
- [ ] The currently-passing Load tests still pass, still asserting the same resolved values | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && go test -tags=integration ./internal/integration/... -run 'TestLoad|TestProvision'`
- [ ] `make test` and `go vet ./...` still pass | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && make test && go vet ./...`
- [ ] CI runs the integration suite, so it cannot silently rot again | verify: `grep -q "test-integration" /Users/archmagece/mywork/scripton/dev-virtual-auto/.github/workflows/ci.yml`
- [ ] Legacy-shape loading keeps explicit coverage (the loader still supports it; migrating every fixture must not silently drop the only test of that back-compat path) | verify: `human — confirm a test still exercises the legacy stack.<entry>.compose loading path, or that dropping it is intended`

## Outcome

Fixed. Three fixtures migrated to `runners.compose` + `default_runner: compose`; a new
`legacy-compose` fixture and `TestLoadLegacyComposeFixture` keep the back-compat path covered; CI
gained an `Integration Test` step running `make test-integration`.

Verified in an isolated worktree at `af90577` (`/tmp/dva-t031`), not in the main tree — other
agents' in-flight edits were present there and would have contaminated the result.

**The bug still reproduced at current HEAD**, so nothing here rests on the stale `e87d02b` report:
reverting only the fixture migration reproduced the identical three `TestValidate*` failures,
EXIT=1.

### The control that mattered — the last criterion, discharged

Migrating every fixture off the legacy shape might silently delete coverage of the loader's
back-compat branch. Deleting `if e.Compose != nil { return e.Compose }` from `ComposeConfig()`
(`internal/config/lifecycle_helpers.go:9`) produced:

```
--- FAIL: TestLoadLegacyComposeFixture
    legacy_compose_test.go:19: ProjectName = "", want "legacy-test"
--- PASS: TestLoadBasicFixture / FullStack / ProvisionProfiles / Invalid / Nonexistent
```

Two facts, one control. The new test fails **for the right reason** (the legacy branch is what it
reads), and the other five Load tests **stay green** — proving the migrated fixtures no longer
depend on that branch, so within the integration suite this test is the only thing holding it.

### CORRECTION — this task's premise was wrong, and so was the first version of this Outcome

Criterion 6 assumed migrating the fixtures could drop "the only test of that back-compat path".
**It could not.** The same mutant run against the *unit* suite:

```
$ make test          # legacy branch deleted
--- FAIL: TestLifecycleEntryParsing, TestConfigHasTag, TestGetComposeServicesExcluding,
          TestGetExcludedComposeServices, TestValidateComposeProjectNames_{Missing,Mismatch}   (internal/config)
--- FAIL: TestBuildManifest_MinimalConfig, TestShowText_WithCompose, TestShowJSON_FullConfig    (internal/cli)
--- FAIL: TestComposePlugin_BuildArgs_Default                                                   (internal/lifecycle)
EXIT=2
```

Ten unit tests across three packages already cover the branch, and CI already runs `make test` —
so removing it was never silent. The `legacy-compose` fixture is therefore an **addition**
(the only fixture-driven, integration-level, mutation-verified cover), **not a rescue**.

Recorded because the error was mine and it is instructive: my RED control only ran the
*integration* package, observed the five sibling Load tests stay green, and I read that as "this is
now the only coverage". The evidence never supported it — a green sibling in one package says
nothing about another package. The commit message of `46d92d1` carries the same overstatement and
cannot be corrected without rewriting history; this note is the correction of record. The scope of
the claim, not the fix, was wrong: the migration and the CI step stand on their own evidence.

Credit: caught by the task031-impl subagent, which checked the mutant against `make test` when I
had not. Its report is also why criterion 6 (`human —`) is answered NO: dropping legacy fixture
coverage would not have been silent, though keeping the fixture is still worthwhile.

### Gates

```
make test-integration           EXIT=0   (was EXIT=2)
make test                       EXIT=0   5 packages ok
go vet ./...                    EXIT=0
grep -q test-integration ci.yml          Integration Test step present
```

### Checked before committing, not assumed

Adding the suite to CI trades a red local gate for a red CI if any test needs a Docker daemon. It
does not: the suite is pure config-loading — no `exec.Command`, no `Skip` guards, runs in 1.4s. The
lone `docker` grep hit (`config_load_test.go:45`) is a config *mode name*, a YAML string.

`legacy-compose` is deliberately schema-invalid and must never reach a validate test. Confirmed
safe by construction: no test enumerates the fixtures directory (`ReadDir`/`Walk`/`Glob` → none);
every validate test names its fixture literally.

`.gitignore` was **not** staged. Its ` M` state (`.sb/dva/`) predates this run and is unrelated to
this task.

## References

- [026-shipped-examples-validate-green-runtime-red.md](./026-shipped-examples-validate-green-runtime-red.md) — same theme: verification reporting something other than the truth
- [010-schema-validate-all-runners.md](../_archive/010-schema-validate-all-runners.md) — last change to `schema.json`; initially suspected, disproved by the baseline check
