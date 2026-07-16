---
id: TASK-027
title: "dva up silently ignores an unknown plan name and starts the entire stack"
type: bug
priority: P1
status: done
effort: S
completed-at: 2026-07-17T01:30:00+09:00
verified-by: orchestrator (independent re-run of every criterion against a rebuilt binary, plus a positive control on the regression test)
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

## Resolution

`rejectUnknownPlanArg` (`internal/cli/plan_lifecycle.go`) now fires on `up`'s non-plan
fallthrough, called from `compose.go:87` **before** `parseDvaFlags` so it reads raw args:

```
$ dva up p1-typo
ERROR: plan 'p1-typo' not found. Available: p1
EXIT=1
```

This mirrors `down`'s existing behavior via `teardownCommon`, closing the asymmetry the task
identified, and satisfies `docs/31:59`'s "없으면 즉시 validation error".

### Why it reads only `args[0]`

The guard inspects **only `args[0]`**, exactly the slot `detectPlanRoute` treats as the plan
name, and returns early when `args[0]` starts with `-`. This scoping is the crux of the fix:

- A wider guard that scanned every arg would read `dva up --var FOO=x` as plan `'FOO=x'` and
  error — turning a silent bug into a loud regression. The orchestrator predicted exactly this
  failure; the implementation had already been revised to avoid it. Confirmed: `dva up --var
  FOO=x` → exit 0, no misparse.
- Mirroring `detectPlanRoute` means the guard fires **only** where a plan was looked for and
  not found. It cannot invent errors on paths that were never plan-routed.

Known limitation, deliberately not fixed here: `dva up --dev p1-typo` does **not** catch the
typo (`args[0]` is a flag, so `detectPlanRoute` never treated it as plan-routed). Confirmed
exit 0. Fixing this requires `detectPlanRoute` to skip leading flags, which is precisely the
undecided **TASK-028**. Widening the guard beyond `detectPlanRoute`'s own contract would
pre-commit that decision.

## Verification (orchestrator, 2026-07-17)

Every criterion re-run independently against a freshly built `bin/dva` — not taken from the
implementation agent's report:

```
C1  dva up p1-typo                 -> exit=1, "plan 'p1-typo' not found. Available: p1"  ✅
C2  dva up p1                      -> MARKERS1 present, MARKERS2 absent (plan-scoped)     ✅
C3  no-plans config + dva up       -> MARKERS1 (legacy path intact)                       ✅
    no-plans config + dva up ARG   -> exit=0 (legacy tolerance preserved)                 ✅
C4  go test ./internal/cli/ -run 'TestUp(Rejects|Accepts|Without|PlanGuard)' -> 4 PASS    ✅
C5  make test -> exit=0, 0 FAIL   ·  go vet ./... -> exit=0, no output                    ✅
```

**Positive control on C4** — the check that the check works. With the guard call disabled
(`if false`), `TestUpRejectsUnknownPlanName` **fails**:

```
--- FAIL: TestUpRejectsUnknownPlanName
    'dva up p1-typo' returned nil; an unknown plan name must not silently start the whole stack
```

while the three regression-guard tests still pass — correct, since they assert preserved
behavior the fix must not touch. So the new test binds to this fix rather than passing
vacuously. (This run has already produced one green check that never executed; a test not
observed failing is not evidence.)

Test hermeticity confirmed: `plan_lifecycle_test.go` uses `t.TempDir` + `t.Chdir` and resets
the cached package-level `cfg` to nil with a `t.Cleanup` restore, so the loader cannot walk up
to the repository's own `dva.yml` and assert against the wrong fixture.

## Completion Criteria

- [x] `dva up <unknown-name>` exits non-zero and names the unknown argument | verify: `cd $(mktemp -d) && printf 'version: "0.1.0"\nstack:\n  s1:\n    default_runner: script\n    runners:\n      script:\n        up: echo s1\nplans:\n  p1:\n    entries:\n      - name: s1\n' > dva.yml && ! /Users/archmagece/mywork/scripton/dev-virtual-auto/bin/dva up p1-typo`
- [x] `dva up p1` still resolves the real plan and runs only its entries | verify: `cd $(mktemp -d) && printf 'version: "0.1.0"\nstack:\n  s1:\n    default_runner: script\n    runners:\n      script:\n        up: echo MARKERS1\n  s2:\n    default_runner: script\n    runners:\n      script:\n        up: echo MARKERS2\nplans:\n  p1:\n    entries:\n      - name: s1\n' > dva.yml && /Users/archmagece/mywork/scripton/dev-virtual-auto/bin/dva up p1 2>&1 | grep -q MARKERS1 && ! /Users/archmagece/mywork/scripton/dev-virtual-auto/bin/dva up p1 2>&1 | grep -q MARKERS2`
- [x] `dva up` with no plans defined still starts the stack (no regression to the legacy path) | verify: `cd $(mktemp -d) && printf 'version: "0.1.0"\nstack:\n  s1:\n    default_runner: script\n    runners:\n      script:\n        up: echo MARKERS1\n' > dva.yml && /Users/archmagece/mywork/scripton/dev-virtual-auto/bin/dva up 2>&1 | grep -q MARKERS1`
- [x] A regression test covers `up <unknown>` failing | verify: `make test`
- [x] `make test` and `go vet ./...` pass | verify: `make test && go vet ./...`

## Out Of Scope

- Adding `--var` to the `--help` surfaces, and whether the non-plan path should honor `--var`
  rather than reject it. Recorded here as evidence; separate change.

## References

- [031-execution-plan-resolution](../../docs/31-execution-plan-resolution.md) — §4-1 specifies the validation error
- [009-fix-runners-plugin-resolution.md](../_archive/009-fix-runners-plugin-resolution.md) — prior silent-success finding
