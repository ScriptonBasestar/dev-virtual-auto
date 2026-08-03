---
id: TASK-139
title: "A failing doctor row reads as a pass, because the row is named after the check rather than the finding"
type: bug
priority: P3
status: todo
effort: S
created-at: 2026-08-03T12:30:00+09:00
source: "TASK-080 finalize verification — its own Left open items, untracked"
depends-on: [TASK-080]
scope: "dva repo — internal/cli/doctor.go"
---

# Task 139: Name doctor rows after the finding, not the check

## Problem

`internal/cli/doctor.go:199`:

```go
r := DoctorResult{Name: fmt.Sprintf("%s/ is ignored in .gitignore", config.DotDirName)}
```

The row is named with the *assertion the check makes*, so when the assertion is false the
output reads as though it were true:

```
[FAIL] .sb/dva/ is ignored in .gitignore
```

A reader scanning for what is wrong sees a sentence stating the desired state. The status
tag is the only thing carrying the negation, and it is the part most easily lost — in a
grep, in a copied line, in a summary.

TASK-080 recorded this under "Left open"; a sweep of `tasks/` on 2026-08-03 found nothing
tracking it.

## Second item, same task's Left open

With the marker files present and unignored, `dva doctor` reports the same finding twice
on the human path: once as the stderr banner (the pre-command warning TASK-080 gated) and
once as its own `[FAIL]` row. Pre-existing rather than introduced by TASK-080, and absent
under `--json`.

Decide whether the banner should be suppressed when `doctor` is the command being run —
`doctor`'s whole job is to report that class of finding, so the banner adds nothing there.

## Acceptance criteria

- [ ] A failing row states the finding (`.sb/dva/ is NOT in .gitignore`, or a name plus a
      separate message field) — reading the row alone cannot suggest the opposite of what
      happened.
- [ ] A passing row still reads correctly; the fix does not just invert the sentence.
- [ ] `dva doctor --json` keys are unchanged, or the change is deliberate and noted —
      the JSON surface is consumed by agents.
- [ ] `dva doctor` reports the gitignore finding once, not twice, on the human path.
- [ ] Tests cover both the pass and fail rendering, so the wording cannot drift back.
- [ ] `make test` exits 0.

## Notes

Check the other `DoctorResult{Name: ...}` constructors in the same file before fixing one
— if the name-the-assertion pattern is used throughout, the fix is a convention, not a
one-liner, and the convention is the deliverable.
