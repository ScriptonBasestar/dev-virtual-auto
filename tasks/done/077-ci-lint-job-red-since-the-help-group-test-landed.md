---
id: TASK-077
title: "The CI lint job has been red since the help-group test landed, one commit after the last red-CI task"
type: fix
priority: P2
status: done
effort: XS
created-at: 2026-07-30T00:00:00+09:00
scope: "internal/cli — root_help_groups_test.go (one negated conjunction)"
---

# Task 077: Clear the lint gate

## Problem

`golangci-lint run` fails at `6fca01e`:

```
internal/cli/root_help_groups_test.go:36:5: QF1001: could apply De Morgan's law (staticcheck)
	if !(recommended < direct && direct < other) {
```

`.github/workflows/ci.yml:34-48` runs golangci-lint v2.12.2 as its own job with no
`continue-on-error`, so CI on `master` is red.

## Why it matters

Same shape as [TASK-075](075-integration-suite-and-ci-red-since-legacy-compose-rejection.md),
one commit later: a gating CI step went red because the check that would have caught it is not
in the target anyone runs locally. `make test` passes, `make build` passes, `go vet` passes —
only the separately-invoked linter disagrees.

P2 for the same reason 075 was: a red gate suppresses signal for every change after it. It was
found while verifying TASK-074, which is exactly the cost — the next change has to prove the
redness is not its own.

## Fix shape

Apply the law the linter names. `!(a < b && b < c)` becomes `a >= b || b >= c`. The assertion,
its failure message, and the block ordering it pins are all unchanged.

## Non-goals

- Do not touch the help-group behaviour or the rest of the test.
- Do not add a lint step to `make test`. That is worth deciding, but it is the same open
  question TASK-075 left about the integration suite and belongs with it.

## Acceptance criteria

- [x] The linter is clean repo-wide at the pinned version | verify: `golangci-lint run ./... 2>&1 | tail -1` — prints `0 issues.`
- [x] The test still pins the block order | verify: `go test ./internal/cli/ -run TestLifecycleHelpSeparatesRecommendedFromDirectAccess`
- [x] Full suite passes under -race | verify: `make test`

## Evidence

The negation came in with the test itself, not from a later edit:

```
$ git log --oneline -1 -- internal/cli/root_help_groups_test.go
6fca01e refactor(cli): reorganize lifecycle help groups
```

`golangci-lint has version 2.12.2` locally, matching the version pinned in `ci.yml:48`, so the
local run reproduces the CI job rather than approximating it.

## Left open

Seven files under `internal/cli/` do not satisfy `gofmt`, including `root.go`, where a
top-level `func loadEnv` and the root command's `PersistentPreRun` block are indented one level
too deep. It predates `6fca01e` (identical `gofmt -d` output at `600f1db` and `6fca01e`) and no
CI job checks formatting, so it is not the cause of this failure. Filed separately as
[TASK-078](../todo/078-seven-cli-files-do-not-satisfy-gofmt.md).
