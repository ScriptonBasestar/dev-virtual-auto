---
id: TASK-139
title: "A failing doctor row reads as a pass, because the row is named after the check rather than the finding"
type: bug
priority: P3
effort: S
created-at: 2026-08-03T12:30:00+09:00
source: "TASK-080 finalize verification — its own Left open items, untracked"
depends-on: [TASK-080]
scope: "dva repo — internal/cli/doctor.go"
status: done
quality-review: pass
quality-reviewed-at: 2026-08-07T18:05:08+09:00
verified-at: 2026-08-07T18:05:08+09:00
archived-at: 2026-08-07T18:05:08+09:00
quality-review-evidence: |
  - kind: test
    command-or-step: make test && make doc-check (mise go 1.26.4)
    result: exit 0; shared suite green
  - kind: recheck
    command-or-step: acceptance criteria re-observed
    result: DoctorFailRow/Finding tests PASS
verification-summary: |
  quality-review pass; re-checked deliverables. DoctorFailRow/Finding tests PASS. Shared: make test OK; make doc-check OK (mise go 1.26.4).
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

**Decided: suppressed, for `doctor` only.** The reasoning was already written down in
`checkGitignoreForWarning`'s own comment, which describes the banner as the version "worth
putting ahead of *another* command's answer" — doctor is not another command, it is the
answer. Two reports of one problem read as two problems, which is the opposite of what a
diagnostic owes its reader. Every other command keeps the banner, because there it is the
only warning there is.

## Acceptance criteria

- [x] A failing row states the finding (`.sb/dva/ is NOT in .gitignore`, or a name plus a
      separate message field) — reading the row alone cannot suggest the opposite of what
      happened | verify: `go test ./internal/cli/ -run TestDoctorFailRow_CannotStateTheOppositeOfWhatHappened`
- [x] A passing row still reads correctly; the fix does not just invert the sentence | verify: `go test ./internal/cli/ -run TestPrintDoctorResults_FailRowPrintsFindingAndPassRowPrintsName`
- [x] `dva doctor --json` keys are unchanged, or the change is deliberate and noted —
      the JSON surface is consumed by agents | verify: `go test ./internal/cli/ -run TestDoctorResultJSON_KeysUnchangedAndFindingOmittedWhenEmpty`
- [x] `dva doctor` reports the gitignore finding once, not twice, on the human path | verify: `go test ./internal/cli/ -run TestGitignoreWarningSuppressedFor`
- [x] Tests cover both the pass and fail rendering, so the wording cannot drift back | verify: `go test ./internal/cli/ -run 'TestDoctorFailRow|TestDoctorCheckNameIsStableAcrossOutcomes|TestPrintDoctorResults_FailRow'`
- [x] `make test` exits 0 | verify: `make test`

## Notes

Check the other `DoctorResult{Name: ...}` constructors in the same file before fixing one
— if the name-the-assertion pattern is used throughout, the fix is a convention, not a
one-liner, and the convention is the deliverable.

## Result

The Notes were right: `doctor.go` already used **both** conventions, so the deliverable was
picking one and applying it. Eleven rows named the assertion (`Docker daemon accessible`,
`Compose file exists: X`), two named the finding (`App %q port %d held by a process dva did
not start`). Neither can do both jobs, so `DoctorResult` now has two fields:

- **`Name`** — the check's stable identity. Reads the same on both outcomes, which is what
  lets a `--json` consumer correlate runs and what makes `[pass]` rows read as English. This
  forces it into assertion shape, and that is precisely why it cannot be the failure line.
- **`Finding`** — what was observed, phrased so it cannot read as its own opposite. Set only
  on rows that can fail, rendered only after `[FAIL]`, `omitempty` in JSON.

`failureLine()` prints `Finding` when set and falls back to `Name`, which is what keeps the
two already-finding-shaped rows unchanged — they never render on the pass path, so their
name *is* the observation, and demanding a `Finding` of them would be duplication.

### A second defect, invisible from the human output

`checkComposeProjectNameAlignment` returned the observation **as its `Name`** when it failed
and `"Compose project name alignment"` when it passed. One check, two `"name"` values in
`--json`, so a consumer keying on the name could not match a failing run to a passing one.
Moving the observation to `Finding` fixes that with the human line byte-identical. Found by
reading every constructor rather than only the one the task named.

### Evidence

Measured on `bin/dva` against a fixture with `.git/`, `.sb/dva/`, an unignoring `.gitignore`
and a missing compose file:

```
  [pass] Docker daemon accessible
  [FAIL] Compose file is MISSING: missing-compose.yml
  [FAIL] compose config does NOT resolve (docker compose config exited non-zero)
  [FAIL] .sb/dva/ is NOT ignored in .gitignore
```

The banner, per command — the duplicate is gone from `doctor` and nowhere else:

| command | `is not in your .gitignore` banners |
| --- | --- |
| `dva doctor` | **0** |
| `dva ls` | 1 |
| `dva validate` | 1 |
| `dva show` | 1 |

`--json` on the same failing row, showing `name` unchanged and `finding` additive:

```json
{"name": ".sb/dva/ is ignored in .gitignore",
 "finding": ".sb/dva/ is NOT ignored in .gitignore",
 "passed": false, "fix_hint": "Add '.sb/dva/' to .gitignore…", "fixable": true}
```

### Falsification

The existing suite passed before *and* after the change, so it could not have caught the
wording drifting back — that is what criterion 5 was about. Three breaks, three mechanisms:

| # | Break | Result |
| - | ----- | ------ |
| F1 | `printDoctorResults` prints `r.Name` again instead of `r.failureLine()` | FAIL — `TestPrintDoctorResults_FailRowPrintsFindingAndPassRowPrintsName`, both assertions |
| F2 | Drop the `Finding` assignment in `checkGitignoreStatus` | FAIL — the paired guard, printing the original defect verbatim: `[FAIL] .sb/dva/ is ignored in .gitignore` |
| F3 | Restore `Name: msg` on the alignment failure path | FAIL — `TestDoctorCheckNameIsStableAcrossOutcomes`: `pass "Compose project name alignment" vs fail "Compose file compose.yml has missing project name"` |

The guard is built to resist a cosmetic fix: it drives each check down **both** branches
through the real functions and asserts the failing line differs from the *passing row's
name*. Rewording `Name` alone, or setting `Finding` to a copy of `Name`, fails it.

### Design notes

- **Findings are derived from the check's inputs, never from `Name`.** `runSingleCheck` is
  shared by user-defined checks from `dva.yml`, whose names are whatever the author wrote —
  so a user row gets `no file at .devcontainer/devcontainer.json`, not a negation of a
  sentence we do not control. The built-in devcontainer check routes through here too.
- **`Finding` is cleared on success and after `--fix`.** A repaired row would otherwise ship
  `passed: true` beside a finding saying the opposite, which is the contradiction the field
  exists to remove.
- **Two rows keep finding-shaped names**: the per-app port-ownership row and the
  per-subproject compose-name row. They are emitted one per offending item and only on
  failure, so there is no passing counterpart to correlate with and no inversion to fix.
- **The banner suppression is a predicate, not a flag.** `gitignoreWarningSuppressedFor`
  takes the command name that `PersistentPreRun` recorded, so the rule is stated once and
  testable without running a command. `doctor` is the only suppressed name: it is the only
  command that reports this class of finding itself.

### Gates

```
make test     ok — cli 68.6% → 69.2% · config 68.3% · doccheck 83.3% · output 100%
gofmt -l      0 files
go vet ./...  exit 0
```

### Changed

- `internal/cli/doctor.go` — `Finding` field, `failureLine()`, findings on every
  assertion-shaped check, `Name` stabilised on the alignment check, `Finding` cleared on fix.
- `internal/cli/gitignore.go` — `gitignoreWarningSuppressedFor`, gating the banner.
- `internal/cli/root.go` — `runningCommand`, recorded in `PersistentPreRun`.
- `internal/cli/doctor_finding_test.go` — new: the both-branches guard over 5 checks, name
  stability, render pass/fail/fallback, per-type findings, fix clearing, JSON shape,
  suppression predicate.
