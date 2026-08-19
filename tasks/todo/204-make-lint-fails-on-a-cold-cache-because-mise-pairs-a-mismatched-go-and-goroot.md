---
id: TASK-204
title: "`make lint` fails on a cold cache because `mise exec` pairs a mismatched go and GOROOT"
type: bug
priority: P2
effort: S
created-at: 2026-08-19T18:39:59+09:00
source: "found while re-running TASK-203's reproduction against the landed fix — the control worktree's first `make lint` failed with 4 typecheck errors that had nothing to do with the cache"
scope: "Makefile lint target: detect a go/GOROOT mismatch and say so. No change to which linters run, no Go source change. The mismatch itself is an environment condition, not a dva defect — this card is about the Makefile reporting it legibly instead of as four import errors."
status: todo
---

## Summary

`make lint` exits 2 with four `typecheck` errors whenever the golangci-lint cache is cold
*and* the shell resolves a `go` that disagrees with the `GOROOT` mise exports. The errors
name stdlib imports and read as if the tree does not compile:

```
tools/cilabels/main.go:11:2: could not import os (.../go/1.26.5/src/os/dir.go:8:2:
  could not import internal/bytealg (... compile: version "go1.26.5" does not match
  go tool version "go1.26.6"))) (typecheck)
```

The tree compiles fine — `go vet ./...` and `go build` both pass in the same run, and
the *same* golangci-lint binary reports `0 issues.` when invoked without the `mise exec`
wrapper. The failure is a toolchain pairing defect inside the wrapper.

**Measured mechanism.** The `lint` target prefers `mise exec -- golangci-lint`. Under
that wrapper:

| what | value |
|---|---|
| `go` binary actually resolved | `/opt/homebrew/bin/go` → **go1.26.6** |
| `GOROOT` mise exports | `~/.local/share/mise/installs/go/1.26.5` → **go1.26.5** |

go 1.26.6 runs, looks up its tools in a GOROOT belonging to 1.26.5, finds `compile`
1.26.5, and refuses the pair. Homebrew's go 1.26.6 was installed **2026-08-18 15:51**;
`.mise.toml` and `go.mod` both pin 1.26.5, and `/opt/homebrew/bin` precedes mise's own
go install directory in the PATH `mise exec` constructs (positions 15 and 20).

**Single-variable confirmation.** Holding everything else fixed and changing only which
`go` the wrapper resolves:

| `go` resolved | GOROOT | cold `golangci-lint run ./...` |
|---|---|---|
| Homebrew 1.26.6 | mise 1.26.5 | rc=1, `4 issues: * typecheck: 4` |
| mise 1.26.5 (shim first on PATH) | mise 1.26.5 | **rc=0, `0 issues.`** |

Neither `env -u GOROOT` nor an explicit `GOROOT=` override changes the outcome — mise
re-exports `GOROOT` itself.

## Known workaround — this is a recurrence, not a new condition

This drift is already documented, including the `does not match go tool version` giveaway.
Putting mise's **shims** directory first resolves `go` to the version `GOROOT` names, and
the gate goes green:

```bash
export PATH="$HOME/.local/share/mise/shims:$PATH"
make lint      # measured on a cold cache: rc 0, "0 issues."
```

Two details worth keeping, because they are easy to get wrong:

- The **shims** directory works; prepending mise's go **install** bin directory does not.
  `mise exec` reorders its own install paths but leaves an unrelated directory alone — a
  scratch directory holding just a `go` symlink to the pinned toolchain also works.
- The shims directory is already on PATH here, at position 59, behind `/opt/homebrew/bin`
  at 15. Being present is not the same as being first.

So the environment half of this has a known one-line fix and needs no investigation. What
this card is for is the other half: with the mismatch present, `make lint` reports four
`could not import` errors about stdlib packages, which reads as "the tree does not
compile" and sends the reader after a defect that is not there. That is worth fixing on
its own, because the condition is now met far more often than it used to be (see below).

## Why it is not visible today

`make lint` passes in the primary checkout because its cache
(`tmp/golangci-lint-cache`) already holds successful results. The failure appears only
when golangci-lint has to re-analyse:

- a newly created checkout or worktree — measured: a fresh export of `c100ba0` fails
  `make lint` with rc=2, cold **and** warm;
- after `make clean`, which as of TASK-203 deletes `$(CURDIR)/tmp/golangci-lint-cache`;
- any run pointed at a fresh `GOLANGCI_LINT_CACHE` — measured in the primary checkout
  itself: rc=1, the same 4 typecheck errors.

TASK-203's per-checkout cache scoping is correct and is **not** the cause, but it does
remove what was masking this: every task now starts from a cold cache by design, so a
condition that used to hide behind one long-lived shared cache is now met on every new
worktree.

## Reproduction

1. `mkdir /tmp/probe && git archive HEAD | tar -x -C /tmp/probe`
2. `cd /tmp/probe && make lint`
3. Observe rc=2 and `4 issues: * typecheck: 4`, with `does not match go tool version`
   inside the import chain.
4. `PATH=<dir containing a go symlink to mise's pinned go>:$PATH make lint` → rc=0.

## Proposed fix

Make the `lint` target refuse to run golangci-lint when the resolved `go` and the
exported `GOROOT` disagree, and print both versions. This follows the same principle the
target already applies to `gopls check` (TASK-130): a tool that cannot run must not be
allowed to read as a clean lint, and the failure must name its own cause.

The environment condition is fixed separately, by whichever of these the maintainer
prefers — removing or unlinking the Homebrew `go`, or moving `.mise.toml` and `go.mod`
to 1.26.6. That choice is out of scope here; this card only makes the Makefile diagnose
it.

## Completion Criteria

- [ ] The lint target inspects the toolchain pairing before running golangci-lint.
      verify: `grep -c 'GOROOT' Makefile` → at least 1 (today: **0**)
- [ ] A mismatched pairing fails with a message naming both versions, not with typecheck
      errors about stdlib imports.
      verify: human — construct the mismatch by putting a differing `go` first on PATH,
      run `make lint`, and confirm the output names the two versions and does not print
      `could not import`
- [ ] The failure names the remedy, so the reader does not have to rediscover it.
      verify: `grep -c 'shims' Makefile` → at least 1 (today: **0**)
- [ ] No change to which linters run.
      verify: `grep -c 'golangci-lint run ./...' Makefile` → 2, unchanged
- [ ] The reported `go`/`GOROOT` pairing is read from the same wrapper the target
      actually uses, not from the ambient shell.
      verify: human — the check must run inside the `mise exec` branch, since the
      ambient shell resolves a *consistent* pair and would report no problem

## Open Questions

1. Should the target hard-fail on a mismatch, or warn and fall back to the bare
   `golangci-lint` branch, which is measured to work? Failing is more honest; falling
   back keeps the gate usable on a machine nobody has cleaned up yet.
2. ~~Is the mismatch reproducible in other sessions?~~ Largely answered. The primary
   checkout's cache was written at 2026-08-19 18:25 — *after* the Homebrew upgrade — and
   holds passing results, so that session resolved a consistent pair, which the shims-first
   PATH above accounts for. The condition is therefore per-session PATH ordering, not a
   machine-wide breakage. What remains open is narrower: whether anything in the repo
   should assert the ordering, or whether it stays a shell-profile concern.
3. `go vet` and `gopls check` are unaffected here (both passed in every failing run),
   but neither was tested under a deliberately mismatched pairing.
