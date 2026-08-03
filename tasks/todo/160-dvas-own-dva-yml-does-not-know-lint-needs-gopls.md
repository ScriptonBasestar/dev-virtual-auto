---
id: TASK-160
title: "DVA's own dva.yml never learned that make lint requires gopls"
type: bug
priority: P3
status: todo
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

- [ ] `dva.yml` declares a `gopls available` prerequisite alongside the other two, with a
      `fix_hint` naming both routes the Makefile accepts (`mise install`, or a direct install) —
      the check must not fail for someone who only has it under `mise`, which is how
      `Makefile:61-62` resolves it first.
- [ ] `dva.yml:55`'s description names both linters.
- [ ] Prove it fails when the tool is absent: run `dva doctor` with `gopls` masked off `PATH`
      (and out of `mise`'s view) and paste the `[FAIL]` row and exit code. A check verified only
      in its passing state is what
      [TASK-112](../_archive/112-check-generate-is-labelled-ci-and-ci-does-not-run-it.md) warns
      about.
- [ ] Diff `dva.yml`'s prerequisites against what `make lint`, `make test` and `make build`
      actually invoke, and report the count of tools required but undeclared. A bare "none left"
      is not a result with the denominator unstated.
- [ ] `make test` exits 0.

## Notes

Distinct from
[TASK-154](154-the-ci-suffix-marks-one-of-the-five-targets-ci-actually-runs.md), whose scope is
Makefile help labels and `ci.yml` duplication. That task treats the `.mise.toml` pins as a
version-drift risk; this one is about a required tool being absent from the prerequisite list at
all. Whoever fixes 154 should read this, since both touch how the Makefile and the repo's own
config describe the same toolchain.
