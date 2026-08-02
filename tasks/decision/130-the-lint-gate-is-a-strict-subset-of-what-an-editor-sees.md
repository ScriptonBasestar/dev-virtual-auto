---
id: TASK-130
title: "The lint gate is a strict subset of what a gopls-backed editor sees, and closing that costs a second mandatory tool"
type: decision
priority: P3
status: decision
effort: S
created-at: 2026-08-02T00:00:00+09:00
scope: "Makefile lint target, .mise.toml, .github/workflows/ci.yml — no Go source changes"
---

# Task 130: Decide whether `gopls check` becomes a gate

## The blind spot

[TASK-127](../done/127-the-record-that-closed-the-coverage-gap-had-two-of-its-own.md) closed with
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

## Related

- [TASK-127](../done/127-the-record-that-closed-the-coverage-gap-had-two-of-its-own.md) — states
  this gap at `:114-121`; the `stringscut` divergence is mutation-tested there.
- [TASK-126](../done/126-the-lint-gate-still-hides-analyzers-task-111-said-it-had-none-left.md) —
  enabled `govet.enable-all` and `unparam`; its "editor and gate agree" claim was the overstatement
  TASK-127 corrected.
- [TASK-111](../done/111-make-lint-reports-zero-issues-while-an-available-analyzer-has-50.md) — the
  first link in the chain: `make lint` green while a shipped, default-off analyzer had 47 findings.
