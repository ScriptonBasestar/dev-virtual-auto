---
id: TASK-056
title: "Warning layer is blind to mode isolation — stack-order warning repeats the fixed compose-split defect"
type: fix
priority: P2
status: done
effort: XS
created-at: 2026-07-30T00:00:00+09:00
scope: "dva repo — internal/config/validate_warnings.go"
verified-at: 2026-08-03T11:55:33+09:00
archived-at: 2026-08-03T11:55:33+09:00
verification-summary: |
  All 6 criteria MET against real behaviour.
  modesIsolateEntries (internal/config/validate_warnings.go:550) is the generalized,
  entry-set-agnostic isolation check; both warnDuplicateStackOrder (:440) and
  warnMultiStackComposeSplit (:514) call it, and no reference to the old
  modesIsolateComposeEntries name survives.
  Live check in ~/mydevbox/primeno1-devbox: `dva validate` rc=0 with the "have order 0"
  warning gone (0 occurrences). TestWarnDuplicateStackOrderModeIsolation passes,
  including the negative cases — a mode holding two entries, a mode with no stack:
  filter, and mixed isolated+racing groups where only the racing pair warns — so the
  suppression is not a blanket mute. The earlier compose-split suppression (b20fee8)
  still passes its 4 subtests.
  Corpus re-measured rather than trusted: the corpus has GROWN from the 83 recorded in
  the task to 92 dva.yml files. Sweeping validate over all of them gives 79 pass /
  13 fail, and all 13 failures are explicitly-named negative fixtures (dva-invalid,
  malformed*, dogfood validation-fixture-legacy) — the same pattern the task describes,
  scaled with the corpus, not a regression.
  An independent Python reimplementation of the isolation rule over raw YAML agrees
  with the Go logic on every group: 23 groups with stack>=2, 4 suppressed, 0 must-warn.
---

# Task 056: Teach the stack-order warning about mode isolation

## Problem

`b20fee8` fixed `warnMultiStackComposeSplit`, which told every config with two
compose entries to consolidate — advice that is impossible when the entries load
different base compose files, and that specifically told users to undo the
migration DVA itself required when `modes.<name>.compose` was removed.

Fixing it exposed the same blind spot one function up. `primeno1-devbox` now emits:

```
[warn] stack: entries compose, compose-minimal, compose-observability,
compose-tracing have order 0 (default); set explicit order values to
control startup sequence
```

Those four entries are each claimed by exactly one mode
(`minimal`/`full`/`observability`/`tracing`), so **no two of them are ever live in
the same invocation** and execution order between them is not a thing that exists.
`warnDuplicateStackOrder` (`internal/config/validate_warnings.go:256`) counts
entries sharing an `order` value without asking whether they can co-occur.

This is a class, not an incident: the warning layer reasons about `stack:` as a
flat set and does not know `modes.<name>.stack` partitions it.

## Root cause

`warnDuplicateStackOrder` groups entries by `order` and warns on any group of 2+.
Mode selection happens later, in `Orchestrator.filterEntries`
(`internal/lifecycle/orchestrator.go:360-364`), which the warning never consults.

## Fix shape

`b20fee8` added `modesIsolateComposeEntries(map[string]bool) bool` for exactly this
question. Generalise it to any entry set (it is already written against a name set,
so this is a rename plus dropping the compose-specific caller assumption), then have
`warnDuplicateStackOrder` skip any order-group whose members are mode-isolated.

Keep the existing suppression semantics established in `b20fee8`:

- a mode with no `stack:` filter selects **every** entry, so it disqualifies the
  arrangement (otherwise a common `full: {}` mode produces a false negative);
- do not re-warn about the unfiltered `dva up` path — `warnMissingDefaultMode`
  already owns it.

## Non-goals

- Do not audit all 22 warning functions in this task. Note candidates in the
  Findings section; a separate task can sweep them.
- No change to hard errors in `Validate()`.

## Acceptance criteria

- [x] Order warning is silent when the order-group is mode-isolated | verify: human — validate the named personal config and confirm the order-0 warning remains absent; the archived result records 0 occurrences
- [x] Order warning still fires when entries can co-occur | verify: `go test ./internal/config/ -run TestWarnDuplicateStackOrder`
- [x] Compose-split suppression from b20fee8 still holds | verify: `go test ./internal/config/ -run TestWarnMultiStackComposeSplit`
- [x] Full suite green | verify: `make test`
- [x] No regression across the real corpus: 83 configs in ~/mydevbox, only the 9 deliberate negative fixtures fail | verify: `human — re-run the documented validate sweep and compare counts`
- [x] Suppression fires only on genuinely isolated groups | verify: human — re-run the recorded personal-corpus audit; the archived result measured 83 configs and 9 deliberate negative fixtures

## Result

`modesIsolateComposeEntries` → `modesIsolateEntries`, now taking any entry set, and
`warnDuplicateStackOrder` skips order-groups it reports as isolated. primeno1 is clean
at rc=0 with the order warning gone; corpus sweep unchanged at 83 configs / 9 failures,
all 9 the known negative fixtures.

Empty warning output across a corpus is not by itself proof of a correct fix — it is
equally the signature of over-suppression. `tmp/scripts/audit-stack-order-groups.py`
settles which it is by reimplementing the isolation rule against the parsed YAML and
classifying every order-group: **19 configs have 2+ stack entries, 4 order-groups exist
in total, all 4 are genuinely mode-isolated, 0 groups should have warned.** So the
silence is the corpus having no racing groups, not the warning being unreachable. The
must-warn directions are held by unit tests instead (a mode holding two members, a mode
with no `stack:` filter, and a mixed config where one group is isolated and another
races — the last one proves suppression is per-group rather than global).

## Findings

Other warning functions that count `stack:` entries flatly and may share the blind
spot (unverified, do not fix here): `warnLegacyStackOrder`,
`warnDuplicateComposeApplicationOwnership`.

## Evidence

- Reproduced 2026-07-30 against `bin/dva` @ `b20fee8` in `primeno1-devbox`
  (4 compose entries, 4 modes each claiming one, `default_mode: full`).
- Context and the compose-split fix it follows: `tmp/71-mydevbox-migration-result.md`.
