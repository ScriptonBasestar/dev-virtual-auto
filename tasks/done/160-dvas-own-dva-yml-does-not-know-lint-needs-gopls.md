---
id: TASK-160
title: "DVA's own dva.yml never learned that make lint requires gopls"
type: bug
priority: P3
status: done
effort: S
created-at: 2026-08-03T15:15:00+09:00
source: "TASK-130 finalize verification — the dogfood config, not updated with the tool the fix added"
depends-on: [TASK-130]
scope: "dva repo — dva.yml:12-15, :55"
---

# Task 160: Declare gopls as a prerequisite in the repo's own config

## Problem

TASK-130 made `gopls` a second **mandatory** contributor tool. `Makefile:60-66` hard-fails
without it:

```make
else \
    echo "Install gopls (https://pkg.go.dev/golang.org/x/tools/gopls) or run 'mise install'"; exit 1; \
fi; \
```

`dva.yml` — the repo's own dogfood config, and the one place that enumerates what a contributor
needs — still declares two prerequisites (`dva.yml:8-15`):

```yaml
  - name: "Go toolchain available"
  - name: "golangci-lint available"
```

and describes the interaction as `Run linters (golangci-lint)` at `dva.yml:55`.

So a contributor who has golangci-lint and not gopls gets a clean `dva doctor` — every declared
check passes, exit 0 — and then a hard failure from `make lint`. Doctor's job is to answer "do I
have what this repo needs before I start", and here it answers wrongly about a tool the repo
requires.

## Why it matters more than one missing line

This is DVA checking DVA. The prerequisite list going stale when a tool is added is the exact
failure mode `doctor` exists to prevent, occurring in the config that demonstrates the feature.
Anyone reading `dva.yml` as the worked example learns a pattern that has already drifted.

## Acceptance criteria

- [x] `dva.yml` declares a `gopls available` prerequisite. Its command mirrors Makefile:60-66
      (`mise which gopls` first, then PATH), so a contributor with gopls only under `mise` passes;
      fix_hint names both routes ("Install gopls … or run 'mise install'").
- [x] The lint description now reads "Run linters (golangci-lint, gopls)".
- [x] Proven to fail when absent: with `gopls` and `mise` masked off PATH (go + golangci-lint
      still present), `dva doctor` prints `[FAIL] gopls available -> Install gopls … or run
      'mise install'` and exits non-zero, while Go toolchain and golangci-lint stay `[pass]`.
      Normal run: 6 passed, 0 failed (including `[pass] gopls available`).
- [x] Tool diff: `make build`/`test` invoke `go` (declared); `make lint` invokes `go vet` +
      `gofmt` (bundled with the Go toolchain) + `golangci-lint` (declared) + `gopls` (now
      declared); `make generate` runs `skillgen`/`libgen` (Go programs, covered by `go`). Required
      external tools: **3 (go, golangci-lint, gopls); all declared after this change; 0
      required-but-undeclared.** `mise` itself is optional.
- [x] `make test` exits 0; `dva validate` reports `dva.yml is valid`.

## Notes

Distinct from
[TASK-154](154-the-ci-suffix-marks-one-of-the-five-targets-ci-actually-runs.md), whose scope is
Makefile help labels and `ci.yml` duplication. That task treats the `.mise.toml` pins as a
version-drift risk; this one is about a required tool being absent from the prerequisite list at
all. Whoever fixes 154 should read this, since both touch how the Makefile and the repo's own
config describe the same toolchain.
