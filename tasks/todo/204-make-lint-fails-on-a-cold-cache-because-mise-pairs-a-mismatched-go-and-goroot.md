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
`.mise.toml` and `go.mod` both pin 1.26.5. In the PATH `mise exec` *constructs*,
`/opt/homebrew/bin` (position 15) precedes mise's own go install directory (position 20).
That signature holds in every failing run measured — but it describes the PATH the wrapper
built, not the PATH you handed it, and nothing inspectable beforehand predicts which one
you will get. See the grid below before drawing any PATH conclusion from it.

**Single-variable confirmation.** Holding everything else fixed and changing only which
`go` the wrapper resolves:

| `go` resolved | GOROOT | cold `golangci-lint run ./...` |
|---|---|---|
| Homebrew 1.26.6 | mise 1.26.5 | rc=1, `4 issues: * typecheck: 4` |
| mise 1.26.5 (shim first on PATH) | mise 1.26.5 | **rc=0, `0 issues.`** |

Neither `env -u GOROOT` nor an explicit `GOROOT=` override changes the outcome — mise
re-exports `GOROOT` itself.

## Workaround, and four explanations that turned out to be wrong

Putting mise's **shims** directory first resolves `go` to the version `GOROOT` names, and
the gate goes green:

```bash
export PATH="$HOME/.local/share/mise/shims:$PATH"
make lint      # measured on a cold cache: rc 0, "0 issues."
```

**That is a working configuration, not the condition** — and it is worth being blunt about
how much stronger that statement is than it looks. Four rules have been proposed for when
this fires. Every one was stated by someone who had measured something real, and every one
is falsified below. They are written down so the next reader spends the ten minutes reading
rather than the hour re-deriving.

The grid. Everything else held, each variant in its own process, run under `mise exec` in
the repo worktree, 2026-08-19. Reproduced independently by a second session on this
machine, cell for cell:

| # | PATH handed to `mise exec` | `go` it resolves | pair |
|---|---|---|---|
| A | as an activated shell inherits it | `/opt/homebrew/bin/go` 1.26.6 | **broken** |
| D1 | A, minus every `installs/go/*` entry | `/opt/homebrew/bin/go` 1.26.6 | **broken** |
| D2 | D1, minus the shims entry as well | mise 1.26.5 | ok |
| E | `installs/go/1.26.5/bin` + a minimal clean PATH | mise 1.26.5 | ok |
| F | a minimal clean PATH, no mise entries at all | mise 1.26.5 | ok |
| G | shims + a minimal clean PATH | mise 1.26.5 | ok |

What that kills:

1. **"mise directories must come first" / PATH ordering.** F and G sit at opposite
   extremes and both pass, while A fails with mise's own go install directory already at
   **position 1** of the inherited PATH. Ordering co-varies; it does not decide.
2. **"The directory is the axis"** — that a directory where mise's go tool is not active
   behaves differently. It does not. Both a checkout and a `git archive` extraction
   directory give identical results under every PATH above. This one survived only because
   the in-repo probe supporting it had been run with a PATH the measuring session had
   already fixed, without treating that habit as a variable.
3. **"It breaks when `mise exec` inherits a PATH holding mise's own go install
   directory."** Falsified in both directions by a single run: **E** has that directory and
   passes, **D1** has none and fails. Note what this would have cost — a Makefile check
   grepping for that path would look for a string that is *present* in a working
   configuration and *absent* in a broken one.
4. **"mise's recorded activation state is stale."** `__MISE_DIFF`, `__MISE_ORIG_PATH`,
   `__MISE_SESSION` and `MISE_SHELL` are all exported by zsh activation. Clearing them
   individually and together, with the raw PATH held fixed, changes nothing — every
   variant stays `go1.26.6` against `GOROOT` 1.26.5.

Also not the axis: `mise trust`. It looks like it should be, since extraction directories
are untrusted and the repo is trusted, but with the directory byte-identical and the two
arms in **separate processes** both trust states break alike. An A/B that runs both arms
inside one shell invocation will appear to confirm it, because the trust write never
reaches the second arm.

**What is actually established.** The ambient shell — no `mise exec` — resolves a
*consistent* pair. So the wrapper is not inheriting a broken environment; it degrades a
working one. The one visible difference between failing and passing runs is the PATH
`mise exec` itself constructs:

| | head of the PATH `mise exec` builds |
|---|---|
| failing (A, D1) | the pre-activation PATH — `/opt/homebrew/bin` at 15, mise's go install dir at 20 |
| passing (D2, E, F, G) | mise's install dirs at 1–6, go first |

With a heavily activated PATH the wrapper hands back something close to the pre-activation
PATH and does not re-apply its tools, while `GOROOT` stays exported from the outer
activation; with a PATH it does not recognise as its own activation it applies normally.
What state triggers that revert is **not known** — the four variables above are not it. It
is Open Question 4, and it does not block this card.

It does change the argument, though, and in the card's favour. **If the trigger cannot be
predicted from the PATH a developer has, no PATH-shaped check is safe** — see the E/D1 pair
above for a plausible one that would misfire in both directions. The mismatch has to be
read where it is used. Which is what this card asks for: with it present, `make lint`
reports four `could not import` errors about stdlib packages, reads as "the tree does not
compile", and sends the reader after a defect that is not there. The condition is now met
far more often than it used to be (see below).

One thing this card cannot do is cite its own prior art. This drift was diagnosed before,
with the same `does not match go tool version` giveaway and the same remedy, but only in a
session-local note that no future reader of this repo can open. That is most of why it is
written out at length here.

## Why it is not visible today

`make lint` passes in the primary checkout because its cache
(`tmp/golangci-lint-cache`) already holds successful results. The failure appears only
when golangci-lint has to re-analyse:

- a newly created checkout or worktree — measured: a fresh export of `c100ba0` fails
  `make lint` with rc=2, cold **and** warm;
- after `make clean`, which as of TASK-203 deletes `$(CURDIR)/tmp/golangci-lint-cache`;
- golangci-lint invoked **directly** against a fresh cache —
  `GOLANGCI_LINT_CACHE=… golangci-lint run ./...` in the primary checkout: rc=1, the same
  4 typecheck errors.

Do **not** try that last one through `make`. `Makefile:54` assigns `GOLANGCI_LINT_CACHE`
unconditionally, so an environment override is discarded and the run quietly uses the
checkout's warm cache instead. Measured here: the supplied directory stayed 0 KB while
`tmp/golangci-lint-cache` was created beside it. A run intended to be cold reads as a pass
without ever having been cold — registered separately as TASK-205. Until that is fixed,
force a cold run with `make clean`, or by moving the cache directory aside.

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

The comparison itself is two lines, measured from inside the `mise exec` branch. It
differs when the pairing is broken and matches when it is not:

```bash
go version | cut -d' ' -f3        # broken: go1.26.6   ok: go1.26.5
cat "$(go env GOROOT)/VERSION"    # broken: go1.26.5   ok: go1.26.5
```

The environment condition is fixed separately, by whichever of these the maintainer
prefers — removing or unlinking the Homebrew `go`, or moving `.mise.toml` and `go.mod` to
1.26.6. **Neither has been tested**, and per Open Question 4 nobody yet knows what makes
the wrapper revert, so treat both as candidates rather than known fixes. That choice is
out of scope here; this card only makes the build diagnose the condition.

## Completion Criteria

- [ ] The lint target inspects the toolchain pairing before running golangci-lint.
      verify: `grep -c 'go env GOROOT' Makefile` → at least 1 (today: **0**)
- [ ] A mismatched pairing fails with a message naming both versions, not with typecheck
      errors about stdlib imports.
      verify: human — construct the mismatch by putting a differing `go` first on PATH,
      run `make lint`, and confirm the output names the two versions and does not print
      `could not import`
- [ ] The check **compares two versions it reads at run time**. It must not sniff a PATH,
      a directory name, or a hard-coded version — the grid above contains a measured case
      where a path check is present in a working configuration and absent in a broken one,
      so such a check would misfire in both directions.
      verify: `grep -c 'GOROOT)/VERSION' Makefile` → at least 1 (today: **0**); measured
      to read `go1.26.6` vs `go1.26.5` when broken and to match when not
      + regression guard, not an acceptance test: `grep -c '1\.26\.[0-9]' Makefile` stays
      **0**, so no specific version is baked into the gate
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
2. ~~Is the mismatch reproducible in other sessions?~~ **Answered: yes, everywhere.** Two
   independent sessions reproduce it, in a checkout and in a `git archive` extraction
   directory alike, with PATH as an activated shell supplies it. Neither the session nor
   the directory is an axis. An earlier table in this card showed the two sessions
   disagreeing inside the repo; that cell was an artifact of the measuring session having
   already applied the workaround and not counting it as a variable. Re-measured without
   it, both break. Every shell here with mise activation applied is in the failing
   configuration by default — which is why the primary checkout passes on a warm cache
   rather than on a working toolchain.
3. `go vet` and `gopls check` are unaffected here (both passed in every failing run),
   but neither was tested under a deliberately mismatched pairing.
4. **What state makes `mise exec` hand back the pre-activation PATH instead of applying
   its tools?** Not known. Four candidate rules are falsified above and the four
   documented activation variables are not it. An answer would allow a cheaper fix — a
   shell-side correction instead of a check in the build — but the check is worth having
   either way, because it is the only defence that does not depend on knowing the answer.
