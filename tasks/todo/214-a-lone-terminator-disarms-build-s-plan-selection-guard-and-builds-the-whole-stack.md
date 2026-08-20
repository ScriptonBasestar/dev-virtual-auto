---
id: TASK-214
title: "A lone terminator disarms build's plan-selection guard and builds the whole stack"
type: bug
priority: P2
effort: S
created-at: 2026-08-20T17:20:00+09:00
source: "found by TASK-210's caller census — the card measured four verbs, the two functions it changed have seven callers, and build was the one whose terminator behaviour was already wrong"
scope: "internal/cli/compose.go buildCmd RunE, the requirePlanSelection call at :661. The routing helpers are not at fault; build is the one caller that cannot be backstopped by rejectUnknownFlags."
status: todo
---

# Task 214: A lone terminator disarms build's plan-selection guard and builds the whole stack

## Summary

In a config with several plans and no default, `dva build` refuses to guess.
Adding a `--` makes it stop refusing and build everything:

```
$ dva build
ERROR: multiple plans configured; specify one: dva build <alpha|beta>

$ dva build --
[+] Building ... Image p1-web  Building
ERROR: failed to connect to the docker API ...
```

`--` means "no names follow". It cannot mean *more* than the bare form — that is
the escalation TASK-198 named and TASK-207 ruled on. Here it converts a refusal
into a whole-stack build.

## Measured

The fixture is three files, written out here because the original lived in a
scratch directory that has since been overwritten — a card whose evidence is a
path is a card nobody can re-run:

```yaml
# dva.yml — two plans, NO default_plan (grep -c default_plan → 0)
version: "0.1.44"
stack:
  s1: {description: entry one, default_runner: compose,
       runners: {compose: {files: [docker-compose.yml], project_name: p1}}}
  s2: {description: entry two, default_runner: compose,
       runners: {compose: {files: [docker-compose.yml], project_name: p2}}}
plans:
  alpha: {description: first plan,  entries: [{name: s1, runner: compose, order: 10}]}
  beta:  {description: second plan, entries: [{name: s2, runner: compose, order: 10}]}
```

```yaml
# docker-compose.yml — a real build: context, so entry selection is visible
services:
  web:
    build: {context: ./ctx}
    command: sleep 1
```

```dockerfile
# ctx/Dockerfile
FROM alpine
RUN true
```

Run with `DOCKER_HOST=unix:///nonexistent-dva-review.sock`, so the docker call
fails instantly and the evidence is what was selected before it failed.

| binary | `dva build` | `dva build --` | `dva build -` |
|---|---|---|---|
| `9bf3ee0` (master) | rc=1, `multiple plans configured` | rc=1, **`Image p1-web Building`** then docker API failure | rc=1, `no such service: -` |
| TASK-210 branch | rc=1, `multiple plans configured` | rc=1, **`Image p1-web Building`** then docker API failure | rc=1, `no such service: -` |

Re-measured on 2026-08-20 against a fixture rebuilt from the definition above,
after discovering the original directory had been overwritten. Both binaries,
all six cells, unchanged from the first run.

The `-` column is here as a control and belongs to TASK-215: it also gets past
`requirePlanSelection`, but docker rejects it, so `build` escalates only for the
token docker accepts. Same guard, same line, different downstream luck.

This is pre-existing and not a TASK-210 regression. TASK-210 is
where it became visible: its census found seven callers of the routing helpers,
and measuring the three it had not covered (`build`, `logs`, `status`) turned
this up. `logs` and `status` were unchanged in all four fixtures — cobra strips
the terminator before their `RunE` ever sees it, because they do not set
`DisableFlagParsing`.

## Cause

`parseDvaFlags` keeps the terminator on purpose, and `requirePlanSelection`
returns nil as soon as any token is left:

```go
mode, _, _, _, remaining, err := parseDvaFlags(args)   // remaining == ["--"]
...
if err := requirePlanSelection(c, "build", remaining); err != nil {   // sees a token, returns nil
```

One token is enough to mean "do not ask for a plan", and `--` is a token. That
rule is right for real flags — `dva build --no-cache` must not be answered with
"name a plan" — and wrong for the separator.

**Why build alone.** The other six callers are covered by something build cannot
use:

- `up`, `down`, `stop` — `rejectUnknownFlags` refuses the surviving `--` (measured
  in fixtures A and B: `unknown flag "--"`).
- `restart` — TASK-207 added an explicit re-check that strips the terminator and
  re-runs `requirePlanSelection` when nothing else is left.
- `logs`, `status` — cobra consumes the terminator; it never reaches this code.
- `build` — cannot refuse unknown flags at all. `dva build --no-cache` has to
  reach docker verbatim (TASK-172), so the guard that backstops up/down/stop is
  deliberately absent here. That is the whole reason this one survived.

## What to change

The narrow form is restart's, applied to `remaining`: when every token was the
terminator, the invocation means "no names given", so re-run the selection guard
with an empty list. `dropLeadingTerminator` (TASK-210,
`internal/cli/plan_lifecycle.go`) already exists for the plan-name slot.

Decide first whether the guard should also cover the several-plans case for
`dva build -- <service>`, which today reaches docker as a service name. That
argument predates plans and TASK-172 protects it, so the likely ruling is "only
a lone terminator", but it must be ruled rather than assumed — the fix is one
line either way and the difference is which invocations start refusing.

## Completion Criteria

- [ ] `dva build --` refuses in the several-plans-no-default shape exactly as a bare `dva build` does | verify: human — build the fixture from the three files in ## Measured, run both, and paste rc plus whether any image began building
- [ ] The identity is pinned by a differential test comparing `build --` to a bare `build`, not by an expected string | verify: `grep -c 'func TestBuildLoneTerminatorMeansABareBuild' internal/cli/build_flag_leak_test.go` returns 1 (today: 0). Bound on the test's source rather than on `go test -run`, which exits 0 when it matches nothing, and on a name that does not exist yet rather than on a count of `buildCmd.RunE` — that count is already 5 and would certify itself
- [ ] `dva build -- <service>` and `dva build --no-cache` still reach docker unchanged, whichever way the ruling goes | verify: human — paste both invocations' first line against a config with one plan
- [ ] The ruling for `build -- <service>` is recorded on this card, not left implicit | verify: human
- [ ] `make test` passes | verify: `make test`

## References

- `internal/cli/compose.go:640-668` — `buildCmd`'s prologue, `parseDvaFlags` → `detectPlanRoute` → `requirePlanSelection`
- `internal/cli/compose.go` — `restartCmd`'s terminator re-check, the shape this would copy
- `internal/cli/plan_lifecycle.go` — `dropLeadingTerminator`, `requirePlanSelection`
- `tasks/_archive/210-the-flag-terminator-is-refused-as-a-flag-that-suppresses-the-default-plan.md` — the census that found this, and the ruling it would extend
- `tasks/_archive/207-restart-exits-0-on-an-unknown-service-name-and-the-test-pinning-it-cites-a-deleted-command.md` — the terminator/bare identity
- `tasks/todo/215-a-lone-dash-escapes-up-s-flag-guard-so-dva-up-dash-starts-what-a-bare-up-refuses.md` — the same `requirePlanSelection` line reached by `-` instead of `--`; whichever card lands first should check the other's table still holds
- `tasks/todo/216-the-bare-and-terminator-forms-diverge-for-up-down-and-stop.md` — the terminator ruling for the verbs that take no names; `build` is the exception it does not cover

## Technical Notes

The harness exists. `build_flag_leak_test.go` already drives `buildCmd.RunE`
directly (5 call sites across that file, `native_build_delegation_test.go` and
`provision_note_test.go`), so a terminator case is an addition to a working
pattern rather than new scaffolding.

What is missing is narrower, and worth stating as its own number: of those 5
invocations, **0 pass a `--`**. The commands that forward unknown flags to a tool
are exactly the ones whose terminator handling nobody writes a test for, because
the interesting cases all look like flags. That is the same blind spot this card
describes in the code.
