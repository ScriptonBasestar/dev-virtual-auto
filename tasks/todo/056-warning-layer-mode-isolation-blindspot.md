---
id: TASK-056
title: "Warning layer is blind to mode isolation — stack-order warning repeats the fixed compose-split defect"
type: fix
priority: P2
status: todo
effort: XS
created-at: 2026-07-30T00:00:00+09:00
scope: "dva repo — internal/config/validate_warnings.go"
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

- [ ] Order warning is silent when the order-group is mode-isolated | verify: `cd /Users/archmagece/mydevbox/primeno1-devbox && /Users/archmagece/mywork/scripton/dev-virtual-auto/bin/dva validate 2>&1 | grep -qv 'have order 0'`
- [ ] Order warning still fires when entries can co-occur | verify: `go test ./internal/config/ -run TestWarnDuplicateStackOrder`
- [ ] Compose-split suppression from b20fee8 still holds | verify: `go test ./internal/config/ -run TestWarnMultiStackComposeSplit`
- [ ] Full suite green | verify: `make test`
- [ ] No regression across the real corpus: 83 configs in ~/mydevbox, only the 9 deliberate negative fixtures fail | verify: `human — re-run the documented validate sweep and compare counts`

## Findings

Other warning functions that count `stack:` entries flatly and may share the blind
spot (unverified, do not fix here): `warnLegacyStackOrder`,
`warnDuplicateComposeApplicationOwnership`.

## Evidence

- Reproduced 2026-07-30 against `bin/dva` @ `b20fee8` in `primeno1-devbox`
  (4 compose entries, 4 modes each claiming one, `default_mode: full`).
- Context and the compose-split fix it follows: `tmp/71-mydevbox-migration-result.md`.
