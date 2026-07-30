---
id: TASK-070
title: "A malformed `version:` is read as 0.0.0 and passes the compatibility gate everywhere — the gate misses exactly what it exists to catch"
type: fix
priority: P3
status: todo
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
without quotes is a YAML **number**, and the two layers disagree about it:

- `dva validate` rejects it — `version: Invalid type. Expected: string, given: number` — because
  the YAML→JSON conversion preserves the numeric type.
- `config.Load` accepts it, because yaml.v3 coerces a numeric scalar into the Go `string` field,
  so `checkConfigVersion` sees `"0.1"` and is satisfied.

Leave it alone. Making these agree means changing how YAML scalars are decoded, which is a much
larger blast radius than this task, and the schema is already rejecting the shape at the one
command where a user is asking to be told about problems.

Note the interaction, though: once `parseVersion` is strict, `version: 0.1` (as the string
`"0.1"`) becomes a **parse failure** at load time rather than a silently-accepted `0.1.0`. So
the accepted pattern must allow a two-segment version, or this task changes behavior for
unquoted values it did not intend to touch. That is why the suggested pattern below has an
optional patch segment.

## Fix shape — XS

Have `parseVersion` report failure instead of returning `[0,0,0]`, and let `checkConfigVersion`
surface it as the malformed-value error it is — naming the offending string and the expected
shape, since the whole point is that a typo is currently invisible.

Optionally also add `"pattern": "^v?\\d+\\.\\d+(\\.\\d+)?$"` to the schema's `version` property
so `dva validate` reports it too. Secondary: the Go gate is what matters, because every command
except `validate` bypasses the schema entirely (`Config.Validate()` has exactly one call site,
`internal/cli/validate.go:23-25`).

**This is a tightening, unlike [TASK-067](067-version-field-rule-stated-three-incompatible-ways.md)'s
rule A.** It can turn a config that loads today into one that does not, so the corpus check is
a precondition, not a formality.

## Corpus risk

Checked before proposing this. Under `~/mydevbox`, 82 of 83 `dva.yml` files declare a
top-level `version:`; the values are `0.1.44` ×53, `0.1.26` ×21, `0.1.0` ×4, `0.0.1` ×1, all of
which parse. The only non-parsing values — and the single omission — live under directories
named `malformed`, `malformed-copy`, `malformed-dir` and `malformed-fixture`, which are
deliberate negative-test fixtures (one contains `schema_version: "1.2"` and
`plans: "this-must-be-an-object"`). So no real config regresses.

Confirm this again at implementation time rather than trusting the numbers above: they were
measured on 2026-07-30 and the corpus is the user's working tree.

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

- [ ] A malformed `version:` fails to load instead of reading as 0.0.0 | verify: `go test ./internal/config/ -run TestParseVersionRejectsMalformed`
- [ ] The error names the offending value | verify: `go test ./internal/config/ -run TestParseVersionRejectsMalformed`
- [ ] `"9.9.9"` still reports the incompatibility, not a parse error | verify: `go test ./internal/config/ -run TestCheckConfigVersion`
- [ ] An absent or empty `version:` still loads | verify: `go test ./internal/config/ -run TestCheckConfigVersion`
- [ ] A two-segment version still parses | verify: `go test ./internal/config/ -run TestParseVersionRejectsMalformed`
- [ ] Full suite green | verify: `make test`
- [ ] No real corpus config regresses | verify: `human — dva show across ~/mydevbox, expect failures only under malformed* fixtures`

## Evidence

Measured 2026-07-30 with `bin/dva` at `config.Version = 0.1.44`, against a scratch `dva.yml`
outside both this repo and `~/mydevbox`, varying only the `version:` line — the table above.

The mechanism was read from source, not inferred from the outputs: `fmt.Sscanf`'s error is
assigned to `_`, and `isVersionCompatible` (`config.go:1147-1160`) compares the three segments
in order, returning true when they are equal — so `[0,0,0]` vs any running version takes the
`cur[i] > req[i]` branch on the first non-zero segment.

Corpus figures come from `/usr/bin/find` + `/usr/bin/grep` on absolute paths, not the
gitignore-honoring `grep` function this shell defines, and the three anomalous files were read
individually to confirm they are fixtures rather than real configs.
