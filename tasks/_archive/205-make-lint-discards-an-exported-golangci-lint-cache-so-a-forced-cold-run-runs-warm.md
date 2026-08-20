---
id: TASK-205
title: "`make lint` discards an exported `GOLANGCI_LINT_CACHE`, so a run forced cold silently runs warm"
type: bug
priority: P2
effort: XS
created-at: 2026-08-19T19:03:34+09:00
source: "found by a peer session while reproducing TASK-204 — it tried to force a cold cache with `GOLANGCI_LINT_CACHE=$(mktemp -d) make lint`, got rc=0 with no typecheck errors, and nearly concluded that a known-broken toolchain pairing does not affect the gate. It caught the discarded override itself before reporting anything; this card's author independently reproduced the defect and its consequence"
scope: "Makefile lint target, one assignment operator. No change to which linters run, no change to the default cache location, no change to TASK-203's per-checkout scoping, no Go source change."
status: done
completed-at: 2026-08-20T10:22:15+09:00
quality-review: pass
quality-reviewed-at: 2026-08-20T10:22:15+09:00
verified-at: 2026-08-20T10:22:15+09:00
archived-at: 2026-08-20T10:22:15+09:00
quality-review-evidence: |
  Fixed in 07d0c47, measured with the default cache directory moved aside first so its
  presence could not be inherited from an earlier run: exported, the supplied dir goes
  0 -> 10,448 KB and the default is never created; unset, the default is created at
  10,444 KB, so TASK-203's per-checkout scoping is preserved. Both arms print `0 issues.`,
  which is why the evidence is stated as a magnitude rather than as a verdict.
  An earlier pass of that measurement recorded the default location as created, which would
  have contradicted the first arm. It was residue from TASK-204's runs in the same worktree
  minutes earlier; it was re-measured under control rather than reported.
  Independent review established by measurement — not by reasoning from the shell fact —
  that `:-` rather than `-` is load-bearing here: an exported-but-empty value survives as
  empty under `-`, and `golangci-lint cache status` then reports the machine-wide dir,
  which would reintroduce TASK-203's bug through the very line written to prevent it.
  Open Question 2 swept with its axis and denominator stated rather than named: 4
  tab-indented recipe lines carrying a `NAME="..."` assignment, one of which was this
  defect; the other three are shell locals where discarding an inherited value is intended.
  No second instance.
  All four bindings on this card re-run and passing. Gates at 4cbfdcd: doc-check,
  check-generate, lint, test, commit-check, and `ce task validate --all` across 9 cards,
  all green.
---

## Summary

The `lint` recipe sets the cache with an unconditional assignment — the first line of the
target's shell block, cited by variable rather than by line number because TASK-204's
proposed check inserts lines into this same recipe:

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

It has already misled a measurement. A **peer session** verifying whether the TASK-204
toolchain mismatch actually fails `make lint` used exactly this command, got rc=0 with
zero typecheck lines, and was about to conclude the mismatch is harmless to the gate. It
caught the discarded override itself, before reporting anything, on the reflex that rc=0
was too convenient — so the near-miss is a near-miss and not a false report.

The conclusion would have been wrong. Forced cold by other means, the same tree gives
rc=2 and 4 typecheck errors, and a direct `golangci-lint` run on a genuinely cold cache
gives `0 issues.` while writing 10 MB of cache — measured for TASK-204. That
reproduction, and the catch above, are **different facts with different authors**: the
peer caught the discarded override, this card's author established what it would have
cost. Left unfixed, the wrong conclusion closes a real bug as unreproducible.

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

- [x] An exported `GOLANGCI_LINT_CACHE` is honoured by `make lint`.
      verify: `grep -c 'GOLANGCI_LINT_CACHE:-' Makefile` → at least 1 (today: **0**)
- [x] The default location is unchanged when nothing is exported, so TASK-203's
      per-checkout scoping still holds.
      verify: `grep -c 'tmp/golangci-lint-cache' Makefile` → 2, unchanged
- [x] No change to which linters run.
      verify: `grep -c 'golangci-lint run ./...' Makefile` → 2, unchanged
- [x] A run forced cold actually runs cold.
      verify: human — run the reproduction above after the fix; the supplied directory
      must be non-empty at step 3, and the run must re-analyse rather than replay

## Verification (2026-08-20)

Fixed as proposed. Both arms measured in one sitting in this worktree, with the default
cache directory moved aside first so its presence could not be inherited from an earlier
run:

| arm | `GOLANGCI_LINT_CACHE` | supplied dir before → after | default dir afterwards |
|---|---|---|---|
| 1 | exported to a fresh empty dir | 0 KB → **10,448 KB** | **not created** |
| 2 | unset | — | created, **10,444 KB** |

Both arms print `0 issues.`, which is exactly why the verdict is stated as a magnitude:
`0 issues.` and *did nothing* are the same output, and 10 MB written versus 0 KB is the
only thing that separates them.

Arm 2 is the TASK-203 regression check — with nothing exported the cache still lands in
`$(CURDIR)/tmp/golangci-lint-cache`, so a reclaimed worktree still takes its cache with it.

An earlier pass of this measurement recorded "default location created: **YES**", which
would have contradicted arm 1. That reading was residue: the directory had been created by
TASK-204's arm A/B runs in the same worktree minutes earlier, not by the override run. The
table above is the re-measurement with the default moved aside, and it is the one to quote.

**Open Question 2, now swept rather than named.** Axis: tab-indented recipe lines
containing a `NAME="..."` assignment. Denominator: **4** in the Makefile today. One was
this defect (the `GOLANGCI_LINT_CACHE` line, fixed). The other three — `GO_BIN_DIR` in
`install`, `gopls_cmd` twice in `lint` — are shell locals computed inside the recipe, not
names a caller conventionally exports; discarding an inherited value is their intent, not a
defect. No second instance of this bug.

Open Question 1 stands as its leaning: `clean` still deletes only the default path.

**`:-` rather than `-` is load-bearing, and this is measured rather than reasoned.** The
two differ only on an exported-but-empty value, which looks like a detail:

| `GOLANGCI_LINT_CACHE` | `${VAR:-D}` | `${VAR-D}` | `golangci-lint cache status` |
|---|---|---|---|
| unset | `D` | `D` | `Dir: ~/Library/Caches/golangci-lint` |
| exported empty | `D` | *empty* | `Dir: ~/Library/Caches/golangci-lint` |
| exported to a path | the path | the path | `Dir: /tmp/xyz-probe-dva` |

With `-`, an exported-empty value survives as empty, and golangci-lint then falls back to
its own machine-wide cache — TASK-203's bug back in full, through the very line written to
prevent it. The third column is the part that is easy to assert from the shell fact alone
and get wrong; it was run, not inferred.

## Open Questions

1. `make clean` deletes `$(CURDIR)/tmp/golangci-lint-cache` only. Once an override is
   honoured, should `clean` follow the override? Leaning no — `clean` cleaning a directory
   the caller chose elsewhere is more surprising than leaving it, and the caller who set it
   can remove it.
2. Are there other unconditional assignments in the Makefile with the same property? Not
   swept for. The axis would be recipe-level `VAR="..."` assignments of names that callers
   conventionally export; a sweep should state that axis and its denominator rather than
   report a bare "checked".
