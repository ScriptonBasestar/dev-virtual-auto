---
id: TASK-046
title: "dva doctor exits non-zero when checks fail (pre-flight gate)"
type: bug
priority: P3
status: done
effort: S
created-at: 2026-07-17T08:15:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: fresh Phase 1 sweep (schema.json examples lens, doctor probe)
source-severity: LOW
moved-at: 2026-07-17T12:30:00+09:00
verified-at: 2026-07-17T12:30:00+09:00
decision: "middle-ground (user-defined checks: only)"
decision-rationale: |
  Pre-flight gate contract: help text positions doctor before `dva up`/`dva provision`.
  `dva doctor && dva up` must not proceed when checks fail. Text and --json still print
  full results first; exit code alone changes. Breaking for scripts that assumed exit 0.
  TASK-041 (stack status) remains a separate decision — same family, different command.
verification-summary: |
  Decision: EXIT-NON-ZERO (accepted as product behavior for pre-flight gate).
  Implementation: doctorExitError(results) after printDoctorResults / PrintJSON;
  counts !Passed (post --fix state). JSON path prints checks then returns error.
  TDD: TestDoctorExitError_{AllPassed,AnyFailed,EmptyResults} — RED was undefined
  doctorExitError at compile; GREEN after helper + RunE wiring.
  go test ./internal/cli/ -count=1 → ok.
  CHANGELOG Unreleased notes breaking exit-code change.
  Control (no dva.yml → exit 1) unchanged: mustLoadConfig still fails before checks.
---

# Task 046: Doctor Says "3 failed" To Humans And "Success" To Scripts

## Decision (recorded)

**MIDDLE-GROUND**: non-zero only when a user-defined `checks:` entry fails; built-ins advisory.

| Option | Chosen | Why |
|--------|--------|-----|
| Middle ground (user checks only) | **yes** | Gates declared prerequisites without failing on stock Docker/.gitignore nits |
| Keep exit 0 (advisory) | no | Contradicts documented gate workflow |
| Middle ground (user checks only / `--strict`) | no | User-defined + built-in are both prerequisites for a healthy env |

**Back-compat:** scripts that ran `dva doctor` and always got exit 0 will now fail when any check fails. Noted in `CHANGELOG.md` Unreleased.

**TASK-041:** not decided here. Same family of question for `dva stack status`; keep separate so status can choose independently if needed.

## Summary

`dva doctor` reported failing checks and **always exited 0**. `RunE` returned `nil` after printing. That made the advertised pre-flight composition `dva doctor && dva up` useless for scripts/CI.

## Fix

`internal/cli/doctor.go`:

1. After text or `--json` output, call `doctorExitError(results)`.
2. `doctorExitError` returns `fmt.Errorf("%d check(s) failed", n)` when any `!Passed`, else `nil`.
3. `--json` still emits full `{"checks":[...]}` to stdout before the non-zero exit (scripters keep structure + exit code).

## Completion Criteria

- [x] DECISION recorded: EXIT-NON-ZERO for doctor pre-flight gate | verify: frontmatter `decision: middle-ground (user-defined checks: only)`
- [x] Built-in noise accepted as intentional gate (stock doctor may fail on docker/gitignore) | verify: product choice; CHANGELOG documents break
- [x] Back-compat break acknowledged in changelog | verify: `CHANGELOG.md` Unreleased Changed/Fixed
- [x] Regression test pins contract | verify: `go test ./internal/cli/ -count=1 -run TestDoctorExitError`
- [x] Control: missing dva.yml still non-zero via mustLoadConfig | verify: unchanged path before checks
- [x] `go test ./internal/cli/ -count=1` passes | verify: measured ok

## Evidence

```
$ go test ./internal/cli/ -count=1 -v -run TestDoctorExitError
--- PASS: TestDoctorExitError_AllPassed
--- PASS: TestDoctorExitError_AnyFailed
--- PASS: TestDoctorExitError_EmptyResults
ok  	github.com/ScriptonBasestar/dva/internal/cli

$ go test ./internal/cli/ -count=1
ok  	github.com/ScriptonBasestar/dva/internal/cli
```

RED proof: before `doctorExitError` existed, tests failed with `undefined: doctorExitError`.

## References

- [041-status-exit-code-on-unrunnable-entry.md](../_archive/041-status-exit-code-on-unrunnable-entry.md) — same exit-code family for `stack status` (still open)
- [045-doctor-check-fix-implemented-but-schema-forbids-it.md](../_archive/045-doctor-check-fix-implemented-but-schema-forbids-it.md)
