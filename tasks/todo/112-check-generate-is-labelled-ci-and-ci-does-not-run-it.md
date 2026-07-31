---
id: TASK-112
title: "`make check-generate` is labelled `(CI)` and no CI job runs it"
type: chore
priority: P4
effort: S
status: todo
created-at: 2026-07-31T00:00:00+09:00
scope: "Makefile:88 check-generate, .github/workflows/ci.yml — the gate exists, is documented as a CI gate, and is never invoked"
---

# Task 112: a gate that says `(CI)` and is not in CI

## Problem

`Makefile:88` defines the target and names its purpose in the help line:

```make
## check-generate: Verify generated files are up-to-date (CI)
check-generate: generate
	@git diff --exit-code $(GEN_LIBRARY) $(WF_LIBRARY)/shared-guardrails.md AGENTS.md .agents/skills claude-plugin/skills \
		|| { echo "ERROR: generated files are stale — run 'make generate' and commit"; exit 1; }
```

`grep -n 'check-generate' .github/workflows/ci.yml` returns **nothing**. Every step in the file,
measured:

| job | steps |
| --- | --- |
| `test` | `make build`, `make test`, `make test-integration`, `go vet ./...` |
| `lint` | `make fmt-check`, `golangci-lint-action@v8` |
| `goreleaser-check` | `goreleaser check` |

So nothing verifies that committed generated files match their sources. The `(CI)` suffix asserts
a guarantee the repo does not have.

## Why it matters

This is latent, not broken. Measured today: `make check-generate` exits **0** and leaves no diff,
so the generated files are currently in sync. The defect is that *nobody would find out* if they
stopped being — which is the same shape as
[TASK-078](../done/078-nine-files-do-not-satisfy-gofmt-and-nothing-checks.md) (nine files drifted
because no gate looked) and
[TASK-109](../done/109-the-task-link-check-has-been-red-for-22-links-since-the-repo-moved.md) (a
check that had been red for 22 links without anyone seeing it).

The blast radius is real because the generated set spans four projections of one source:

| guarded path | what it is |
| --- | --- |
| `internal/cli/library_reference.txt` | embeddable library reference, corpus for `removed_keys_test.go` |
| `agent-mesh-flows/shared/library/shared-guardrails.md` | has `tools/libgen`-injected Go facts (reserved commands, section order) |
| `AGENTS.md` | Codex projection — a marked section rewritten by `skillgen` |
| `.agents/skills`, `claude-plugin/skills` | symlinks into `skills/` |

A stale `shared-guardrails.md` means the AUTOGEN fact blocks disagree with the Go source they were
extracted from — the exact single-source guarantee that
[TASK-061](../done/061-go-facts-hand-copied-into-flow-library.md) built `tools/libgen` to provide.
That task replaced hand-copied Go facts with generated ones; an unrun freshness check is how they
quietly become hand-copied again.

## A second, smaller finding in the same file

CI restates commands the Makefile already owns:

- `- name: Vet` / `run: go vet ./...`, while `Makefile:54` defines `vet: go vet ./...`
- the `lint` job uses `golangci-lint-action@v8` with a pinned version, while `Makefile:44`
  defines `lint` to run the mise-pinned binary

Two owners for one command means they can drift — CI can pass a check the developer's `make lint`
does not run, and vice versa. The `Format` step added under TASK-078 deliberately went the other
way (`run: make fmt-check`, with the gofmt invocation living only in the Makefile).

## Options

- **A — add `make check-generate` to the `test` job.** One step, uses the Go toolchain already set
  up. Turns a latent gap into an enforced invariant. Cost: any PR that edits `skills/` or the
  library sources without running `make generate` now goes red — correct, but it is a new failure
  mode for contributors who did not know the target existed.
- **B — add it, and also collapse the duplicated commands** (`Vet` → `make vet`), so the Makefile
  is the single owner of what each gate runs. Larger diff, addresses both findings.
- **C — drop the `(CI)` label.** Zero enforcement, but the Makefile stops claiming a guarantee that
  does not exist. Cheapest honest option.

## Unverified

**Whether `check-generate` can actually fail has not been demonstrated.** It passes today, but a
gate proven only in its green state is exactly what TASK-078 and TASK-109 warn about. Verifying it
requires perturbing a source file and confirming the target goes red — deliberately not done here,
because `make generate` rewrites `shared-guardrails.md` in place (libgen injects into it), so the
probe must be reverted with care rather than casually.

## Acceptance criteria

- [ ] The gate is proven able to fail | verify: `human — perturb a source under agent-mesh-flows/shared/library/, confirm make check-generate exits non-zero naming the stale file, revert and confirm it returns to 0`
- [ ] The gate is non-vacuous | verify: `make check-generate` — it must name the paths it compared, not just exit silently; a run that compares zero paths must fail
- [ ] The label matches reality | verify: `grep -c check-generate .github/workflows/ci.yml` — non-zero under A/B, or the `(CI)` suffix is gone under C
- [ ] Nothing regenerates dirty | verify: `make check-generate && git status --porcelain` — empty
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-078](../done/078-nine-files-do-not-satisfy-gofmt-and-nothing-checks.md) — found this while
  adding the format gate; same class, and its `Format` step is the pattern to copy.
- [TASK-111](111-make-lint-reports-zero-issues-while-an-available-analyzer-has-50.md) — the third
  instance: a green gate whose coverage nobody had stated.
