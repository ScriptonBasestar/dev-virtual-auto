---
id: TASK-045
title: "checks[].fix is implemented and works, and dva validate rejects it as an unknown property"
type: bug
priority: P3
status: todo
needs-human: true
effort: S
created-at: 2026-07-17T08:05:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: fresh Phase 1 sweep (schema.json examples lens)
source-severity: LOW
---

# Task 045: The Feature Works, The Validator Says It Is Invalid

## Summary

`checks[].fix` is a **working, implemented feature**: `dva doctor --fix` executes it and it does its
job. `dva validate` rejects any config that uses it:

```
ERROR: schema validation failed in dva.yml:
  - checks.0: Additional property fix is not allowed
```

Three sources of truth disagree about one field:

| Source | Says |
|---|---|
| `internal/config/config.go:48` | `Fix string \`yaml:"fix"\`` — "shell command to auto-fix (used by `dva doctor --fix`)" |
| `internal/cli/doctor.go:202-204` | reads it and executes it |
| `internal/config/schema.json:1460-1488` | `additionalProperties: false`, and `fix` is **not** in the property list |
| `docs/`, `examples/`, `README.md`, `USAGE.md` | silent — the field appears nowhere |

This is the **inverse** of TASK-035/036/037/040/044. Those are fields the schema accepts and the code
ignores. This is a field the code honors and the schema forbids.

## Evidence — the schema enforcement is real, not editor decoration

`internal/config/validate.go:15-16` embeds the schema and `:51-54` enforces it with `gojsonschema`:

```go
//go:embed schema.json
var embeddedSchema embed.FS
...
result, err := gojsonschema.Validate(schemaLoader, docLoader)
```

So `additionalProperties: false` at `schema.json:1460` is a hard `dva validate` failure, not a hint.
The allowed property list (`schema.json:1462-1487`) is exactly `name`, `type`, `path`, `command`,
`fix_hint`. There is no `fix`.

## Evidence — measured at `c6c8447`

### Liveness gate + RED control (the check is real and really evaluated)

Same config, `fix:` omitted — schema-legal:

```yaml
version: "0.1.0"
checks:
  - name: "Sentinel file exists"
    type: file_exists
    path: .sentinel
    fix_hint: "touch .sentinel"
```

```
$ dva validate
✅ dva.yml is valid
VALIDATE_EXIT=0            # <-- gate holds: the shape loads

$ dva doctor              # .sentinel absent
  [FAIL] Sentinel file exists
         -> touch .sentinel
```

The check is **live and RED** — it genuinely evaluates and genuinely fails. So the probe is not
vacuous, and anything that changes when `fix:` is added is attributable to `fix:` alone.

### The decisive pair — add `fix:`, change nothing else

```yaml
checks:
  - name: "Sentinel file exists"
    type: file_exists
    path: .sentinel
    fix_hint: "touch .sentinel"
    fix: "touch .sentinel"      # <-- the only difference
```

```
$ dva validate
ERROR: schema validation failed in dva.yml:
  - checks.0: Additional property fix is not allowed
VALIDATE_EXIT=1            # <-- the validator rejects the config

$ dva doctor --fix
  [fixed] Sentinel file exists
  4 passed, 1 failed (2 auto-fixed)
DOCTOR_FIX_EXIT=0

$ ls .sentinel
-rw-r--r--  0  .sentinel  # <-- the fix REALLY RAN. The file exists.
```

The same config is "invalid" to `dva validate` and fully functional to `dva doctor --fix`. The
feature is not theoretical: the sentinel file was created by the `fix:` command.

## Why it matters — and why it is only P3

The failure is **loud**: `dva validate` names the exact property and exits 1. Nothing is silently
skipped, nothing is silently mutated. This is the harmless direction — the opposite of TASK-044,
where a green surface certified work that never happened.

Exposure is also small, because the field is **undiscoverable**: it is in no doc, no example, and —
crucially — not in the schema, so editors will never complete it. A user essentially has to read
`config.go` to learn it exists.

But it is a real defect with a real cost:

- `dva doctor --fix`'s own flag help (`doctor.go:57`) advertises "Automatically fix issues that can
  be resolved". For **user-defined** checks, `fix:` is the only mechanism that does this — and any
  config using it fails `dva validate`. So the advertised capability is unreachable for user checks
  through a validating workflow. (Built-in fixes, like the `.gitignore` one, still work.)
- A user who does discover `fix:` and puts it in CI gets a red `dva validate`, and the natural
  response is to delete a working feature.
- Editors that consume the schema will mark a **correct, working** config as invalid — the mirror
  image of the TASK-015 reasoning, where a wrong schema example was worth P2 because editors surface
  schema content.

## Scope note — needs a decision

Two coherent options, and the choice is not the implementer's:

- **Expose it** — add `fix` to `schema.json`'s checks properties (a ~4-line change next to
  `fix_hint` at `:1482`) and document it. This matches what the code already does and costs almost
  nothing.
- **Remove it** — drop `Fix` from `DoctorCheck` (`config.go:48`) and the fix path at
  `doctor.go:202-214`.

**The omission may well be deliberate, and that is exactly why this needs a human.** `fix:` runs
**arbitrary shell from a config file** (`doctor.go:208`: `exec.CommandContext(ctx, "sh", "-c", fixCmd)`)
— on `dva doctor --fix`, against a `dva.yml` that may have arrived via `subprojects:` or a cloned
repo. A maintainer may have consciously chosen not to advertise config-driven shell execution in the
machine-readable contract. If so, the correct resolution is **remove the code**, not add the schema
entry — because leaving it live-but-unlisted gives the security exposure with none of the usability.

Lean **expose**, weakly: `fix_hint` already tells the user to run a shell command by hand, and
`checks[].type: command` (`schema.json:1469`) *already* executes arbitrary config-supplied shell on a
plain `dva doctor` with no flag at all — so the schema has already accepted this class of execution,
and `fix:` behind an explicit `--fix` flag is strictly more guarded than what `type: command` does by
default. That undercuts the security argument for keeping it hidden, but does not settle it: running
a *diagnostic* and running a *mutation* are different consent levels, and only a maintainer can say
whether `--fix` is sufficient consent.

Either way, the current state — implemented, functional, unlisted, and rejected by the tool's own
validator — is not a defensible resting point.

## Completion Criteria

- [ ] DECISION recorded: expose `checks[].fix` in the schema, or remove it from the code | verify: `human — maintainer picks one and records why; the arbitrary-shell-execution question is the crux, see Scope note`
- [ ] If EXPOSE: `fix` is in schema.json's checks properties and a config using it validates | verify: `human — reproduce the decisive pair: the sentinel config with 'fix:' must give VALIDATE_EXIT=0 and 'dva doctor --fix' must still create .sentinel`
- [ ] If EXPOSE: `fix` is documented, including that it executes arbitrary shell on --fix | verify: `human — assert the doc states the execution semantics, not just the syntax`
- [ ] If REMOVE: no reader of `DoctorCheck.Fix` remains | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && ! grep -rn "\.Fix\b" --include="*.go" . | grep -v FixHint`
- [ ] If REMOVE: `dva doctor --fix` still performs built-in fixes, or the flag's help stops promising more than it does | verify: `human — the built-in .gitignore fix is independent of checks[].fix; confirm it survives or the flag help is corrected`
- [ ] A regression test pins whichever contract is chosen, proven to fail without the change | verify: `human — for EXPOSE, revert the schema line and confirm the test FAILS on 'Additional property fix is not allowed'; restore and confirm it passes`
- [ ] The schema-legal control still validates — no collateral damage to checks without `fix:` | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && go test ./internal/config/`
- [ ] `make test` and `go vet ./...` pass | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && make test && go vet ./...`

## References

- [046-doctor-exits-zero-when-checks-fail.md](./046-doctor-exits-zero-when-checks-fail.md) — found in the same probe; both are `dva doctor` contract gaps and should be decided together
- [015-fix-schema-version-example.md](../_archive/015-fix-schema-version-example.md) — the precedent that schema content is P2-worthy because editors surface it; here the schema's *omission* makes editors flag a working config
- [044-legacy-structured-provision-shell-sleep-docker-inert.md](./044-legacy-structured-provision-shell-sleep-docker-inert.md) — the inverse direction: schema accepts, code ignores. This one: code honors, schema rejects
- [026-shipped-examples-validate-green-runtime-red.md](./026-shipped-examples-validate-green-runtime-red.md) — validate-green/runtime-red; this task is validate-red/runtime-green, the harmless direction
