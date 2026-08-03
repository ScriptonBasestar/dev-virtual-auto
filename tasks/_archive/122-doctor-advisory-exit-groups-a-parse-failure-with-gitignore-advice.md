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
verified-at: 2026-08-03T15:00:00+09:00
archived-at: 2026-08-03T15:00:00+09:00
verification-summary: |
  doctorExitError is now `func doctorExitError(results []DoctorResult, strict bool)` (doctor.go:681) with
  `if (r.UserDefined || strict) && !r.Passed`; both call sites (doctor.go:71 JSON, :75 text) pass doctorStrict.
  Measured with ./bin/dva (0.1.44), $? read directly, fixtures under scratchpad/v122:
  - built-in-only failure (missing compose file): default → "2 passed, 3 failed", EXIT=0; --strict → EXIT=1,
    "ERROR: 3 check(s) failed (--strict)". Same pair on the --json path: EXIT=0 / EXIT=1.
  - user check failing: default EXIT=1 "1 user check(s) failed"; --strict EXIT=1 "2 check(s) failed (--strict)".
  - all-pass fixture: --strict EXIT=0 (flag invents no failures).
  - --strict --fix on a fixable-only failure: EXIT=0, "3 passed, 0 failed (1 auto-fixed)" (applyDoctorFixes
    sets Passed=true, so fixes are not double-counted).
  Flag is discoverable: `dva doctor --help` lists --strict; `dva manifest` line 49 carries its help string.
  Tests are non-vacuous — the strict test asserts both the count "3 check" and the literal "--strict".
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

⚠️ The inventory is one short: `devcontainer.json exists` (`internal/cli/doctor.go:120-127`) is a
built-in check — `UserDefined` stays false — and belongs in the diagnosis group by the same test
(the config declares a devcontainer, the file is missing). Verified live that it counts under
`--strict`. Direction C is unaffected, since under C every built-in counts uniformly; only this
classification table was incomplete.

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
- [TASK-159](../todo/159-doctor-strict-is-the-ci-flag-nobody-can-find.md) — ⚠️ `--strict`, the
  flag this task added for CI, is in `--help` and the manifest and in no user-facing doc.
  `USAGE.md:333-335` lists `--fix` and `--json` only, while the sibling
  `dva config validate --strict` is documented at `USAGE.md:352`.
