---
id: TASK-197
title: "The migration layer has no end condition and the guide it points to never names it"
type: docs
priority: P2
effort: M
created-at: 2026-08-19T17:25:35+09:00
source: "measured 2026-08-19 — 1057 non-test lines behind one opt-in call site, 0 removal notices against an infra: control of 1, and 0 mentions of 'config migrate' in the 212-line docs/42 the warnings link to"
scope: "docs/42 gains the compatibility-layer section for the converter, and internal/cli/config_migrate.go gains the version-stamped notice the infra: fold already carries. No change to what the converter converts."
status: done
completed-at: 2026-08-26T11:13:55+09:00
completion-summary: "Documented the config migration compatibility layer, its corpus-based removal predicate, and its CLI deprecation horizon."
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "go test ./internal/cli && make doc-check && git diff --check"
    result: "passed"
  - kind: automated
    command-or-step: "preview config migration across repository examples"
    result: "16 configs; 0 converted; 1 with Left for you; 1 planless-order config with 3 items; 0 failures"
  - kind: automated
    command-or-step: "preview config migration across /Users/archmagece/mydevbox"
    result: "25 configs; 4 converted; 14 with Left for you; 13 planless-order configs with 27 items; 0 failures"
quality-review: pass
quality-reviewed-at: 2026-08-26T11:16:39+09:00
quality-review-evidence:
  - "go test ./internal/cli -count=1, make doc-check, and git diff --check passed"
  - "corpus sweep totals and planless-order exception counts were independently reviewed"
quality-review-receipt: tmp/task-management/direct/queue-run/task-197-review-receipt.json
archived-at: 2026-08-26T11:18:04+09:00
verified-at: 2026-08-26T11:18:04+09:00
verification-summary: "Migration compatibility documentation and CLI horizon verified by targeted tests, documentation gates, and repository plus devbox preview sweeps."
---

# Task 197: The migration layer has no end condition and the guide it points to never names it

## Summary

DVA carries a legacy-config converter with no stated condition for removing it.
`internal/config/migrate.go`, `migrate_applications.go`, `migrate_stack_order.go`
and `migrate_report.go` plus `internal/cli/config_migrate.go` are 1057 non-test
lines (838 more in tests) reachable from exactly one non-test call site —
`internal/cli/config_migrate.go:63`. Nothing calls it automatically; it runs only
when a user types `dva config migrate`.

The repository already has a house idiom for this, and the converter does not
follow it. The deprecated `infra:` section is folded automatically at
`internal/config/config.go:708-712` and the fold prints a warning that names the
card that deprecated it and states `'infra:' will be removed in a future
release`. Measured across the eight migrate files: 0 occurrences of `future
release` or `will be removed`; the same matcher against `config.go` returns 1, so
it is alive. TASK-051 additionally carries a `## Migration / deprecation plan`
section listing the removal step. The converter has no such section anywhere.

This gap is inherited, not new. TASK-007 (`tasks/_archive/007-migration-compat.md:26`)
listed `legacy command/section deprecation 정책` as a Deliverable and was archived
without producing one.

Two consequences are measurable today.

**The guide the warnings link to never names the tool.** `dva validate` on a
config with `stack.*.order` prints `Run: dva config migrate` and, on the next
line, `Migration guide: .../docs/42-migration-and-compatibility.md#11-migration`.
That document is 212 lines and contains the string `config migrate` 0 times. Its
`## 12. Compatibility Layers` section documents subprojects (12-1), interactions
(12-2) and provision (12-3), and summarises them in 12-4. The converter — the
one compatibility layer with actual conversion code behind it — has no
subsection. No removal version or date appears in the file either (0 matches).

**The escape hatch loops on the repository's own example.** `examples/stack-source.yml`
is a shipped example. `dva validate` on it warns twice and says to run
`dva config migrate`. `dva config migrate` on it answers `nothing to convert`,
then `Left for you: stack.*.order: this config declares no plans, so there is
nowhere to move the ordering to — declare a plan whose entries[] name these
declarations, then re-run`. So validate sends the user to migrate, and migrate
sends them back to edit by hand. The advice is individually correct at each step
and circular as a path.

The decision this card asks for is not "delete the converter". It is: state the
predicate under which the converter stops being carried, and put it where the
warnings already point.

## Completion Criteria

- [x] `docs/42-migration-and-compatibility.md` gains a `### 12-5.` subsection for the converter, covering what it converts, what it reports under `Left for you`, and the removal predicate | verify: `grep -c 'config migrate' docs/42-migration-and-compatibility.md` returns ≥ 3 (today: 0)
- [x] The removal condition is written as a testable predicate over the config corpus, not as a date or a version alone | verify: human — read 12-5 and confirm a reader could run the predicate and get a yes/no without asking the author
- [x] `dva config migrate` states its own deprecation horizon the way the `infra:` fold does | verify: `grep -cE 'future release|will be removed' internal/cli/config_migrate.go` returns ≥ 1 (today: 0; control `internal/config/config.go` returns 1)
- [x] TASK-007's unfulfilled deliverable is either satisfied by 12-5 or explicitly declared out of scope in it | verify: human — 12-5 names TASK-007 and says which
- [x] The post-converter corpus sweep exists, run against the real config corpus rather than only the repository | verify: `human — for f in $(find ~/mydevbox -maxdepth 2 -name dva.yml); do (cd $(dirname $f) && dva config migrate); done` — preview only, never `--write`; record total / converts / has-"Left for you", as TASK-069 did
- [x] The in-repository corpus is swept and recorded in the same units | verify: `for f in examples/*.yml; do mkdir -p tmp/sw && cp "$f" tmp/sw/dva.yml && (cd tmp/sw && ../../bin/dva config migrate); done` — today: total=16 converts=0 has-"Left for you"=1
- [x] The `examples/stack-source.yml` loop is closed — either the example declares a plan, or 12-5 states that a plans-less config is expected to be edited by hand and says so in the `Left for you` text | verify: `cd tmp/sw && ../../bin/dva validate` on a copy of `examples/stack-source.yml` reports 0 warnings, OR 12-5 contains the accepted-limitation statement
- [x] `make doc-check` passes | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && make doc-check`

## References

- `internal/cli/config_migrate.go:63` — the only non-test call site of `config.Migrate`
- `internal/config/config.go:708-712` — the `infra:` fold and its deprecation warning; the idiom to copy
- `tasks/_archive/051-infra-compose-command-semantics.md:130` — `## Migration / deprecation plan`, the section shape
- `tasks/_archive/007-migration-compat.md:26` — the deliverable archived unfulfilled
- `tasks/_archive/069-migrate-applies-to-no-live-config-while-18-warn.md` — the last corpus sweep, and why it cannot answer this card
- `docs/42-migration-and-compatibility.md` — §12 Compatibility Layers, where 12-5 belongs
- `docs/43-command-surface-restructure.md:56` — the hard-break ruling that made `dva config migrate` the sole route off `applications:`
- `internal/config/validate.go:45` — the `applications` rejection that points at the converter

## Open Questions

- Should the removal predicate be corpus-based (`0 configs convert across two
  consecutive releases`) or surface-based (`the rejected keys have been rejected
  for N minor versions`)? The corpus form is honest but depends on a corpus only
  the maintainer can sweep; the surface form is checkable in CI. A hybrid — CI
  checks the surface, the release checklist requires the corpus sweep — is
  probably right but is a maintainer's call, not this card's.
- `migrate_stack_order.go` converts nothing when a config declares no plans, by
  design, because deleting an `order:` without a plan entry to move it to would
  silently drop the ordering. That makes `stack.*.order` permanently
  unconvertible for plans-less configs. Does the end condition require those
  configs to migrate first, or does it accept that they never will?

## Technical Notes

Line counts, measured 2026-08-19 with `wc -l`:

```
non-test  migrate.go 285  migrate_applications.go 333  migrate_stack_order.go 151
          migrate_report.go 124  config_migrate.go 164            = 1057
tests     migrate_test.go 189  migrate_applications_test.go 455
          migrate_stack_order_test.go 194                          =  838
```

The converter matcher was proved alive before the zero counts above were
trusted. A synthetic config containing only

```yaml
applications:
  api:
    path: ./api
    run: "go run ."
```

produces `Converted: applications.api → stack.api` and the equivalent `stack:`
block on stdout, so a sweep reporting `converts=0` is a real zero rather than a
dead matcher.

TASK-069's sweep cannot be reused as this card's baseline. It is stamped
`verified-at: 2026-08-03` and reports `would-migrate=0 nothing-to-do=31`. But
`internal/config/migrate_applications.go` was created 2026-08-06 in `6710766`,
the same commit that removed `applications:`. The sweep therefore ran three days
before the applications conversion path existed and says nothing about it. No
sweep has been run since.

One counting trap, recorded because it cost a re-run here: `dva config migrate`
prints `nothing to convert` and a `Left for you:` block in the same run, so a
sweep that buckets on `nothing to convert` alone reports a clean corpus while
unconverted deprecations sit in the output. The in-repository numbers above
count both, which is why `converts=0` and `has-"Left for you"=1` are reported
separately rather than as one "clean" figure.
