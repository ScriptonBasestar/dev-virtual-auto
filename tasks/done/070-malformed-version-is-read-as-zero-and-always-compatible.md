---
id: TASK-070
title: "A malformed `version:` is read as 0.0.0 and passes the compatibility gate everywhere — the gate misses exactly what it exists to catch"
type: fix
priority: P3
status: done
effort: XS
created-at: 2026-07-30T00:00:00+09:00
scope: "dva repo — internal/config/config.go parseVersion/checkConfigVersion; optionally internal/config/schema.json version pattern"
---

# Task 070: Reject a `version:` that is not a version, instead of reading it as 0.0.0

## Problem

`parseVersion` (`internal/config/config.go:1162-1167`) discards both of `fmt.Sscanf`'s return
values — the field count and the error:

```go
func parseVersion(v string) [3]int {
	var parts [3]int
	v = strings.TrimPrefix(v, "v")
	_, _ = fmt.Sscanf(v, "%d.%d.%d", &parts[0], &parts[1], &parts[2])
	return parts
}
```

A string that does not parse leaves `parts` at its zero value, so `isVersionCompatible` asks
whether the running version is at least `0.0.0` and always answers yes. The gate is skipped
silently, with no error and no warning.

Measured against `bin/dva` at `Version = 0.1.44`:

| `version:` | `dva validate` | `dva show` |
| --- | --- | --- |
| `"9.9.9"` | rc=1 — "config requires minimum version 9.9.9" | rc=1, same |
| `"not-a-version"` | **rc=0** | **rc=0** |
| `"O.2.0"` (letter O, not zero) | **rc=0** | **rc=0** |

The third row is the damaging one. `checkConfigVersion` exists to stop a config that requires a
newer DVA from loading on an older one — and it does that correctly for `9.9.9`. But a
**misspelled** high version defeats it entirely: `O.2.0` was meant to require 0.2.0 and instead
requires nothing. The failure mode is not "a weird value is tolerated", it is "the one check
this field has is bypassed by a typo".

`dva validate` does not catch it either. The schema constrains `version` to `type: string` with
no `pattern`, so any string satisfies it — which contradicts the schema's own description of
the field: *"Must not exceed the installed DVA version, or the config is rejected."*

## Not in scope: the unquoted-number inconsistency

Recorded so it is not re-found and not accidentally "fixed" alongside this. `version: 0.1`
without quotes is a YAML **number**, and the two layers disagree: `dva validate` rejects it
(`Invalid type. Expected: string, given: number`, because YAML→JSON preserves the numeric type)
while `config.Load` accepts it (yaml.v3 coerces the numeric scalar into the Go `string` field,
so `checkConfigVersion` sees `"0.1"`). Leave it alone — making them agree means changing YAML
scalar decoding, a far larger blast radius, and the schema already rejects the shape at the one
command where the user is asking to be told about problems.

Note the interaction: once `parseVersion` is strict, `"0.1"` becomes a **parse failure** rather
than a silently-accepted `0.1.0`. So the pattern must allow a two-segment version, or this task
changes behavior for unquoted values it did not intend to touch.

## Fix shape — XS

Have `parseVersion` report failure instead of returning `[0,0,0]`, and let `checkConfigVersion`
surface it as the malformed-value error it is — naming the offending string and the expected
shape, since the whole point is that a typo is currently invisible.

Optionally also add `"pattern": "^v?\\d+\\.\\d+(\\.\\d+)?$"` to the schema's `version` property
so `dva validate` reports it too — secondary, because every command except `validate` bypasses
the schema entirely (`Config.Validate()` has one call site, `internal/cli/validate.go:23-25`).

**This is a tightening, unlike [TASK-067](067-version-field-rule-stated-three-incompatible-ways.md)'s
rule A** — it can turn a config that loads today into one that does not, so the corpus check is
a precondition, not a formality.

## Corpus risk

Checked before proposing this. Under `~/mydevbox`, 82 of 83 `dva.yml` files declare a
top-level `version:`, and every value that is a version parses (corrected figures in
Resolution — the first count here omitted `0.1.22` ×1). The only non-parsing values — and the
single omission — live under directories named `malformed`, `malformed-copy`, `malformed-dir`
and `malformed-fixture`, which are deliberate negative-test fixtures (one contains
`schema_version: "1.2"` and `plans: "this-must-be-an-object"`). So no real config regresses.
Re-measured at implementation time rather than trusted; the corpus is the user's working tree.

## Non-goals

- Do not change the minimum-version *semantics*. `internal/config/version.go` is right that
  `version:` is a floor rather than the producing binary's version; this task only makes a
  malformed floor an error instead of a silent zero.
- Do not make an empty/absent `version:` an error. That is deliberate
  (`checkConfigVersion` returns nil early) and TASK-067 fixes the schema to agree with it.
  Absent means "no gate"; malformed means "the gate was miswritten".
- Do not change YAML scalar decoding to resolve the unquoted-number case.
- Do not fix `~/mydevbox` fixtures that fail after this lands — they are negative tests and
  are supposed to be malformed.

## Acceptance criteria

- [x] A malformed `version:` fails to load instead of reading as 0.0.0 | verify: `go test ./internal/config/ -run TestParseVersionRejectsMalformed`
- [x] The error names the offending value | verify: `go test ./internal/config/ -run TestParseVersionRejectsMalformed`
- [x] `"9.9.9"` still reports the incompatibility, not a parse error | verify: `go test ./internal/config/ -run TestCheckConfigVersion`
- [x] An absent or empty `version:` still loads | verify: `go test ./internal/config/ -run TestCheckConfigVersion`
- [x] A two-segment version still parses | verify: `go test ./internal/config/ -run TestParseVersionRejectsMalformed`
- [x] Full suite green | verify: `make test`
- [x] No real corpus config regresses | verify: `human — every distinct version: value in ~/mydevbox accepted by the new pattern; see Resolution`

## Resolution

`parseVersion` now returns `([3]int, error)` and matches against a package-level
`versionPattern` — `^v?(\d+)\.(\d+)(?:\.(\d+))?$` — instead of `fmt.Sscanf`. **Sscanf could not
have been made strict by checking its error**: it does not fail on trailing input, so
`Sscanf("0.1.44-rc1", "%d.%d.%d", …)` returns `n=3, err=nil`. That is why this is a regexp.

`isVersionCompatible` returns an error too, and names which side is at fault: a bad
`cfg.Version` is a config defect, a bad `config.Version` a build defect (it is a `var` set by
ldflags).

`malformedVersionError` names the offending value and cites `MinScaffoldVersion` as the
example — so, like TASK-067, this adds **no** hand-written DVA version string.

`schema.json`'s `version` property got the same rule as a `pattern`. The two copies are
deliberate: neither can be derived from the other without making `Load` parse the schema.
`TestVersionPatternMatchesSchema` reads the pattern back out of the embedded schema and asserts
both engines judge the same inputs identically — the divergence guard, in place of coupling.

The comparison is exact, which was not a given: `gojsonschema` v1.2.0
compiles `pattern` with Go's own `regexp` (`schema.go:655`), so both sides are RE2, and the
usual ECMA-262-vs-RE2 anchor and `$`-before-newline differences do not arise here.

**One asymmetry is intentional and is asserted so nobody reports it as a divergence bug.** A
segment can be all digits and still not fit an `int`: `99999999999999999999.0.0` satisfies the
schema, which constrains shape only, but `parseVersion` also has to represent the value, so
`strconv.Atoi`'s range error rejects it. This is the sole path that reaches the `Atoi` error
branch — the pattern guarantees digits otherwise — which is what makes the comment there true.

### Two existing tests were pinning the defect

`TestParseVersion` asserted `{"invalid", [3]int{0,0,0}}` and `{"", [3]int{0,0,0}}` — the exact
behavior this task removes. Re-signaturing it was not enough; those rows had to go, and they
moved to `TestParseVersionRejectsMalformed` as rejections. Note that the old table *passing* was
itself the evidence that the gate was defeated.

**Corpus figures corrected.** Re-measured 2026-07-30: `0.1.44` ×53, `0.1.26` ×21, `0.1.0` ×4,
`0.1.22` ×1 (omitted above), `0.0.1` ×1 — 80 parseable values, plus the two YAML-broken lines
(`version: [` and `version: [broken`), which is the 82-of-83 the task states.

### The acceptance criterion's prediction was wrong; the conclusion was not

The criterion expected `dva show` across `~/mydevbox` to fail "only under malformed\* fixtures".
The sweep gives 83 files, 7 failures — but **none of the 7 are caused by this change**. All of
them fail before the version gate is reached: four on YAML parse errors (including the two
`malformed-copy`/`malformed-dir` files, whose `version: [` is a *syntax* error, not a
version-format one) and three on `compose must be declared under runners.compose`. So the
`show` sweep cannot isolate this change's effect at all.

The decisive check is the one that isolates the variable: run every distinct real `version:`
value in the corpus through the new pattern. All five are accepted — zero regressions, which is
stronger than the task predicted.

Confirmed in both directions with the real `bin/dva`, not only in tests:

| fixture | `show` | `ls` | `doctor` | `validate` |
| --- | --- | --- | --- | --- |
| `version: "O.2.0"` | rc=1 | rc=1 | rc=1 | rc=1 |
| no `version:` key | rc=0 | rc=0 | rc=0 | rc=0 |

Before this change the first row was rc=0 on all four.

## Evidence

Measured 2026-07-30 with `bin/dva` at `config.Version = 0.1.44`, against a scratch `dva.yml`
outside both this repo and `~/mydevbox`, varying only the `version:` line — the table above.

The mechanism was read from source, not inferred from the outputs: `fmt.Sscanf`'s error is
assigned to `_`, and `isVersionCompatible` compares the three segments in order, returning true
when they are equal — so `[0,0,0]` vs any running version takes the `cur[i] > req[i]` branch on
the first non-zero segment.

Corpus figures come from `/usr/bin/find` + `/usr/bin/grep` on absolute paths, not the
gitignore-honoring `grep` function this shell defines.
