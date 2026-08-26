---
id: TASK-203
title: "`make lint` reports findings from deleted worktrees and cannot read their //nolint"
type: bug
priority: P2
effort: S
created-at: 2026-08-19T19:12:04+09:00
source: "found verifying c41f88e's integration — gz-git reported 'baseline failure, non-worsening: count 1 → 1' and the one finding turned out to be anchored in a worktree that no longer exists"
scope: "Makefile lint target: scope the golangci-lint cache per checkout. No change to which linters run or to any Go source. Grew one line in `clean` — see the evidence note."
status: done
completed-at: 2026-08-19T18:22:53+09:00
quality-review: pass
quality-reviewed-at: 2026-08-19T18:22:53+09:00
verified-at: 2026-08-19T18:22:53+09:00
archived-at: 2026-08-19T18:22:53+09:00
quality-review-evidence: |
  - kind: automated
    command-or-step: "AC1/AC2/AC4 — the target scopes the cache, the path is ignored and untracked, the suppression survived"
    result: `grep -c GOLANGCI_LINT_CACHE Makefile` → 1 (was 0); `git check-ignore -q tmp/golangci-lint-cache` → rc 0 and `git ls-files` on it prints nothing; `grep -c 'nolint:staticcheck' internal/runner/interaction_tree.go` → 1. The escaping suppression was not "resolved" by rewriting the string it fires on
  - kind: automated
    command-or-step: "AC3/AC6/AC7 — make lint, make test, make doc-check on the changed tree"
    result: rc 0, rc 0, rc 0. lint reports `0 issues.` and the cache lands at `tmp/golangci-lint-cache` (10M) inside the checkout rather than in the machine-wide directory
  - kind: automated
    command-or-step: "AC5 — the four-step reproduction, run as a controlled A/B with the observer held fixed (the primary checkout, unfixed, both times)"
    result: control (worktree at origin/master, shared cache) — lint inside rc 0, remove the worktree, lint in the primary **rc 2, 1 issue, 3 dead-path references**. Fixed (worktree at this branch) — lint inside rc 0, remove, lint in the primary **rc 0, 0 issues, 0 dead-path references, with no cache clean in between**. The ghost still fires on demand today, so the green half is not a repro that stopped working
  - kind: automated
    command-or-step: "the redirect observed at the source rather than inferred from the verdict"
    result: after linting inside the fixed worktree, `golangci-lint cache status` printed its `Dir:` line with no `Size:` line — the machine-wide cache stayed empty while the worktree held 10M of its own
  - kind: automated
    command-or-step: "Open Question 1 — tmp/ or build/, decided on measurement"
    result: `BUILD_DIR := ./bin` and `clean` is exactly `rm -rf ./bin` + `go clean -cache`, so nothing in this Makefile sweeps `tmp/`, and `.gitignore:34` already ignores it. The card's worry that `tmp/` may be swept does not hold in this repository
  - kind: automated
    command-or-step: "Open Question 2 — do go vet and gopls check share the defect"
    result: no, and neither needed a change. `gopls check` is handed an explicit file list from `find cmd internal tools -name '*.go'`, recomputed per run under the current directory. `go vet` renders positions relative to the module it was invoked in — measured on a two-copy scratch module sharing one GOCACHE: copy A vetted, A's path made to disappear, byte-identical copy B still printed `main.go:7:14`, a path that resolves inside B
  - kind: manual
    command-or-step: "scope grew by one line outside the lint target"
    result: accepted, and caused by this change rather than found beside it. `go clean -cache` clears GOCACHE, a different variable, so scoping the lint cache left `clean` with no route to it at all. Per-checkout scoping does not remove the need — deleting a source file inside a single checkout strands its cached findings by the same mechanism, just more rarely
  - kind: manual
    command-or-step: "boundary stated rather than left to be inferred from the criteria"
    result: the fix holds for lint runs that go through `make`. A bare `golangci-lint run` still uses the default cache and can still replay a dead path. The integration path does go through make — `gz-git integrate check` reports `PASS make lint`
---

# Task 203: `make lint` reports findings from deleted worktrees and cannot read their //nolint

## Summary

`make lint` fails in a clean checkout, reporting a finding whose file path is a
worktree that has been removed:

```
../../../worktrees/dev-virtual-auto/celee__fix__task-type-vocabulary/internal/runner/interaction_tree.go:83:9:
  ST1005: error strings should not end with punctuation or newlines (staticcheck)
1 issues:
* staticcheck: 1
make: *** [lint] Error 1
```

accompanied by warnings that golangci-lint cannot read the file it is reporting
on (`open ...: no such file or directory`).

The finding is not a false positive. ST1005 does fire on that string. It is a
**deliberately suppressed** finding escaping its suppression —
`internal/runner/interaction_tree.go:83` carries:

```go
return fmt.Errorf( //nolint:staticcheck // ST1005: last token is a YAML key, not punctuation
	"%s has nothing to run — add command:, script:, script_file:, steps:, service:, pod:, or default_args:",
	name,
)
```

`//nolint` directives are resolved by re-reading the source file, not from the
cache entry. When the anchor path is gone, golangci-lint replays the cached
diagnostic and cannot read the directive that silenced it, so a resolved issue
resurfaces as a gate failure.

## Reproduction

Measured at `c41f88e`, four steps, each exit code observed directly (not through
a pipe):

| # | action | `make lint` |
|---|---|---|
| 1 | `golangci-lint cache clean`, then lint **inside** worktree `W` | rc=0, `0 issues.` |
| 2 | `git worktree remove W` | — |
| 3 | lint in the **primary checkout** | **rc=2**, ST1005 at `W`'s dead path |
| 4 | `golangci-lint cache clean`, then lint in the primary checkout | rc=0, `0 issues.`, 0 dead-path references |

Step 1 is what plants it: linting inside `W` writes cache entries keyed to `W`'s
absolute paths into a cache shared by every checkout on the machine.

Two independent instances were observed on 2026-08-19, naming two different
removed worktrees (`celee__chore__register-review-residue`, then
`celee__fix__task-type-vocabulary`), so this is not specific to one branch.

## Why it recurs on its own

The git workflow mandates a worktree per task and reclamation of that worktree in
the same step as integration. Every completed task therefore deletes a path that
the shared golangci-lint cache still refers to. The next `make lint` in any
checkout — the integrator's, or a peer session's — inherits the ghost. The two
instances above are one session inheriting a peer's, then inheriting its own.

## Why it matters beyond the noise

**Measured.** `gz-git integrate run` emitted, verbatim: `WARN make lint — baseline
failure, non-worsening: count 1 → 1, no diagnostics on changed paths`. The count
of 1 was the phantom; the tree had zero findings. So the gate reached a correct
verdict (the change was markdown) from a number that did not describe the tree.

**Unverified hypothesis, recorded as such.** If a phantom inflates the baseline,
a genuine finding introduced by a change could arrive as `1 → 1` and read as
non-worsening. This is *not* measured. It is a claim about how `gz-git` diffs and
counts findings, and gz-git's source is not available on this machine — only the
installed binary (`/Users/archmagece/go/bin/gz-git`, no source checkout found).
Confirming or refuting it means reading how gz-git pairs before/after findings:
if it matches on file+line+linter rather than on a bare count, a dead-path
phantom cannot mask a live finding and the hypothesis is void.

Do not treat that bullet as a reason to fix this card. **The measured reason is
sufficient on its own**: a lint gate that fails on code containing no findings is
wrong regardless of what any downstream consumer does with the number.

The one judgement (not a measurement) worth stating: once a reader learns that
dead-path findings are phantoms, dismissing them becomes a habit, and the habit
does not check the path every time.

This is the mirror of the defect the `lint` target already guards against for
`gopls` (TASK-130, comment at the target): there the gate could report a pass it
had not earned, here it reports a failure it has not earned. Same root category —
the verdict decoupled from the measurement.

## Proposed fix

Point `GOLANGCI_LINT_CACHE` at a path inside the checkout in the `lint` target,
so each checkout and worktree keeps its own cache and can never inherit another's
paths. The target currently sets no cache environment at all
(`grep -c 'GOLANGCI_LINT_CACHE' Makefile` → 0).

A per-checkout cache directory must be gitignored and should sit under the
project's designated build/temp area rather than at the repo root.

Rejected alternative: clearing the cache in the reclaim step. Reclamation is done
by `gz-git`, which is outside this repo, and it would not help a peer session that
never ran the reclaim.

## Completion Criteria

- [x] The `lint` target scopes the golangci-lint cache to the checkout | verify: `/usr/bin/grep -c 'GOLANGCI_LINT_CACHE' Makefile` returns ≥ 1 (today: 0)
- [x] The cache directory it points at is gitignored and untracked | verify: human — read the path the `lint` target assigns, then confirm `git check-ignore -q <path>` exits 0 and `git ls-files -- <path>` prints nothing. Bound to a human because the path is an Open Question below; do not hardcode a guess here.
- [x] Lint still passes on a clean tree | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && make lint`
- [x] The suppression that escaped is still present and still justified, i.e. the fix did not "resolve" this by rewriting the string | verify: `/usr/bin/grep -c 'nolint:staticcheck' internal/runner/interaction_tree.go` returns 1
- [x] The 4-step reproduction no longer reds at step 3 | verify: human — create a worktree, `make lint` in it, remove it, then `make lint` in the primary checkout; step 3 must report `0 issues.` without a prior cache clean
- [x] `make test` passes | verify: `make test`
- [x] `make doc-check` passes | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && make doc-check`

## References

- `Makefile` `lint:` target — the two golangci-lint invocation branches (mise and bare)
- `Makefile` `lint:` target comment — the TASK-130 `gopls` rc-check, the same class of gate-integrity guard
- `internal/runner/interaction_tree.go:83` — the `//nolint:staticcheck` that cannot be re-read once its path is gone
- `tasks/_archive/130-*.md` — gate reporting a verdict it did not earn, the pass-side instance

## Open Questions

- ~~Should the cache live under `tmp/` or under `build/`?~~ **Answered: `tmp/`.** The
  worry was that `tmp/` may be swept. It is not, here: `BUILD_DIR := ./bin` and `clean`
  is exactly `rm -rf ./bin` plus `go clean -cache`. Nothing in this Makefile touches
  `tmp/`, and `.gitignore:34` already ignores it. `clean` now drops the lint cache
  deliberately, which is a different thing from sweeping it out from under a run.
- ~~Do `go vet` and `gopls check` share the defect?~~ **Answered: no, and neither needed
  a change.** `gopls check` is handed an explicit file list from
  `find cmd internal tools -name '*.go'`, recomputed per run under the current directory,
  so it cannot name a path outside the checkout. `go vet` renders positions relative to
  the module it was invoked in — measured on a two-copy scratch module sharing one
  `GOCACHE`: copy A was vetted, A's path was made to disappear, and vetting byte-identical
  copy B still printed `main.go:7:14`, a path that resolves inside B. The symptom that
  defines this card — a finding at a path that no longer exists — has no route into
  either tool's output.
- Whether a phantom can mask a live finding through gz-git's `N → N` non-worsening
  judgement is unverified and **does not belong to this card or this repo** — it is
  a question about gz-git's counting logic, whose source is not on this machine. If
  it turns out to be real it wants a card in whichever repo owns gz-git. Nothing in
  this card's fix depends on the answer.

## Technical Notes

The same run also surfaced an `errcheck` finding at
`.../celee__fix__task-type-vocabulary/internal/lifecycle/orchestrator_test.go:426`
that the `generated_file_filter` processor could not process, for the identical
reason (`failed to parse file: ... no such file or directory`). That one stayed a
warning and never reached the issue list, so the observable symptom is one
staticcheck issue — but it shows the dead-path replay affects the processor chain,
not just `//nolint` resolution.

Exit codes throughout were read with `>file 2>&1; echo $?`, never through a pipe —
`make lint | tail` reports the pipe's status and shows rc=0 while make itself
reports `Error 1`.
