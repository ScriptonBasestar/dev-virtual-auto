---
id: TASK-205
title: "`make lint` discards an exported `GOLANGCI_LINT_CACHE`, so a run forced cold silently runs warm"
type: bug
priority: P2
effort: XS
created-at: 2026-08-19T19:03:34+09:00
source: "found while reproducing TASK-204 — a session tried to force a cold cache with `GOLANGCI_LINT_CACHE=$(mktemp -d) make lint`, got rc=0 with no typecheck errors, and nearly reported that a known-broken toolchain pairing does not affect the gate"
scope: "Makefile lint target, one assignment operator. No change to which linters run, no change to the default cache location, no change to TASK-203's per-checkout scoping, no Go source change."
status: todo
---

## Summary

`Makefile:54` sets the lint cache with an unconditional assignment:

```make
@GOLANGCI_LINT_CACHE="$(CURDIR)/tmp/golangci-lint-cache"; export GOLANGCI_LINT_CACHE; \
```

A caller who exports `GOLANGCI_LINT_CACHE` to force a cold run has it discarded silently.
The target uses the checkout's existing warm cache instead, and reports a pass.

**Measured** — a fresh worktree, `GOLANGCI_LINT_CACHE=$(mktemp -d) make lint`:

| observation | result |
|---|---|
| the supplied directory afterwards | **0 KB — never written to** |
| `$(CURDIR)/tmp/golangci-lint-cache` afterwards | created and populated |

## Why it matters

The failure mode is a **vacuous pass**. A run whose entire purpose was to re-analyse from
scratch reports success without having re-analysed anything, and nothing in the output
says so. This is the same shape the repo already gates against in acceptance criteria — a
check that cannot fail certifies itself — except here it is the build doing it.

It has already misled a measurement. A session verifying whether the TASK-204 toolchain
mismatch actually fails `make lint` used exactly this command, got rc=0 with zero
typecheck lines, and was about to conclude the mismatch is harmless to the gate. It is
not: forced cold by other means, the same tree gives rc=2 and 4 typecheck errors. The
wrong conclusion would have closed a real bug as unreproducible.

TASK-203, which introduced the line, is not wrong — scoping the cache per checkout is what
stops a reclaimed worktree leaving phantom findings behind. The defect is only that the
scoping was written in a form that also forbids the caller from redirecting it.

## Reproduction

1. In a checkout with a warm cache (any tree where `make lint` has run once), take note of
   `du -sk tmp/golangci-lint-cache`.
2. `PROBE=$(mktemp -d); GOLANGCI_LINT_CACHE="$PROBE" make lint`
3. `du -sk "$PROBE"` → **0**. The override went nowhere, and step 2 ran against the warm
   cache from step 1.

## Proposed fix

Use a default-if-unset expansion, so the scoping still applies by default and a caller can
still redirect it:

```make
@GOLANGCI_LINT_CACHE="$${GOLANGCI_LINT_CACHE:-$(CURDIR)/tmp/golangci-lint-cache}"; \
export GOLANGCI_LINT_CACHE; \
```

TASK-203's guarantee survives unchanged: with nothing exported the path is exactly what it
is today, so a reclaimed worktree still takes its cache with it.

## Completion Criteria

- [ ] An exported `GOLANGCI_LINT_CACHE` is honoured by `make lint`.
      verify: `grep -c 'GOLANGCI_LINT_CACHE:-' Makefile` → at least 1 (today: **0**)
- [ ] The default location is unchanged when nothing is exported, so TASK-203's
      per-checkout scoping still holds.
      verify: `grep -c 'tmp/golangci-lint-cache' Makefile` → 2, unchanged
- [ ] No change to which linters run.
      verify: `grep -c 'golangci-lint run ./...' Makefile` → 2, unchanged
- [ ] A run forced cold actually runs cold.
      verify: human — run the reproduction above after the fix; the supplied directory
      must be non-empty at step 3, and the run must re-analyse rather than replay

## Open Questions

1. `make clean` deletes `$(CURDIR)/tmp/golangci-lint-cache` only. Once an override is
   honoured, should `clean` follow the override? Leaning no — `clean` cleaning a directory
   the caller chose elsewhere is more surprising than leaving it, and the caller who set it
   can remove it.
2. Are there other unconditional assignments in the Makefile with the same property? Not
   swept for. The axis would be recipe-level `VAR="..."` assignments of names that callers
   conventionally export; a sweep should state that axis and its denominator rather than
   report a bare "checked".
