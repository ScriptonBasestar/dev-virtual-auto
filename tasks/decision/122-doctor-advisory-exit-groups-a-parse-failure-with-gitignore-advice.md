---
id: TASK-122
title: "`dva doctor` exits 0 when the compose files do not parse, because every built-in check is advisory"
type: decision
priority: P3
effort: S
status: decision
created-at: 2026-07-31T00:00:00+09:00
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

- [ ] The grouping is confirmed against the current check list | verify: `grep -n 'results = append(results' internal/cli/doctor.go` — enumerate every built-in and say which are diagnoses and which are advice
- [ ] A direction is chosen | verify: `human — A, B, or C, recorded here with the reason`
- [ ] If A, the trap is documented where a reader meets it | verify: `human — doctor's --help or USAGE.md must say built-in failures do not affect the exit code`
- [ ] If B or C, the advisory test is updated rather than deleted | verify: `go test ./internal/cli/ -run 'DoctorExitError' -v` — `TestDoctorExitError_BuiltinFailedOnly_Advisory` encodes the current contract and must be rewritten to encode the new one

## Related

- [TASK-119](../done/119-doctor-compose-check-ignores-the-configured-command.md) — where this was
  measured. That task made `Compose config resolves` report on the right binary; it did not make
  anyone notice when it fails.
