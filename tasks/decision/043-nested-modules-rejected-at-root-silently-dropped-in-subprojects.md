---
id: TASK-043
title: "Nested modules are a hard error at the root and a silent drop in subprojects"
type: bug
priority: P3
status: decision
needs-human: true
effort: S
created-at: 2026-07-16T22:55:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: fresh Phase 1 sweep (config surfaces; merge-completeness audit)
source-severity: LOW
moved-at: 2026-07-17T10:55:00+09:00
---

# Task 043: The Same YAML Errors In One Load Path And Is Ignored In The Other

## Summary

A module that declares its own `modules:` is **rejected with a clear error** when loaded from the root
`dva.yml`, and **silently not loaded** when loaded from a subproject. Same YAML, same nesting, two
different answers, and the quiet one produces a misleading error that blames the wrong file.

## Root cause (by construction)

The root path checks; the subproject path does not.

`internal/config/config.go:751-753` — root module loading:

```go
if len(modCfg.Modules) > 0 {
    return nil, fmt.Errorf("nested modules are not supported")
}
```

`internal/config/subproject.go:27-39` — subproject module loading. Structurally the same loop
(`loadFile` → `mergeFrom`), and the comment even says *"same `.sb/dva/*.yml` pattern"* — but the
nested check is absent:

```go
if len(cfg.Modules) > 0 {
    modulesDir := filepath.Join(subPath, DotDirName)
    for _, mod := range cfg.Modules {
        modFile := filepath.Join(modulesDir, mod+".yml")
        modCfg, err := loadFile(modFile)
        ...
        if err := cfg.mergeFrom(modCfg); err != nil { ... }   // <-- no modCfg.Modules check
    }
}
```

`mergeFrom` does not merge `Modules` (it is one of only two top-level fields it skips — the other is
`Version`, see TASK-042). So `modCfg.Modules` is parsed into the struct and then dropped: the nested
module file is **never opened**, and nothing says so.

## Evidence — measured, both paths, same shape

### Control — the root path rejects it, loudly

```
dva.yml            modules: [mod1]
.sb/dva/mod1.yml   modules: [nested1]        # nested
.sb/dva/nested1.yml  interaction.from_nested: echo HELLO_FROM_NESTED

$ dva validate
ERROR: nested modules are not supported
VALIDATE_EXIT=1
```

Unambiguous, names the actual problem, and is trivially actionable by the user.

### The finding — the subproject path, same nesting

```
dva.yml                     subprojects.sub.path: ./sub
                            subprojects.sub.import.interactions: [{name: sub_from_nested}]
sub/dva.yml                 modules: [submod]
sub/.sb/dva/submod.yml      modules: [subnested]      # nested — identical shape to the control
sub/.sb/dva/subnested.yml   interaction.sub_from_nested: echo HELLO_SUB_FROM_NESTED

$ dva validate
ERROR: resolving subprojects: subproject "sub" interaction "sub_from_nested" not found
VALIDATE_EXIT=1
```

No "nested modules are not supported". The user is told their **import** names a missing interaction.
The interaction is not missing — it is defined, in `subnested.yml`, exactly where they put it. The
real cause is that `subnested.yml` was never loaded, and the error never mentions modules at all.

### Positive control — the outer subproject module DID load

Rules out "the whole subproject module chain was skipped, so of course the nested one was too". With
the import pointed one file up, at an interaction defined in `submod.yml`:

```
$ dva validate                    -> ✅ dva.yml is valid          EXIT=0
$ dva run sub/sub_from_module     -> HELLO_SUB_FROM_MODULE        EXIT=0
```

So the subproject's module loads and merges normally. Only the **nested** level is dropped. The
absence is specific, not incidental — which is what makes the misleading error a real defect rather
than a side effect of an unrelated failure.

**This control is why the finding is stated this narrowly.** The first version of this probe imported
only `sub_hello` and observed that `sub_from_nested` was absent — which proved nothing, because
`sub_from_module` was absent too, for the unrelated reason that the import list simply did not name
it. Recorded because the wrong conclusion was available and cheap, and only the control excluded it.

## Impact — and the honest bound on it

Two cases, and only one is silent:

1. **Something is imported from the nested module** (probed above): the user gets an error, but it
   blames the import for naming a missing interaction. Debugging leads to the import and the
   interaction — both correct — rather than to the nesting, which is the actual cause and is not
   mentioned. Recoverable, but the diagnostic points away from the answer.
2. **Nothing is imported from the nested module**: fully silent. The subproject's config tree is
   quietly smaller than written. Bounded in practice, because a subproject only contributes to the
   parent through `import:` (plans / interactions / provision — `subproject.go:84-163`), so a dropped
   nested module can only matter by way of an import, which lands back in case 1.

That bound is why this is P3 and not higher: the failure mode is a misleading message, not a wrong
result. Nothing is mutated, nothing is destroyed, and no green surface certifies a broken stack — the
config does fail, just with the wrong sentence.

## Severity: LOW / P3

Filed despite the low severity because it is a **two-line inconsistency with a one-line intent already
written in the code**. The root path states the rule (`"nested modules are not supported"`); the
subproject path was evidently meant to mirror it — its own comment says *"same `.sb/dva/*.yml`
pattern"* — and does not. The cost of the gap is a user chasing the wrong file.

## Scope note — needs a decision

Smaller than TASK-042's, but still not unilateral:

- **Reject (make it consistent)** — add the same `len(modCfg.Modules) > 0` check to
  `subproject.go:31`. Two lines. Matches the intent already stated at the root and turns a misleading
  error into an accurate one. **Back-compat caveat, and it is the real reason this is not just done:**
  a config with a nested module in a subproject loads today (the nesting is ignored, nothing else
  breaks). Adding the check makes that config start failing. The nesting never *worked* — nobody can
  depend on its effect — but they can depend on the file loading, and this would be a new hard error
  on a config that is green today. That is a maintainer's call to make.
- **Support nesting in both paths** — the opposite direction. Requires deciding merge order and
  cycle detection (`a → b → a`), neither of which exists today. Materially more work, and the root
  path's explicit rejection reads as a deliberate "no", not an oversight.

Lean **reject/consistent**, weakly: it makes the code match its own stated intent, and the alternative
requires designing cycle handling to support something the root path already refuses. But the
back-compat break is real and is the whole decision.

**Decide together with TASK-042** — both are `loadFile()` being the ungated loader while `Load()` is
the gated one, and the two fields `mergeFrom` skips are precisely `Version` and `Modules`. One coherent
answer about what the non-root load paths enforce plausibly settles both.

## Completion Criteria

- [ ] DECISION recorded: reject nested modules everywhere, or support them everywhere | verify: `human — maintainer picks one and records why; note the back-compat break in the reject direction`
- [ ] The decision is made together with TASK-042, or the reason for splitting them is recorded | verify: `human — both stem from loadFile() being ungated; confirm one coherent answer`
- [ ] If REJECT: a subproject module declaring `modules:` fails with the same message as the root path | verify: `human — reproduce the probe; assert the error says nested modules are not supported and names the subproject, NOT "interaction not found"`
- [ ] If REJECT: the back-compat break is acknowledged — configs that load today will fail | verify: `human — confirm this is intended, and that it is worth an accurate error; note the nesting never took effect, so only the load succeeds today, not the behavior`
- [ ] If SUPPORT: cycle detection exists (a -> b -> a must not hang or overflow) | verify: `human — neither path has any cycle detection today; a probe with a two-module cycle must terminate with a clear error`
- [ ] If SUPPORT: merge order is documented and tested | verify: `human — nested merge order is unspecified today; depth-first vs breadth-first changes which value wins`
- [ ] Either way, a test covers the subproject nested-module path, proven to fail without the change | verify: `human — revert, confirm the new test FAILS for the right reason, restore, confirm it passes`
- [ ] The root path's rejection does not regress | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && go test ./internal/config/`
- [ ] `make test` and `go vet ./...` pass | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && make test && go vet ./...`

## References

- [042-version-gate-bypassed-for-modules-and-subprojects.md](./042-version-gate-bypassed-for-modules-and-subprojects.md) — same root cause (`loadFile` is ungated); decide together
- [006-subprojects.md](../_archive/006-subprojects.md) — the subproject load path this finding is in
- [003-merge-semantics.md](../_archive/003-merge-semantics.md) — merge semantics; `mergeFrom` skipping `Modules` is what makes the drop silent
