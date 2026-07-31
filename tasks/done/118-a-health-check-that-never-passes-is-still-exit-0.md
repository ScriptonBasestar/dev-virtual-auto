---
id: TASK-118
title: "An app whose health check never passes exits 0 — decide whether that stays a warning"
type: decision
priority: P3
status: done
effort: S
created-at: 2026-07-31T00:00:00+09:00
decided-at: 2026-07-31T23:16:06+09:00
completed-at: 2026-07-31T23:16:06+09:00
scope: "internal/lifecycle/app_manager.go — the [warn] app %s not ready branch in startWave, the else arm of the crash check"
decision-status: decided
decision: "C — opt-in strict readiness via applications.*.health.required Boolean, default false, application/variant only"
needs-human: false
verification-status: verified
completion-summary: |
  Decision C implemented: applications.<name>.health.required (and variants.*.health.required)
  Boolean, default false. Omitted/false keeps advisory [warn] + exit 0; required:true promotes
  the alive/not-ready arm to [FAIL] + recordErr + non-zero exit. Top-level health_checks does
  not gain required. Wait:false still skips the readiness path entirely.
verification-evidence:
  - "config red→green: TestValidateApplicationHealthRequiredContract 5/5 PASS EXIT=0"
  - "lifecycle red→green: TestStartAppsHealthRequiredContract 5/5 PASS EXIT=0"
  - "full gates: make build + make check-generate + make test -race + make lint EXIT=0"
  - "cli-qa omitted EXIT=0 [warn]; required:false EXIT=0 [warn]; required:true fail EXIT=1 [FAIL]; required:true pass EXIT=0"
---

# Task 118: the one readiness branch TASK-117 deliberately left alone

## What was decided in TASK-117, and what was not

[TASK-117](../done/117-startapps-prints-fail-and-returns-nil.md) made the three `[FAIL]`
branches of `startWave` reach `errors.Join`, so a readiness failure now sets the exit code.
It left one sibling branch untouched:

```go
} else {
	fmt.Fprintf(os.Stderr, "[warn] app %s not ready after %s\n", name, timeout)
}
```

That branch fires when the health check did not pass **and the process is still alive**. It
prints a warning, appends nothing, and the command exits 0.

## Why it was not swept in

The original write-up justified leaving it with "turning it into an error would change
behaviour for anyone relying on `wait: false` semantics." **That reason is wrong** and should
not be reused: the enclosing goroutine returns at `if !opts.Wait { return }` well before this
point, so `wait: false` never reaches it. This branch only runs when the user explicitly
asked DVA to wait for readiness.

The reason that does hold is narrower. DVA cannot distinguish "slow to warm up" from "broken"
from this signal alone — the process is running, it just has not answered the probe yet. And
the genuinely-broken shape has a sharper detector right below it: the port-ownership check,
which now errors.

## The hole that leaves

An app that **binds its port** but **never passes its health check** is caught by neither.
Port ownership is satisfied, so the check below stays quiet; the process is alive, so this
branch only warns. `dva up` exits 0 with a `[warn]` and an application that is up but not
serving.

That is a real gap, not a hypothetical. It is filed here rather than fixed because promoting
it changes the exit code of every existing project with a flaky or slow probe — a product
decision about what `dva up` promises, not a defect to correct unilaterally.

## The decision to make

1. **Keep it a warning.** `dva up` means "started", not "ready". Simple, and no existing setup
   breaks. Leaves the bind-but-never-healthy app reporting success.
2. **Promote it to an error.** `dva up` means "ready", consistent with the fact that the user
   set a timeout and asked to wait. Breaks CI for anyone whose probe is slower than their
   configured `ready_timeout`.
3. **Make it configurable** — e.g. `health.required: true` per application, defaulting to
   today's warning. Preserves both, at the cost of one more knob and a migration story for
   people who would rather have had option 2 by default.

Option 3 is the only one that does not force a choice on existing users, and it is also the
one that adds surface area to a config file the project's own guardrails try to keep small.
That trade-off is the decision.

## Recommendation (recorded for the human decision, not yet confirmed)

**C — configurable, defaulting to today's warning.** Add `health.required: bool` (default
`false`) on the application's health-check config; when `true`, the `[warn] app %s not ready`
branch calls `recordErr` exactly as the three sibling `[FAIL]` branches TASK-117 fixed do, so
the run exits non-zero. When `false`/absent, behaviour is byte-for-byte today.

The reasoning, for the human to confirm or override:

- This branch is reached **only when the user already opted in to waiting** (`if !opts.Wait { return }`
  gates it at `app_manager.go:181-183`). So whoever hits it asked DVA to confirm readiness — that
  is the audience option 2 protects. But option 2 makes the choice for every project at once, and
  a probe slower than `ready_timeout` is a common, non-broken condition. C lets the opt-in that
  already exists at `Wait` extend one level deeper, to per-app strictness, without breaking the
  default.
- It is the only option that does not regress an existing caller. Option 1 keeps the bind-but-
  never-healthy app reporting success — the gap this task exists to name. Option 2 closes the gap
  by changing the default exit code for every project with a slow probe.
- The "one more knob" cost is real but bounded: it sits beside `ready_timeout`, which is already a
  per-app health knob, so it does not open a new category of config so much as complete one.

If the human picks C, the change is: `Required bool` on `HealthCheckConfig` (`config.go:217`),
honoured in the `else` arm at `app_manager.go:218-229`, one test in `app_start_exit_test.go`, and
a USAGE.md line stating `dva up` means "started" unless `health.required: true` opts into "ready".

If the human picks 1 or 2 instead, this recommendation is wrong and the implementation differs —
hence the recorded recommendation rather than code.

## Decision confirmed: C

**Chosen: C — opt-in strict readiness via `applications.*.health.required` Boolean, default `false`, application/variant only.**

Confirmed and implemented. Default remains advisory (exit 0 + `[warn]`); projects that want
`dva app up` to mean "ready" set `health.required: true`. Top-level `health_checks` does **not**
support `required` (scope guard).

### Implementation paths (4 commits)

| Commit | Scope | Files |
| --- | --- | --- |
| `a653ca7` feat(config): add required application health contract | config + schema + contract test | `internal/config/config.go`, `internal/config/schema.json`, `internal/config/health_required_schema_test.go` |
| `7c16ebc` fix(lifecycle): fail required application readiness timeout | lifecycle else-arm + contract test | `internal/lifecycle/app_manager.go`, `internal/lifecycle/app_health_required_test.go` |
| `c5ea2d6` docs(config): document required application readiness | docs + generated library | `USAGE.md`, `skills/config/references/schema-reference.md`, `internal/cli/library_reference.txt` |
| (t5d) docs(tasks): close TASK-118 | this file move only | `tasks/done/118-a-health-check-that-never-passes-is-still-exit-0.md` |

Behaviour in `startWave` alive/not-ready else arm:

- `app.Health.Required == false` (omitted/zero): `[warn] app %s not ready after %s` — exit 0
- `app.Health.Required == true`: `[FAIL] app %s not ready after %s` + `recordErr` — non-zero
- `Wait: false`: readiness path skipped entirely (unchanged)

### Verification evidence (machine-verifiable)

Evidence root: `.omo/evidence/task-118-health-required/`

**Focused tests**

| Suite | Command | Result | Evidence |
| --- | --- | --- | --- |
| config contract | `go test ./internal/config -run '^TestValidateApplicationHealthRequiredContract$' -count=1 -v` | EXIT 0, 5/5 PASS | `task-2-config-green.txt` |
| lifecycle contract | `go test ./internal/lifecycle -run '^TestStartAppsHealthRequiredContract$' -count=1 -v` | EXIT 0, 5/5 PASS | `task-3-lifecycle-green.txt` |

Config subtests: `parent_required_true_loads_and_parses`, `variant_required_true_survives_resolve_app`, `omitted_defaults_false`, `non_boolean_application_value_rejected`, `top_level_health_checks_required_rejected`.

Lifecycle subtests: `omitted_defaults_advisory`, `required_false_advisory`, `required_true_unhealthy_alive_and_port_owned`, `required_true_healthy`, `wait_false_skips_required_health`.

**Full gates** (`task-5-full-gates.txt`, binary commit `c5ea2d6`)

| Gate | Result |
| --- | --- |
| `make build` | EXIT 0 → `./bin/dva` v0.1.44 |
| focused config + lifecycle re-run | EXIT 0 |
| `make check-generate` | EXIT 0 |
| `make test` (`go test -race -cover ./...`) | EXIT 0 (all packages) |
| `make lint` | EXIT 0 (`0 issues`, gofmt clean) |

**CLI QA end-to-end** (`task-5-cli-summary.txt`, binary `./bin/dva` @ `c5ea2d6`)

| Case | EXIT | Label | Evidence |
| --- | --- | --- | --- |
| required omitted | 0 | exactly one `[warn] app sleeper not ready after 1s` | `task-5-cli-default.txt` |
| `required: false` | 0 | exactly one `[warn] app sleeper not ready after 1s` | `task-5-cli-required-false.txt` |
| `required: true` + probe fail | 1 | exactly one `[FAIL] app sleeper not ready after 1s` | `task-5-cli-required-true-fail.txt` |
| `required: true` + probe pass | 0 | no not-ready line | `task-5-cli-required-true-pass.txt` |

Cleanup: `dva app down` after every up (DOWN_EXIT=0); `tmp/task-118-qa/` removed; no leftover `sleep 30` / pid files.

### Commit stats

```
a653ca7  internal/config/config.go (+1), health_required_schema_test.go (+182), schema.json (+14/-2)
7c16ebc  internal/lifecycle/app_health_required_test.go (+206), app_manager.go (+18/-10)
c5ea2d6  USAGE.md (+2), library_reference.txt (+3/-1), schema-reference.md (+3/-1)
```

## Acceptance criteria

- [x] The option is chosen and recorded here with its reasoning | verify: this file names **C** under "Decision confirmed: C"
- [x] If 2 or 3: the behaviour change is implemented and USAGE.md says what `dva up` / `dva app up` promises | verify: commits `a653ca7` `7c16ebc` `c5ea2d6`; USAGE.md contract prose; focused tests + cli-qa EXIT codes above
- [x] If 1: N/A (option 1 not chosen) | verify: decision is C, not 1

## Related

- [TASK-117](../done/117-startapps-prints-fail-and-returns-nil.md) — fixed the three sibling
  branches; this is the one it left, and the comment in the code points here.
- [TASK-113](../done/113-up-and-app-commands-swallow-unknown-flags.md) — the same recurring
  shape: DVA reaches the right conclusion and then does not let it reach the exit code.
