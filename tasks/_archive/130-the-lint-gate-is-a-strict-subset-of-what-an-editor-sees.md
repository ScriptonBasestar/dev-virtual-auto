---
id: TASK-130
title: "The lint gate is a strict subset of what a gopls-backed editor sees, and closing that costs a second mandatory tool"
type: chore
priority: P3
normalized-by: "TASK-194 — was type: decision"
status: done
effort: S
resolved-at: 2026-08-02T00:00:00+09:00
resolution: "Chose option B. gopls 0.22.0 pinned in .mise.toml, a gopls check sweep added to make lint hard-failing like golangci-lint, and a matching pinned install plus check step added to the CI lint job"
created-at: 2026-08-02T00:00:00+09:00
scope: "Makefile lint target, .mise.toml, .github/workflows/ci.yml — no Go source changes"
verified-at: 2026-08-03T15:45:00+09:00
archived-at: 2026-08-03T15:45:00+09:00
verification-summary: |
  All eight criteria hold against real deliverables, and the central one was re-measured
  independently rather than trusted. A throwaway module in the scratchpad carrying the exact
  pre-072fbc5 form (`strings.SplitN(s, "\n", 2)[0]`, recovered from git history of
  internal/lifecycle/compose_error_test.go:38) is reported clean by golangci-lint 2.12.2 under the
  repo's own .golangci.yml ("0 issues.") and flagged by gopls v0.22.0 with -severity=hint. Both
  binaries are the mise-pinned ones. The task's two implementation discoveries reproduce exactly:
  gopls exits 0 while printing findings, and a tool failure yields exit 2 with empty stdout, which
  the pre-fix recipe tail swallowed into "GATE PASSED SILENTLY" under /bin/sh. The shipped rc check
  turns that into exit 1. The repo tree is clean of any TASK-130 residue; the only modified files
  are unrelated task records from other sessions.
---

# Task 130: Decide whether `gopls check` becomes a gate

## The blind spot

[TASK-127](127-the-record-that-closed-the-coverage-gap-had-two-of-its-own.md) closed with
this stated and unresolved: gopls's `modernize` suite exceeds golangci-lint 2.12.2's vendored one,
and the divergence runs *inside* same-named analyzers — vendored `stringscut` catches the
`strings.Index` form but not the `strings.SplitN` form. `make lint` cannot see that class. A
contributor whose editor runs gopls sees hints the gate will never enforce; a contributor whose
editor does not see nothing at all, and neither does CI.

No task ever decided against wiring it in. TASK-127 recorded the gap as open, not as considered
and rejected, so this is unfinished business rather than a settled call being re-litigated.

## Measured

| | value | how |
|---|---|---|
| gopls | v0.22.0 | `gopls version` |
| golangci-lint | 2.12.2 | `golangci-lint version` |
| linters enabled | **7** | `golangci-lint linters`, section-counted |
| linters disabled by config | **107** | same command |
| Go files | 235 | `find cmd internal tools -name '*.go'` |
| `gopls check -severity=hint` findings | **0** | exit 0 over all 235 files |
| that sweep's wall time | **~2s** | timed on this machine |
| gopls declared as a dependency anywhere | **0 hits** | `git grep -in gopls` excluding `tasks/done` |

The 107 are almost all style (`wsl`, `varnamelen`, `nlreturn`) and enabling them is not proposed
here — the number is recorded because "nothing is excluded" was the claim TASK-126 got wrong.

**0 findings today is the important one.** Wiring `gopls check` in would be a regression guard,
not a cleanup. There is nothing to fix; the question is only whether the next
`SplitN`-class miss should be catchable before an editor happens to notice it.

## The cost

`gopls` is pinned nowhere — not `.mise.toml` (which pins only `go` and `golangci-lint`), not
`go.mod`, no `tools.go`. CI (`.github/workflows/ci.yml`, jobs `test`/`lint`/`goreleaser-check`)
installs golangci-lint through `golangci/golangci-lint-action@v8`; there is no equivalent action
for gopls, so it would need its own install step.

The Makefile has **no graceful-skip precedent**. Its only tool-presence check is `lint`, and it
hard-fails:

```makefile
	else \
		echo "Install golangci-lint v2 (...) or run 'mise install'"; exit 1; \
	fi
```

## Options

- **A — CI-only job.** Contributors unaffected; the class is caught before merge. Feedback arrives
  only after push.
- **B — Pin gopls in `.mise.toml`, hard-fail in `make lint` like golangci-lint.** Consistent with
  the one pattern the Makefile already has. Cost: a second mandatory tool for every contributor,
  to guard a check that currently finds nothing.
- **C — New `make lint-hint`, advisory locally (skip when gopls is absent), mandatory in CI.**
- **D — Do nothing; record as an accepted limitation and close.**

## Recommendation: B, and specifically not C

C is the tempting one and it is the wrong shape for this repo. A gate that passes when its tool is
missing is a gate whose silence is read as a pass — the exact defect class closed in TASK-074,
076, 079, 107, 111, 112, 116, 118, 124, 125, 126, 127 and 128. Introducing a thirteenth instance
to avoid a `mise install` would be a poor trade, and the repo has no skip-gracefully precedent to
extend anyway.

The real question B forces into the open is worth answering directly: **is a 2-second, zero-finding
regression guard worth making a second tool mandatory?** That is a team-tooling call, not a
technical one, which is why this is filed as a decision rather than done.

D is a legitimate answer. If it is chosen, TASK-127's residual-gap paragraph should be updated to
say the gap was accepted, not that it remains open — otherwise the next audit reopens it.

## Resolution

**Option B, as recommended.** `gopls` 0.22.0 is pinned in `.mise.toml` via mise's `go:` module
backend (no plain `gopls` shortname exists in its registry); `make lint` runs a `gopls check
-severity=hint` sweep after golangci-lint, hard-failing on the same missing-tool branch shape the
golangci-lint block already uses; CI installs the same version explicitly, since no
`golangci-lint-action` equivalent exists for gopls.

Two things the analysis above did not know, both found during implementation:

**`gopls check` exits 0 even when it reports findings.** It is not exit-code-driven like a linter —
diagnostics go to stdout and the status stays 0. The measured table above (`exit 0 over all 235
files`) was consistent with zero findings, so this never surfaced. A naive `gopls check ... || exit
1` would therefore never fire. Both the Makefile and CI capture the output and fail on non-empty
text, matching the `gofmt -s -l` capture-and-check precedent already in `fmt-check`.

**That corollary opened the exact hole this task rejected option C for.** Since a non-zero exit
cannot mean "findings", it can only mean the tool failed — and gopls fails with **stdout empty**
(measured: exit 2, nothing on stdout, message on stderr). `make` runs recipes under `/bin/sh`
without `-e`, so the first implementation discarded that status and an unrunnable gopls read as a
clean lint:

| shell | tool-failure behaviour |
|---|---|
| `/bin/sh`, no `-e` (make) | **passed silently, exit 0** |
| `bash -e` (GitHub Actions default) | exit 2 |

CI was protected only by the runner's default shell. Both now check the status explicitly, so the
invariant does not depend on a platform default that can change.

## Acceptance criteria

- [x] gopls pinned and resolvable | verify: `mise which gopls && mise exec -- gopls version` — **`v0.22.0`, resolved from the mise install path**
- [x] Local and CI pins cannot drift apart silently | verify: `grep gopls .mise.toml .github/workflows/ci.yml` — **`"go:golang.org/x/tools/gopls" = "0.22.0"` and `gopls@v0.22.0` — same version, and the CI comment states why they must match**
- [x] **The gate catches what golangci-lint misses** — the criterion the whole change exists for | verify: `human — restore the strings.SplitN form at compose_error_test.go:38, run both gates` — **golangci-lint: `0 issues.`; `make lint`: `ERROR: gopls check found issues: ...:38:13-27: strings.SplitN call can be simplified using strings.Cut`, `make: *** [lint] Error 1`. Re-measured by the supervisor, not taken from a report; reverted byte-identical afterward (`git diff` empty)**
- [x] The gate does not pass silently when gopls cannot run | verify: `human — run the recipe's tail with gopls_cmd pointed at a stub that exits 2 with empty stdout` — **before the rc check: `GATE PASSED SILENTLY`, exit 0. After: `ERROR: gopls check could not run (exit 2)`, exit 1**
- [x] Missing gopls hard-fails rather than skipping | verify: `human — read the else branch` — **`echo "Install gopls ..."; exit 1`, same shape as the golangci-lint block; no skip-when-absent path exists**
- [x] Zero findings on the current tree, so this is a regression guard and not a cleanup | verify: `make lint` — **exit 0 over 237 Go files (235 at filing plus the two TASK-131 added)**
- [x] CI workflow still parses | verify: `python3 -c "yaml.safe_load(open('.github/workflows/ci.yml'))"` — **OK, lint job has 6 steps**
- [x] Full gate suite green | verify: `make test && make lint && make doc-check && make check-generate` — **all exit 0**

Known and accepted: the `$(find cmd internal tools -name '*.go')` argument is unquoted, which
`actionlint` flags as SC2046. It is the command form TASK-127's own acceptance criteria used, and
every path under those three directories is a plain Go source name. It would break on a filename
containing whitespace.

## Follow-up found at archival

⚠️ Making `gopls` mandatory did not reach the repo's own config. `dva.yml:8-15` still declares two
prerequisites — Go and golangci-lint — while `Makefile:60-66` hard-fails without gopls, so a
contributor missing it gets a clean `dva doctor` and then a broken `make lint`. `dva.yml:55` also
still calls the interaction `Run linters (golangci-lint)`. Tracked as
[TASK-160](../todo/160-dvas-own-dva-yml-does-not-know-lint-needs-gopls.md).

## Related

- [TASK-127](127-the-record-that-closed-the-coverage-gap-had-two-of-its-own.md) — states
  this gap at `:114-121`; the `stringscut` divergence is mutation-tested there.
- [TASK-126](../_archive/126-the-lint-gate-still-hides-analyzers-task-111-said-it-had-none-left.md) —
  enabled `govet.enable-all` and `unparam`; its "editor and gate agree" claim was the overstatement
  TASK-127 corrected.
- [TASK-111](../_archive/111-make-lint-reports-zero-issues-while-an-available-analyzer-has-50.md) — the
  first link in the chain: `make lint` green while a shipped, default-off analyzer had 47 findings.
