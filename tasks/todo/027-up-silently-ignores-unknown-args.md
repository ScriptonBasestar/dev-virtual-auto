---
id: TASK-027
title: "dva up silently ignores an unknown plan name and starts the entire stack"
type: bug
priority: P1
status: todo
effort: S
created-at: 2026-07-17T00:20:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: TASK-025 verification
source-severity: HIGH
---

# Task 027: `dva up <typo>` Silently Escalates To The Whole Stack

## Summary

`dva up <name>` silently discards an unrecognized positional argument and starts **every**
stack entry. A typo in a plan name turns a scoped "start these two services" into "start
everything", with exit 0 and no warning.

`docs/31-execution-plan-resolution.md:59` specifies the opposite:

> 1. CLI 인자로 `<name>`을 받음
> 2. `plans.<name>` 또는 import된 canonical name을 조회
> 3. **없으면 즉시 validation error**

## Evidence

Reproduced at HEAD `1719e25` with a two-entry stack and one plan `p1` covering only `s1`:

```
$ dva up p1            # control -- plan-scoped
MARKER s1 ran
EXIT=0                 # only s1. correct.

$ dva up p1-typo       # one character off
MARKER s1 ran
MARKER s2 ran          # <-- s2 was NOT in the plan
EXIT=0                 # no error, no warning
```

The argument is not overloaded to entry names either — `dva up s1` (a real entry, not a plan)
also runs both `s1` and `s2`. **Any** non-plan argument is discarded.

## Root cause

`internal/cli/plan_lifecycle.go:26-30` — an unknown name is simply "not a plan route":

```go
if len(args) > 0 {
    if _, exists := c.Plans[args[0]]; exists {
        return args[0], args[1:], true
    }
    return "", nil, false   // unknown name -> fall through, arg still in args
}
```

`internal/cli/compose.go:95-105` — `up`'s fallthrough then loops over those args with a
`switch` carrying **only flag cases and no `default`**, so the leftover positional arg is
silently dropped:

```go
for _, a := range args {
    switch a {
    case "--force":   force = true
    case "--no-wait": noWait = true
    case "--dev":     devMode = true
    case "--docker":  docker = true
    }                        // <-- no default: unknown args vanish
}
```

## Why this is not a design question

Unlike TASK-017/019, no product decision is needed — the intended behavior is already
specified and already implemented elsewhere:

- `docs/31:59` states an unknown name must be a validation error.
- **`dva down` already does exactly that.** Same dispatcher, different fallthrough:
  `runPlanDown` is skipped identically, but `down` falls through to
  `teardownCommon(args, "down")` (`compose.go:242`), which validates and exits 1.

```
$ dva down p1-typo   -> EXIT=1     (correct)
$ dva up   p1-typo   -> EXIT=0     (runs everything)
```

The asymmetry is the bug. `up` is the one command where a typo silently does *more* than asked.

## Related symptom, same root cause

`--var` is parsed only by `parsePlanFlags` (`plan_lifecycle.go:40`), which is reachable solely
from the plan path. Off that path it is silently swallowed by the same defaulted-out `switch`:

```
$ dva up p1 --var FOO=plan        -> MARKER FOO=[plan]    (works)
$ dva up --var FOO=bare           -> MARKER FOO=[]        (silently ignored, exit 0)
$ dva stack up s1 --var FOO=stack -> MARKER FOO=[]        (silently ignored, exit 0)
$ dva run showvar --var BAR=cli   -> ERROR: unknown flag: --var, exit 1   (correct)
```

`dva run` rejects it loudly; the stack path swallows it. Note `--var` is absent from every
`--help` surface (`dva --help`, `up`, `stack up`, `app up`) despite being documented at
`USAGE.md:175`, so a user cannot discover it from the binary either.

## Severity

HIGH. This is the harmful direction with **no gate in front of it**: `dva validate` never sees
CLI arguments, so nothing catches the typo. Every other silent-success finding this run was
either fixed (TASK-009) or shielded by `dva validate` rejecting the config first. This one
reaches the user directly, and its failure mode is starting unintended infrastructure.

## Completion Criteria

- [ ] `dva up <unknown-name>` exits non-zero and names the unknown argument | verify: `cd $(mktemp -d) && printf 'version: "0.1.0"\nstack:\n  s1:\n    default_runner: script\n    runners:\n      script:\n        up: echo s1\nplans:\n  p1:\n    entries:\n      - name: s1\n' > dva.yml && ! /Users/archmagece/mywork/scripton/dev-virtual-auto/bin/dva up p1-typo`
- [ ] `dva up p1` still resolves the real plan and runs only its entries | verify: `cd $(mktemp -d) && printf 'version: "0.1.0"\nstack:\n  s1:\n    default_runner: script\n    runners:\n      script:\n        up: echo MARKERS1\n  s2:\n    default_runner: script\n    runners:\n      script:\n        up: echo MARKERS2\nplans:\n  p1:\n    entries:\n      - name: s1\n' > dva.yml && /Users/archmagece/mywork/scripton/dev-virtual-auto/bin/dva up p1 2>&1 | grep -q MARKERS1 && ! /Users/archmagece/mywork/scripton/dev-virtual-auto/bin/dva up p1 2>&1 | grep -q MARKERS2`
- [ ] `dva up` with no plans defined still starts the stack (no regression to the legacy path) | verify: `cd $(mktemp -d) && printf 'version: "0.1.0"\nstack:\n  s1:\n    default_runner: script\n    runners:\n      script:\n        up: echo MARKERS1\n' > dva.yml && /Users/archmagece/mywork/scripton/dev-virtual-auto/bin/dva up 2>&1 | grep -q MARKERS1`
- [ ] A regression test covers `up <unknown>` failing | verify: `make test`
- [ ] `make test` and `go vet ./...` pass | verify: `make test && go vet ./...`

## Out Of Scope

- Adding `--var` to the `--help` surfaces, and whether the non-plan path should honor `--var`
  rather than reject it. Recorded here as evidence; separate change.

## References

- [031-execution-plan-resolution](../../docs/31-execution-plan-resolution.md) — §4-1 specifies the validation error
- [009-fix-runners-plugin-resolution.md](../_archive/009-fix-runners-plugin-resolution.md) — prior silent-success finding
