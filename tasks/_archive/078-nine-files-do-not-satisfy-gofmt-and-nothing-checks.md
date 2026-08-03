---
id: TASK-078
title: "Nine files do not satisfy gofmt, one of them indents a top-level function, and no CI job checks"
type: fix
priority: P4
status: done
effort: XS
created-at: 2026-07-30T00:00:00+09:00
closed-at: 2026-07-31T00:00:00+09:00
decision: "A — run make fmt, commit whitespace alone, then gate it in CI"
scope: "repo-wide gofmt run (9 files) + the decision on whether CI should gate formatting"
verified-at: 2026-08-03T12:30:00+09:00
archived-at: 2026-08-03T12:30:00+09:00
verification-summary: |
  Re-verified 2026-08-03 against the live tree, not the task's claims.
  - `make fmt-check` → `gofmt -s: 240 files checked, 0 unformatted`, exit 0; `gofmt -s -l .` → 0 lines. (Count is 240 now vs 213 at closing: corpus growth, not drift.)
  - `git diff -w --ignore-blank-lines --numstat 621d55a^ 621d55a` → empty; range touches exactly the 9 named files; `--numstat` totals 222+/223-.
  - `internal/cli/root.go:356` `func loadEnv` is at column 0 and `PersistentPreRun` at :40 is correctly nested — the specific defect in the Problem section is gone.
  - `mise exec -- golangci-lint run ./...` → `0 issues.` (v2.12.2 pinned).
  - Gate is wired and non-vacuous: ci.yml:49 runs `make fmt-check`; Makefile:44 `lint: vet fmt-check`; Makefile:88-91 exits 1 if zero .go files found, Makefile:92-97 exits 1 naming offenders.
  - `make test` / `make build` green per lead baseline; not re-run.
---

# Task 078: Decide whether formatting is enforced, then make it true

## Problem

`gofmt -l .` names nine files, 586 diff lines in total, all whitespace:

| file | diff lines |
| --- | --- |
| `internal/cli/root_test.go` | 202 |
| `internal/cli/compose.go` | 109 |
| `internal/cli/stack.go` | 77 |
| `internal/cli/root.go` | 71 |
| `internal/cli/doctor.go` | 49 |
| `internal/cli/status.go` | 28 |
| `internal/cli/devcontainer.go` | 22 |
| `internal/runner/interaction_tree.go` | 16 |
| `tools/skillgen/main.go` | 12 |

`internal/cli/root.go` is the worst of them: the root command's `PersistentPreRun` block and the
whole of `func loadEnv` are indented one level too deep, so a top-level function reads as if it
were nested. Go compiles it either way, which is why nothing complained.

The repo already ships the fix — `Makefile:57` defines `fmt: gofmt -w -s .` — and `gofmt -s -l`
names the same nine files, so `make fmt` would produce exactly this diff and nothing more.

## Why it matters

Low severity: it is whitespace, and `make build`, `make test`, `go vet` and `golangci-lint run`
(v2.12.2, the pinned version) all pass over it. Formatting is genuinely not gated —
`.github/workflows/ci.yml` has no format step and the enabled linters do not include one.

It is worth a decision rather than a shrug because the state is self-perpetuating. Every editor
configured to format on save produces an unrelated whitespace hunk in these nine files, so the
next author either commits noise or turns formatting off for the repo.

## The decision

**A. Run `make fmt`, commit the 586 whitespace lines, and add a format check to CI.** Makes the
`make fmt` target mean something. Costs one blame-polluting commit across nine files, three of
which `6fca01e` just touched.

**B. Run `make fmt` and add no CI check.** Fixes today's drift and guarantees it returns.

**C. Leave it and drop `make fmt`.** Honest, but gives up on `root.go` reading correctly.

Recommendation: **A**, as its own commit touching nothing but whitespace, so the diff is
verifiable by `git show --stat` and `gofmt -l` alone. The CI half is what stops it recurring —
without it this task gets refiled.

## Non-goals

- Do not mix the formatting run with any behavioural change. A whitespace commit is only
  reviewable if it is provably whitespace.
- Do not hand-fix `root.go`'s indentation. `gofmt -w` is the tool for it.

## Acceptance criteria

- [x] Every file satisfies gofmt | verify: `make fmt-check` — prints `gofmt -s: 213 files checked, 0 unformatted`
- [x] The formatting commit changed no behaviour | verify: `git diff -w --ignore-blank-lines --numstat 621d55a^ 621d55a` — empty
- [x] Full suite still passes | verify: `make test`
- [x] Lint still clean at the pinned version | verify: `golangci-lint run ./... 2>&1 | tail -1` — `0 issues.`
- [x] If A: CI fails on unformatted code | verify: inject a badly-indented file, `make fmt-check` exits non-zero naming it, remove it, gate returns green

## Evidence

The drift predates the commit that surfaced it. Identical `gofmt -d` output at `600f1db` and at
`6fca01e` for all seven `internal/cli` files, so
[TASK-077](077-ci-lint-job-red-since-the-help-group-test-landed.md)'s lint failure and
this are unrelated faults that happened to be found in the same pass.

## Resolution

Option A, in two commits, in the order the task asked for.

### `621d55a` — the whitespace

`make fmt` (`gofmt -w -s .`) over the same nine files. The claim "whitespace only" is not an
eyeball judgement; it is one command:

| measurement | result |
| --- | --- |
| `git diff --name-only` | 9 files |
| `git diff -w --ignore-blank-lines --numstat` | **empty** |
| `gofmt -s -l . \| wc -l` | 9 → **0** |
| `make test` | pass (`-race`, all packages) |
| `golangci-lint run ./...` | `0 issues.` |

The second row is the whole proof. Ignoring whitespace *and* blank-line changes, there is no diff
left — so nothing non-whitespace is hiding inside the churn. This replaces the task's original
`human —` criterion, which asked a reviewer to eyeball a 445-line diff for indentation-only
changes; a reviewer cannot do that reliably and git can.

### The line count moved, and the reason is on the record

The task predicted **586** diff lines. The commit recorded **445** (222 + 223). Both numbers were
correct when taken:

- **586** counted raw `gofmt -d` *output* lines, which include per-file diff headers and context.
- **445** counts only `+`/`-` lines, as `git show --numstat` reports them.
- The corpus also changed underneath: **16 commits** touched these nine files between the task
  being filed on 2026-07-30 and the fix landing, including
  [TASK-108](108-two-did-you-mean-blocks-answer-one-error-differently.md), which deleted three test
  functions from `root_test.go` — the file that dominated the estimate at 202 lines and landed
  at 179.

Recorded rather than silently adopting either figure, because a stale estimate that quietly
becomes the new "measured" number is how a number stops meaning anything.

`root_test.go` is `+89/-90`, and the one-line asymmetry is not a lost statement: 6 blank-only lines
removed, 5 added. `gofmt -s` collapsed a blank line, which is why `--ignore-blank-lines` is part of
the proof above and not decoration.

### `Makefile` + `.github/workflows/ci.yml` — the gate

Without this half the task refiles itself, as the recommendation said. Three design points:

- **The gate prints what it checked.** `gofmt -s -l .` prints *nothing* when everything passes —
  and prints exactly the same nothing if it checked zero files. `make fmt-check` therefore reports
  `gofmt -s: 213 files checked, 0 unformatted` and **exits 1 if the file count is 0**, so the
  check cannot pass vacuously after a bad path edit. Same failure mode as
  [TASK-109](109-the-task-link-check-has-been-red-for-22-links-since-the-repo-moved.md).
- **One owner.** The gofmt invocation lives in the Makefile; CI runs `make fmt-check`. The YAML
  does not restate the command, so local and CI cannot disagree — `SOUL.md` 신념 3.
- **`lint` depends on it.** `lint: vet fmt-check`, so the local entry point covers formatting too.

Proven able to fail, which is the criterion that matters:

| state | `make fmt-check` |
| --- | --- |
| clean tree | `213 files checked, 0 unformatted`, exit 0 |
| one badly-indented file injected | `ERROR: not gofmt -s formatted` + the filename, **exit non-zero** |
| probe removed | back to exit 0 |

The failing exit is `2`, not `1` — that is `make`'s exit code for a failed recipe, not the
recipe's own. CI tests for non-zero, so the gate works; the number is written down here so nobody
later "fixes" a discrepancy that isn't one.

## Left open

`make check-generate` is labelled `(CI)` in the Makefile and **CI never invokes it** — the same
shape as this task one level up: a gate that exists and does not run. It currently passes
(exit 0, no diff), so this is latent rather than broken, and switching it on changes what CI
enforces for generated files. Filed as
[TASK-112](../done/112-check-generate-is-labelled-ci-and-ci-does-not-run-it.md) rather than folded
in here.
