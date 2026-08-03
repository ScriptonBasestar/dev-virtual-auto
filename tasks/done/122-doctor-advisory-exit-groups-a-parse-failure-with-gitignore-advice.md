---
id: TASK-122
title: "`dva doctor` exits 0 when the compose files do not parse, because every built-in check is advisory"
type: decision
priority: P3
effort: S
status: done
created-at: 2026-07-31T00:00:00+09:00
decided-at: 2026-08-01T00:00:00+09:00
completed-at: 2026-08-01T00:00:00+09:00
decision: C
scope: "internal/cli/doctor.go:658 doctorExitError"
---

# Task 122: one bucket for two kinds of built-in check

`doctorExitError` counts only user-defined failures:

```go
for _, r := range results {
	if r.UserDefined && !r.Passed {
		failed++
	}
}
```

`UserDefined` is set in exactly one place (`doctor.go:99`) — the loop over `c.DoctorChecks`. Every
built-in check is therefore advisory: it can print `[FAIL]` and the process still exits 0.

This is deliberate. `TestDoctorExitError_BuiltinFailedOnly_Advisory` pins it and names the intent.
This task does not propose reversing it.

## The narrower question

The advisory rule treats all built-in checks as one kind of thing, and they are not.

| check | what a `[FAIL]` means |
|---|---|
| `.sb/dva/ is ignored in .gitignore` | housekeeping advice; the environment works |
| `Docker daemon accessible` | true on any laptop with Docker Desktop closed; transient |
| `Compose file exists: X` | a path in `dva.yml` names nothing |
| `Compose config resolves` | the configured compose tool **rejected the files** |

The last two are not advice. `Compose config resolves` failing means the tool the user actually runs
was handed their files and refused them — the same refusal `dva up` is about to hit. Reporting that
as `[FAIL]` and exiting 0 means `dva doctor && dva up` proceeds into the failure doctor just
diagnosed.

Measured (TASK-119's fixture, before that fix, both binaries as PATH shims):

```
  [FAIL] Compose file exists: compose.${STAGE}.yml
  [FAIL] .sb/dva/ is ignored in .gitignore
  3 passed, 2 failed
$ echo $?
0
```

Two failures on screen, one of them about a file the config depends on, and an exit code that says
everything is fine.

## Why it is a decision and not a fix

Any change here changes doctor's contract with whatever runs it. Three shapes, none obviously right:

- **A: leave it.** `dva doctor` stays a report; anything wanting an exit code writes a user check.
  Costs nothing, and keeps the trap.
- **B: a severity field on `DoctorResult`.** Built-ins opt into counting. Honest, but it means
  deciding severity for every existing check, and `Docker daemon accessible` is the hard one — it
  fails constantly for reasons that are not the config's fault.
- **C: a flag (`--strict`, or `--exit-on-builtin`).** No existing caller changes behaviour; CI opts
  in. Cheapest to land, and pushes the severity question onto the user instead of answering it.

Not a recommendation between B and C without knowing whether anything runs `dva doctor` in CI today.

## Acceptance criteria

- [x] The grouping is confirmed against the current check list | verify: `grep -n 'results = append(results' internal/cli/doctor.go` — enumerate every built-in and say which are diagnoses and which are advice
- [x] A direction is chosen | verify: `human — C chosen; see rationale below`
- [N/A] If A, the trap is documented where a reader meets it | verify: `N/A — direction C chosen; A criterion does not apply`
- [x] If B or C, the advisory test is preserved | verify: `go test ./internal/cli/ -run 'DoctorExitError' -v` — B would rewrite/add severity behavior; C preserves advisory default and adds strict-side tests (test kept as-is)

## Progress (landed while the direction stays open)

The grouping (criterion 1) is done — an agent pass classified every built-in check:

- **Advice** (transient/environmental): Docker daemon accessible; Docker socket permissions; `.sb/dva/` gitignored; compose project-name alignment (nit); app port ownership (runtime orphan).
- **Diagnosis** (config names a missing path, or the tool rejected the files): compose file exists; env file exists; stack kubeconfig exists; compose config resolves. Borderline: subproject project-name collision (runs but silently reaps the parent stack on `dva down`).

CI runs no `dva doctor` today (`.github/workflows/ci.yml` has zero references; `Makefile` has no doctor target). The only in-repo callers are agent-mesh flows and all swallow the exit with `|| echo`.

**C's implementation landed as default-off** (criterion 4 satisfied by preservation, not rewrite):
`doctorExitError(results, strict)` counts every failing check under `--strict`, the advisory test is kept unchanged at `false`, and two new tests pin the strict side. `make test` and `make lint` are green; a mutant that drops the `|| strict` branch is killed by `TestDoctorExitError_StrictCountsBuiltins`.

**Decision finalized: C** (direction criterion satisfied).

Rationale: `--strict` means every reported failure counts toward the exit code; the explicit opt-in preserves compatibility for existing callers and scripts. Narrowing the set of counted checks would require severity metadata on results and therefore becomes option B. Transient failures (Docker daemon, socket permissions) are acceptable under explicit `--strict` because the flag is a deliberate CI opt-in; the default remains advisory.

Historical progress preserved above; implementation already green under focused `DoctorExitError` tests.

## Related

- [TASK-119](../_archive/119-doctor-compose-check-ignores-the-configured-command.md) — where this was
  measured. That task made `Compose config resolves` report on the right binary; it did not make
  anyone notice when it fails.
