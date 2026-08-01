---
id: TASK-126
title: "`make lint` is still green over 8 findings from analyzers already in the pinned toolchain, after TASK-111 closed by declaring there was no residual gap"
type: chore
priority: P2
effort: M
status: done
created-at: 2026-08-02T09:00:00+09:00
resolved-at: 2026-08-02T09:00:00+09:00
resolution: "Enabled govet enable-all (minus two named exclusions) and unparam, fixed all 8 findings; make lint and gopls now agree at 0 across 235 files, and the analyzers that stay off are listed with reasons"
scope: ".golangci.yml — govet ran its default analyzer subset and unparam was not enabled; both ship in the pinned golangci-lint 2.12.2"
---

# Task 126: the gate closed a gap and announced there were none left

[TASK-111](111-make-lint-reports-zero-issues-while-an-available-analyzer-has-50.md) found `make lint`
reporting `0 issues.` while `modernize` — present in the same pinned binary, off by default — had
47. It enabled `modernize`, fixed everything, and closed on this criterion:

> The gate's coverage is stated, not implied | verify: `.golangci.yml` names `modernize` under
> `enable:` with the reason; **no analyzer is now deliberately excluded, so there is no residual gap
> to report**

The bolded clause claimed more than TASK-111 had checked. Precisely: at analyzer granularity
nothing *was* deliberately excluded — govet's non-default set and `unparam` were never considered,
which is not the same as being kept out on purpose. What fails is the inference. A gap you never
surveyed is not a gap you decided to accept, so "nothing deliberately excluded" does not yield "no
residual gap to report".

> **Corrected by [TASK-127](127-the-record-that-closed-the-coverage-gap-had-two-of-its-own.md).**
> Two claims in this record reach past what it measured — the `gopls` criterion below and the
> "what stays excluded is named" criterion. Both are marked inline.

## Measured

`make lint` reported `0 issues.` Three analyzer sets that were already installed did not agree.
Measured 2026-08-02 on golangci-lint 2.12.2 / gopls v0.22.0:

| analyzer set | how it was off | findings |
|---|---|---|
| `govet` non-default analyzers (`unusedwrite`, …) | golangci-lint runs govet's default subset; `enable-all` was never set | **1** |
| `unparam` | ships in the pinned binary, not in `enable:` | **4** |
| `writestring` (gopls modernize) | gopls's modernize suite is a superset of the one vendored into golangci-lint 2.12.2 | **3** |
| | | **8 total** |

The first two are the exact shape TASK-111 closed: available in the pinned binary, silently off. The
third is a different and more awkward one — two tools named `modernize` that are not the same set.

## Not a regression

Nothing broke. `modernize` is still enabled and still reports 0. TASK-111's *fix* holds; its
*closing claim* was broader than what it verified. `unusedwrite` and `unparam` were available and
unreported on the day that criterion was written, so "no residual gap to report" described a survey
that had not been done.

Worth stating plainly because of where it happened: this is the repo's recurring defect class —
output that claims a coverage it does not have — appearing inside the record of a task that closed
an instance of it. A green gate is a claim about what was checked, and so is a task record.

## What the 8 were

None are crashes. Two are real defects, the rest are dead weight:

| site | finding | verdict |
|---|---|---|
| `config_test.go:1092` | `unusedwrite`: unused write to field `Modes` | **real** — the test was vacuous |
| `app_manager.go:622` | `startDockerApp` — `name` is unused | **real** — dead parameter |
| `docker_compose.go:83` | `buildStepArgs` — `env` is unused | **real** — dead parameter |
| `ssh.go:94` | `runSilent` — `name` always receives `"docker"` | speculative generality |
| `doctor_compose_test.go:112` | `writeComposeFile` — `name` always receives `"compose.dev.yml"` | speculative generality |
| `stack.go:427`, `stack.go:472`, `app.go:468` | inefficient concatenation in `WriteString` | micro |

`TestDefaultMode_Empty` is the one worth reading. It built a `Config`, then asserted that the `""`
it had just assigned was still `""` — true no matter what the validator did, and the reason `Modes`
was written but never read. It now loads a real config and calls `Validate()`, so it actually
guards the `if c.DefaultMode != ""` branch in `validate.go:152`.

`buildStepArgs` is the other one: `DockerComposeRunner`'s took an `*config.Environment` it ignored,
while `KubectlRunner`'s sibling has only ever taken the command. The env reaches the child through
`execComposeStep`, so the parameter was never wired to anything.

## Acceptance criteria

- [x] The two off-by-default analyzer sets run | verify: `golangci-lint linters` — **`unparam` now listed under "Enabled by your configuration"; `govet` runs with `enable-all: true`**
- [x] The gate fails on a reintroduced finding | verify: `human — re-add the unused param, then the unread struct write; run make lint` — **2 mutations, `unparam: 1` then `govet: 1`, `make: *** [lint] Error 1` both times**
- [x] All 8 findings fixed | verify: `make lint` — **`0 issues.`**
- [~] The editor and the gate agree | verify: `gopls check $(find cmd internal tools -name '*.go')` — **235 files, 0 findings; was 3.** ⚠️ Command too narrow: it omits `-severity=hint`, and `stringscut` reports at hint level, so this criterion could not see the one live instance of the very class the residual-gap section below calls empty. Corrected in TASK-127.
- [x] The rewritten test is not vacuous | verify: `human — delete the `if c.DefaultMode != ""` guard in validate.go` — **fails with `default_mode '' not found in modes. Available: dev`; the previous version passed**
- [x] User-facing messages unchanged | verify: `human — run the three rewritten WriteString branches against the real binary` — **all three render correctly; see Resolution**
- [~] What stays excluded is named, not implied | verify: `.golangci.yml` lists `fieldalignment` and `shadow` under `govet.disable` with a reason each, and the gopls/golangci `modernize` divergence is recorded below — ⚠️ the survey behind this was govet-only. `exclusions.presets` held four unnamed entries, one of which (`std-error-handling`) was silently dropping 6 errcheck findings including a real defect. Corrected in TASK-127.
- [x] Full suite passes | verify: `make test` — exit 0
- [x] Docs consistent | verify: `make doc-check` — `broken_links: 0`
- [x] Generated artifacts current | verify: `make check-generate` — clean

## Resolution

`.golangci.yml` adds `unparam` to `enable:` and sets `govet.enable-all: true`.

The three `WriteString` branches were exercised against the built binary rather than assumed
identical, since they are error text a user reads:

```
ERROR: no such stack entry: nosuchentry
       → defined in dva.yml: app-k8s, core-infra, frontend, web

ERROR: unknown flag "--" for "dva stack up"
       → a stack entry name cannot start with "-", so this was read as one and matched nothing
       → accepted here: --force, --no-wait, --mode, -M, --env, -E, --tag, --tags, -T, ...
```

### Mutation-tested

| Mutation | Result |
|---|---|
| Re-add unused `name` param to `startDockerApp` | `make lint` → `unparam: 1`, exit 1 |
| Re-add unread `Modes` struct write in a test | `make lint` → `govet: 1` (`unusedwrite`), exit 1 |
| Delete the `if c.DefaultMode != ""` guard | `TestDefaultMode_Empty` fails |

The first two matter more than they look: without them, `0 issues.` after a config edit is
indistinguishable from a config edit that golangci-lint silently ignored — which is the failure
mode this whole task is about.

## The residual gap, stated

Two govet analyzers stay off, with the reason recorded in `.golangci.yml` beside each:

- `fieldalignment` — reports struct field ordering for memory savings. Config structs mirror
  `dva.yml` key order deliberately; that readability is worth more than the padding.
- `shadow` — flags every shadowed `err`, which is idiomatic here.

And one gap cannot be closed from `.golangci.yml` at all: **gopls's `modernize` is a superset of the
one vendored into golangci-lint 2.12.2.** `writestring` lives only in the former, so the gate cannot
see the class and the next instance will be reported by an editor and not by CI. Measuring it means
running gopls directly:

```
gopls check -severity=hint $(find cmd internal tools -name '*.go')
```

> ⚠️ **Two corrections from [TASK-127](127-the-record-that-closed-the-coverage-gap-had-two-of-its-own.md).**
> This paragraph originally read "there are currently 0 instances" and prescribed the same command
> without `-severity=hint`. Both were wrong in the same direction: there was 1 live instance
> (`stringscut` in `internal/lifecycle/compose_error_test.go:38`), and it reports at hint severity,
> so the command written here as the remedy for the gap could not see the gap's only occurrence.
> The divergence is also finer-grained than "`writestring` lives only in gopls" — it runs *inside*
> analyzers of the same name: golangci-lint's vendored `stringscut` handles the `strings.Index`
> variant but not the `SplitN` one.

This is written down rather than left implicit precisely because leaving it implicit is what this
task exists to correct.

## Related

- [TASK-111](111-make-lint-reports-zero-issues-while-an-available-analyzer-has-50.md) — enabled
  `modernize` and closed by asserting no analyzer was excluded. Same gate, same shape, and the
  claim this task falsifies.
- [TASK-116](116-stack-override-warning-goes-to-stdout.md) — set the standard that a criterion
  counts only when the test fails on reverted code, which is why the two gate mutations above exist.
- [TASK-125](125-three-interaction-warnings-stop-at-depth-1.md) — immediate predecessor; there a
  clean `✅ dva.yml is valid` described a tree that was never fully walked, here `0 issues.`
  described a codebase that was never fully analyzed.
- [TASK-118](118-a-health-check-that-never-passes-is-still-exit-0.md) — the recurring shape all of
  these belong to: a check whose silence is read as a pass.
