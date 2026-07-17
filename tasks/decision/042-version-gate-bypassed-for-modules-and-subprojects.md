---
id: TASK-042
title: "The minimum-version gate fires only for the root dva.yml; modules and subprojects bypass it entirely"
type: bug
priority: P2
status: decision
needs-human: true
effort: S
created-at: 2026-07-16T22:50:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: fresh Phase 1 sweep (config surfaces; merge-completeness audit)
source-severity: MEDIUM
moved-at: 2026-07-17T10:55:00+09:00
---

# Task 042: `version: "99.0.0"` In A Module Is Validated Green And Then Executed

## Summary

`version:` is a **minimum-DVA-version gate**: a config declaring `version: "99.0.0"` means "refuse to
run unless dva >= 99". The root `dva.yml` enforces it. A **module** or **subproject** declaring the
same thing is loaded, merged, validated `✅ valid`, and **executed** — the requirement is silently
discarded.

This is the validate-green / runtime-wrong direction: `dva validate` prints a green checkmark over a
config tree that explicitly says it cannot run on this dva.

## Root cause (by construction — no probe needed to see it)

There are two loaders, and only one of them gates.

`Load()` performs the version check **once, on the root config only** (`internal/config/config.go:736`):

```go
if !o.skipVersionCheck && cfg.Version != "" {
    if !isVersionCompatible(cfg.Version) {
        return nil, fmt.Errorf("your dva version is `%s`, but config requires minimum version `%s`...")
    }
}
```

`loadFile()` (`internal/config/config.go:858-870`) is the raw loader — `os.ReadFile` + `yaml.Unmarshal`,
and **nothing else**. It applies no gates at all.

Every module and every subproject enters through `loadFile()`:

| Path | Call site | Version-gated? |
|---|---|---|
| root `dva.yml` | `Load()` → check at `config.go:736` | **yes** |
| root module | `config.go:747` `loadFile(modFile)` | no |
| root override | `config.go:762` `loadFile(overrideFile)` | no |
| subproject `dva.yml` | `subproject.go:20` `loadFile(subCfgPath)` | no |
| subproject module | `subproject.go:31` `loadFile(modFile)` | no |
| subproject override | `subproject.go:43` `loadFile(overrideFile)` | no |

`mergeFrom()` compounds it: `Version` is one of only **two** top-level `Config` fields it never
touches (the other is `Modules`, see TASK-043). So a module's `Version` is not merged into the root
either — it is read into the struct and dropped on the floor.

Audit of `mergeFrom` (`config.go:881-1091`) against the 22 top-level fields (`config.go:15-36`):
every field is merged **except** `Version` and `Modules`.

## Evidence — measured, with the control that makes it mean something

Probe tree (`dva validate` EXIT=0 on the final shape — the liveness gate holds, so "nothing happened"
is not trivially true):

```
dva.yml                     version: "0.1.0",  modules: [mod1]
.sb/dva/mod1.yml            version: "99.0.0", interaction.from_module: echo HELLO_FROM_MODULE
```

### Control — the gate is real and CAN fire

Same `version: "99.0.0"`, declared at the **root** instead:

```
$ dva validate
ERROR: your dva version is `0.1.44`, but config requires minimum version `99.0.0`. Please upgrade dva
VALIDATE_EXIT=1
```

So the gate works, the version string is well-formed, and `99.0.0` is genuinely incompatible with the
shipped `0.1.44`. Nothing about the probe is malformed.

### The finding — the same declaration, one file down

```
$ dva validate
[warn] semantic: dva.yml version "0.1.0" is older than running DVA "0.1.44"; consider updating
✅ dva.yml is valid
VALIDATE_EXIT=0
```

### Positive control — the module was NOT merely ignored; it was USED

This is the part that rules out "the module never loaded, so of course nothing happened":

```
$ dva run from_module
HELLO_FROM_MODULE
RUN_EXIT=0
```

The module was loaded, merged, and its interaction executed — while its own declared minimum version
was discarded. The requirement did not fail to apply because the file was skipped; it failed to apply
because nothing ever checks it.

### Same result one level deeper (subproject module)

```
sub/.sb/dva/submod.yml      version: "99.0.0", interaction.sub_from_module: echo HELLO_SUB_FROM_MODULE

$ dva validate                             -> ✅ dva.yml is valid            EXIT=0
$ dva run sub/sub_from_module              -> HELLO_SUB_FROM_MODULE          EXIT=0
```

## Why it matters

The gate exists to turn a confusing downstream failure into one clear sentence: *"upgrade dva"*.
Bypassing it does not make the incompatibility disappear — it just relocates the symptom to whatever
breaks first, with no mention of the version. That is the entire value of the gate, lost precisely in
the case it was built for: **a module is the most likely thing to be authored against a newer dva**,
because modules are the shared/vendored unit (`.sb/dva/*.yml`).

Partial mitigation, recorded honestly: `schema.json` uses `additionalProperties: false` in places, so
a module using *new keys* from a future dva may be caught by schema validation with a different error.
That does not cover **semantic** changes (same keys, changed meaning), which is exactly what a version
gate is for. So the mitigation narrows the blast radius; it does not close the gap.

Related but distinct: TASK-015 fixed the schema's `version` *example* (`"8.1.0"`) that could never
load. That task establishes the gate is understood as real and load-bearing; this task is that the
gate does not run on most of the files it is documented to govern.

## Severity: MEDIUM / P2

Not P1: nothing is mutated or destroyed, and the user does eventually hit *an* error if the module is
genuinely incompatible — just the wrong one.

P2 rather than P3 because `dva validate` is **the** gate users are told to trust, and here it returns
`✅ valid` for a config tree that contains an explicit, machine-readable statement that it cannot run
on this dva. A green surface that certifies nothing is this run's organizing theme, and this is a
clean instance of it: the contradiction is not subtle or emergent — it is written in the file, in a
field designed to be checked, and simply never read.

## Scope note — needs a decision

Same honor-vs-reject shape as TASK-035/036/037/040, and not the implementer's call:

- **Honor it** — gate every loaded config, not just the root. Cheapest form: move the check into
  `loadFile()`, or call it after each `loadFile()` in `config.go:747`/`:762` and
  `subproject.go:20`/`:31`/`:43`. Note `Load()` has a `skipVersionCheck` option (`config.go:706`,
  set at `:713`) that must be threaded through, or the escape hatch silently stops working — a
  regression that would be easy to ship unnoticed, since nothing currently tests it.
- **Reject it** — decide `version:` is meaningful *only* at the root, and make that true rather than
  implied: have `schema.json` reject `version:` in module files, or have `validate` warn on it. This
  needs a way to tell a module file from a root file, which the schema currently cannot do (both are
  parsed as a full `Config`) — so this option is more work than it looks.

Lean **honor**, weakly: the field's whole purpose is to be checked, a module is the most likely file
to need it, and `isVersionCompatible` already exists and is already proven to work. But this changes
`Load()`'s failure surface — configs that load today would start failing — and whether a module may
raise the floor for the whole project is a real product question about what a module is allowed to do.
That is a maintainer's call, not an implementer's.

**Do not decide this in isolation from TASK-043.** Both findings are the same root cause — `loadFile()`
is an ungated loader while `Load()` is the gated one — and the two top-level fields `mergeFrom` skips
are exactly `Version` and `Modules`. A fix that gives `loadFile()` a gated variant plausibly resolves
both; deciding them separately risks two half-answers to one question.

## Completion Criteria

- [ ] DECISION recorded: is `version:` honored in modules/subprojects/overrides, or explicitly root-only? | verify: `human — maintainer picks one and records why; a documented "root-only is correct" closes this task legitimately, provided the docs say so`
- [ ] The decision is made together with TASK-043, or the reason for splitting them is recorded | verify: `human — both stem from loadFile() being ungated; confirm one coherent answer`
- [ ] If HONOR: `dva validate` exits non-zero when a module declares a version newer than the running dva | verify: `human — reproduce the probe: root version "0.1.0" + .sb/dva/mod1.yml version "99.0.0"; assert EXIT!=0 and the message names the module`
- [ ] If HONOR: the `skipVersionCheck` escape hatch still works through the new path | verify: `human — config.go:706/:713; assert the option still bypasses the check for every loaded file, not just the root`
- [ ] If HONOR: a regression test asserts the module gate, proven to fail without the change | verify: `human — revert the gate, confirm the new test FAILS for the right reason, restore, confirm it passes`
- [ ] If ROOT-ONLY: the docs state that `version:` is ignored outside the root dva.yml | verify: `human — a field silently ignored in 5 of 6 load paths must say so where users read about it`
- [ ] The root gate does not regress — it still fires | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && go test ./internal/config/`
- [ ] `make test` and `go vet ./...` pass | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && make test && go vet ./...`

## References

- [043-nested-modules-rejected-at-root-silently-dropped-in-subprojects.md](./043-nested-modules-rejected-at-root-silently-dropped-in-subprojects.md) — same root cause (`loadFile` is ungated); decide together
- [015-fix-schema-version-example.md](../_archive/015-fix-schema-version-example.md) — the version gate's example; establishes the gate as real and load-bearing
- [035-env-file-interpolate-and-priority-ignored.md](./035-env-file-interpolate-and-priority-ignored.md) — same class, same honor-vs-reject decision shape
- [039-plan-entry-runner-resolved-then-discarded.md](./039-plan-entry-runner-resolved-then-discarded.md) — same class: a value parsed, then discarded
