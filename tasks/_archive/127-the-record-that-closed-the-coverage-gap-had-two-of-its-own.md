---
id: TASK-127
title: "The record that closed the lint coverage gap overstated two of its own claims, and one unnamed exclusion was hiding a real defect"
type: fix
priority: P2
effort: S
status: done
created-at: 2026-08-02T12:00:00+09:00
resolved-at: 2026-08-02T12:00:00+09:00
resolution: "Removed all four exclusion presets after measuring what each actually suppressed, fixed the unchecked f.Close() before a script is chmod'ed and executed, fixed the one live hint-severity modernize finding, and corrected the two TASK-126 claims that reached past their survey"
scope: ".golangci.yml exclusions.presets — four unnamed entries, one suppressing 6 errcheck findings; internal/exec/exec.go — unchecked Close on a temp script that is then executed"
verified-at: 2026-08-03T15:45:00+09:00
archived-at: 2026-08-03T15:45:00+09:00
verification-summary: |
  Pinned tooling: golangci-lint 2.12.2 (go1.26.2), gopls v0.22.0, go1.26.4 darwin/arm64.
  Repo HEAD moved 881f2e0 -> e8a4ba2 -> 1695f9d during the run (concurrent archiving);
  `git diff 881f2e0 e8a4ba2 -- '*.go' .golangci.yml Makefile` is empty, so no code
  measurement is affected. All work done on `git archive HEAD` copies in scratchpad.
  Every "0" is backed by a control on the same binaries: golangci-lint 0 vs 6/2/1 under
  reverted call sites; gopls hint 0 vs 1 under reverted strings.Cut; doc-check 0 vs 1
  under a reverted link; check-generate clean vs stale under a tampered source.
  The record's per-preset attribution reproduces exactly (std-error-handling 6, other
  three 0 each and 0 combined), as do both mutation-table line numbers (exec.go:183,
  compose_error_test.go:38). `golangci-lint linters` still reports 107 disabled by
  configuration, matching the residual-gap count. writeNote's `w` is os.Stdout
  (provision.go:152) or a bytes.Buffer (provision.go:271), as claimed. The criterion's
  `tasks/done/126-*.md` binding is a preserved historical path; TASK-126 is now under
  `tasks/_archive/`, so the claim remains valid but the original shell path no longer runs.
---

# Task 127: the audit that TASK-126 asked for, applied to TASK-126

[TASK-126](../_archive/126-the-lint-gate-still-hides-analyzers-task-111-said-it-had-none-left.md) closed by
falsifying TASK-111's closing claim. It was then audited under its own standard, and two of its
claims reach past what it measured. One of them was hiding a real defect.

This is the third link in the same chain (111 → 126 → 127), which is the finding as much as
anything in the table below: the failure mode is not any one gate, it is that *writing down that a
gate is now complete* is itself an unverified claim, and it keeps being made.

## Measured

golangci-lint 2.12.2 / gopls v0.22.0, at `1d9de9c`:

| claim in TASK-126 | measured | verdict |
|---|---|---|
| 8-findings table (govet 1, unparam 4, gopls 3) | reproduces exactly on the parent tree | **holds** |
| `make lint` and `gopls` agree at 0 | agree at *default* severity; `-severity=hint` → **1** | **overstated** |
| only `fieldalignment` and `shadow` stay excluded | **107** linters disabled by config, and `exclusions.presets` held 4 unnamed entries | **overstated** |
| `make check-generate`, `make doc-check` pass | both exit 0 | **holds** |

The numbers TASK-126 reported were honest. What it got wrong was the scope of its own survey —
twice, in the two criteria that assert completeness rather than a count.

## The one that mattered

Removing `exclusions.presets` surfaced 6 errcheck findings. Attributing them one preset at a time:

| preset | inline reason | findings it suppressed |
|---|---|---|
| `std-error-handling` | none | **6** |
| `comments` | none | 0 |
| `common-false-positives` | none | 0 |
| `legacy` | yes — the only one with a comment | **0** |

Two separate problems there. `legacy` carried a comment claiming it "keeps the default exclusions
close to golangci-lint v1 so the migration does not surface a new wave of style-only findings" — an
effect it does not have, since it suppresses nothing. A stated reason that isn't doing the work it
claims is the same defect as an unstated one, and harder to notice.

And `std-error-handling` was hiding this, in `internal/exec/exec.go`:

```go
	if _, err := f.WriteString(script); err != nil {
		f.Close()
		return fmt.Errorf("writing temp script: %w", err)
	}
	f.Close()                       // ← unchecked

	if err := os.Chmod(f.Name(), 0700); err != nil { ... }
	return ExecSubprocess(env, f.Name(), nil, false)   // ← then executed
```

`ExecScriptInline` writes a user's script to a temp file, closes it, chmods it, and runs it. A
buffered write can surface a short write or ENOSPC **only at Close**. Dropping that error means
`dva` executes a truncated script and reports success — and a shell script that stops early is not a
script that failed, it is a different script. This is the one finding in the six that can corrupt a
run rather than merely annoy a reader.

The other five are genuinely benign — a `defer os.Remove` cleanup and three `fmt.Fprintln` calls to
either stdout or a `bytes.Buffer`. They are now dropped with an explicit `_ =` at the call site
rather than by a config-level preset, which puts the reason next to the code instead of in a file
that decides it elsewhere.

## Why presets were removed rather than annotated

A preset is an exclusion whose *contents are chosen by the linter, not by this repo*, and they
change when the linter updates. So a preset cannot carry a reason that stays true — which is what
`legacy`'s stale comment demonstrates. Three of the four already suppressed nothing; the fourth
suppressed one real defect and five call sites better handled locally. Nothing was left for the
mechanism to do.

## Acceptance criteria

- [x] The unchecked Close is checked | verify: `human — read internal/exec/exec.go ExecScriptInline` — **`f.Close()` error returned as `closing temp script: %w`, with the reason it is not deferred recorded inline**
- [x] The gate fails on either reverted error check | verify: `human — restore f.Close() and fmt.Fprintln(w), run make lint` — **`2 issues: * errcheck: 2`, `make: *** [lint] Error 1`**
- [x] No exclusion is unnamed | verify: `.golangci.yml` has no `presets:` key; every remaining `rules:` entry carries a reason, and the 6 previously-suppressed call sites carry `_ =` with a comment

  ⚠️ Four of the six do. `internal/exec/exec.go:166`
  (`defer func() { _ = os.Remove(f.Name()) }()`) and `:175` (`_ = f.Close()` on the WriteString
  error path) carry a bare `_ =` and no reason — at those two the exclusion moved from the config
  to the call site and stayed unnamed, which is the half of this task's own point that did not
  land. The prose above at lines 73-74 has the matching slip: it enumerates four of the five
  benign sites and omits `exec.go:175`, and one of the three `fmt.Fprintln` calls it names is a
  `Fprintf` (`provision.go:133`). Tracked as
  [TASK-161](../todo/161-two-of-the-six-relocated-exclusions-are-still-unnamed.md).
- [x] Each preset's real effect was measured, not assumed | verify: `golangci-lint run -c <probe>` per preset — **`std-error-handling` alone → 6; `comments`+`common-false-positives`+`legacy` together → 0**
- [x] The gate is unchanged for everything else | verify: `make lint` — **`0 issues.`, 235 files gofmt-clean**
- [x] The live hint-severity finding is fixed | verify: `gopls check -severity=hint $(/usr/bin/find cmd internal tools -name '*.go')` — **0; was 1 (`stringscut`, `internal/lifecycle/compose_error_test.go:38`)**
- [x] The residual-gap command can actually see the gap | verify: `human — revert strings.Cut to strings.SplitN, run all three commands` — **`make lint` → `0 issues.`, `gopls check` → 0, `gopls check -severity=hint` → 1. Only the third sees it.**
- [x] TASK-126's two overstated criteria are marked, not silently rewritten | verify: `tasks/done/126-*.md` — **both criteria marked `[~]` with ⚠️ and the correction stated inline**
- [x] Full suite passes | verify: `make test` — exit 0
- [x] Docs consistent | verify: `make doc-check` — `broken_links: 0`
- [x] Generated artifacts current | verify: `make check-generate` — clean

## Mutation-tested

| Mutation | Result |
|---|---|
| Restore unchecked `f.Close()` in `ExecScriptInline` | `make lint` → `errcheck: 1` at `exec.go:183`, exit 1 |
| Restore unchecked `fmt.Fprintln(w)` in `writeNote` | same run → `errcheck: 2` total, exit 1 |
| Revert `strings.Cut` → `strings.SplitN` | `make lint` **still `0 issues.`**, `gopls check` **still 0**, `gopls check -severity=hint` → 1 |

The third mutation is the one worth keeping. It shows the gate and the record's own prescribed
remedy are *both* blind to that class, which is why the remedy command now carries `-severity=hint`.
The unchecked-Close fix has no unit test and does not get a criterion claiming one: forcing a real
`Close` failure needs a filesystem seam this package does not have, so errcheck *is* the regression
test, and that is stated rather than papered over.

## The residual gap, restated

- **107 linters remain disabled by configuration.** Almost all are style (`wsl`, `varnamelen`,
  `nlreturn`) and enabling them is not proposed. The honest statement is the count, not "nothing is
  excluded" — TASK-126's criterion asserted the latter on a govet-only survey.
- **gopls's modernize suite still exceeds golangci-lint 2.12.2's**, and the divergence is *inside*
  same-named analyzers: vendored `stringscut` catches the `strings.Index` form but not `SplitN`.
  Only `gopls check -severity=hint` sees the difference, and it is not wired into CI.

  ⚠️ It is now. [TASK-130](130-the-lint-gate-is-a-strict-subset-of-what-an-editor-sees.md)
  wired it into both `make lint` (`Makefile:60-77`, hard-failing when gopls is absent) and CI
  (`.github/workflows/ci.yml:62-82`). TASK-130 anticipated updating this paragraph only under its
  option D; option B landed, which closed the gap and left the sentence describing the old state.

## Related

- [TASK-126](../_archive/126-the-lint-gate-still-hides-analyzers-task-111-said-it-had-none-left.md) — the record
  this audits and corrects. Its 8-findings measurement holds; its two completeness claims did not.
- [TASK-111](../_archive/111-make-lint-reports-zero-issues-while-an-available-analyzer-has-50.md) — the first
  link. Same shape, two records earlier.
- [TASK-116](../_archive/116-stack-override-warning-goes-to-stdout.md) — the standard that a criterion counts
  only when the gate fails on reverted code, which is why the three mutations above exist and why
  the Close fix is explicitly *not* claimed as unit-tested.
- [TASK-118](../_archive/118-a-health-check-that-never-passes-is-still-exit-0.md) — the family: a check whose
  silence is read as a pass.
