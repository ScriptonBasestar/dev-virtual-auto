---
id: TASK-078
title: "Nine files do not satisfy gofmt, one of them indents a top-level function, and no CI job checks"
type: fix
priority: P4
status: todo
effort: XS
created-at: 2026-07-30T00:00:00+09:00
scope: "repo-wide gofmt run (9 files) + the decision on whether CI should gate formatting"
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

- [ ] Every file satisfies gofmt | verify: `test 0 -eq "$(gofmt -l . | wc -l)"` — print the count
- [ ] The formatting commit changed no behaviour | verify: `human — git show <sha> | grep -v '^[-+][[:space:]]*$' shows only indentation changes`
- [ ] Full suite still passes | verify: `make test`
- [ ] Lint still clean at the pinned version | verify: `golangci-lint run ./... 2>&1 | tail -1`
- [ ] If A: CI fails on unformatted code | verify: `human — reintroduce one bad indent on a branch, confirm the new job goes red`

## Evidence

The drift predates the commit that surfaced it. Identical `gofmt -d` output at `600f1db` and at
`6fca01e` for all seven `internal/cli` files, so
[TASK-077](../done/077-ci-lint-job-red-since-the-help-group-test-landed.md)'s lint failure and
this are unrelated faults that happened to be found in the same pass.
