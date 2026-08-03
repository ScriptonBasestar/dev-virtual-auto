---
id: TASK-159
title: "doctor --strict exists for CI and appears in no user-facing doc"
type: docs
priority: P3
status: todo
effort: S
created-at: 2026-08-03T15:00:00+09:00
source: "TASK-122 finalize verification — the flag the fix added, undocumented"
depends-on: [TASK-122]
scope: "dva repo — USAGE.md:330-336, CHANGELOG.md"
---

# Task 159: Document doctor --strict where its audience looks

## Problem

`dva doctor --strict` makes every failing check count toward the exit code, instead of the
default where built-ins are advisory. Its entire purpose is CI adoption. It appears in `--help`
and the manifest and nowhere else:

```
$ grep -n 'dva doctor' USAGE.md
320: | `dva doctor` | 환경 사전조건 및 설정 문제 진단 (`--fix` 자동 수정) |
333: dva doctor                # 환경 사전조건 체크
334: dva doctor --fix          # 수정 가능한 문제 자동 해결
335: dva doctor --json         # JSON 출력
```

Three flags listed, `--strict` absent. `CHANGELOG.md`'s Unreleased section documents the
advisory-exit contract (TASK-046) with no `--strict` entry.

The repo's own norm is the other way: `USAGE.md:352` documents the sibling
`dva config validate --strict`. So a reader who knows one flag exists has no reason to think the
other does.

## Why it matters beyond a missing line

The advisory default is the thing a CI author most needs to know about, because it is the reason
`dva doctor` in a pipeline passes while reporting failures. `--strict` is the answer to that, and
it is reachable only by someone who already suspected the problem and ran `--help`.

## Acceptance criteria

- [ ] `USAGE.md`'s doctor section lists `--strict` alongside `--fix` and `--json`, and states the
      default it changes — built-in checks are advisory, so exit 0 does not mean every check
      passed.
- [ ] `CHANGELOG.md` records the flag under the same entry as the advisory-exit contract it
      completes.
- [ ] The wording matches the sibling at `USAGE.md:352` in form, so the two read as one
      convention.
- [ ] Check the remaining commands for flags present in `--help` and absent from `USAGE.md`, and
      print the count found. A bare "none" is not a result with the denominator unstated.
- [ ] `make doc-check` exits 0 with a non-zero `links_checked`.

## Notes

Related but distinct:
[TASK-151](151-the-manifest-never-mentions-json-the-flag-its-audience-needs-most.md) — the
manifest omits the three root persistent flags including `--json`. Same failure shape (a flag its
audience cannot discover) on a different surface;
[TASK-149](149-default-args-inheritance-is-documented-only-in-the-schema.md) is the third.
Whoever fixes these should consider whether the sweep is one job.
