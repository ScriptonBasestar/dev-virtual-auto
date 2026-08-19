---
id: TASK-203
title: "`make lint` reports findings from deleted worktrees and cannot read their //nolint"
type: bug
priority: P2
effort: S
created-at: 2026-08-19T19:12:04+09:00
source: "found verifying c41f88e's integration — gz-git reported 'baseline failure, non-worsening: count 1 → 1' and the one finding turned out to be anchored in a worktree that no longer exists"
scope: "Makefile lint target: scope the golangci-lint cache per checkout. No change to which linters run or to any Go source."
status: todo
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

`gz-git integrate run` judges lint failures as "baseline failure, non-worsening:
count 1 → 1, no diagnostics on changed paths". That judgement was correct here,
but it is being made against a number that does not describe the tree. Two ways
this goes wrong:

- A phantom inflates the baseline, so a **real** finding introduced by the change
  can arrive as "1 → 1" and read as non-worsening.
- Once a reader learns that dead-path findings are phantoms, the habit of
  dismissing them is one careless step from dismissing a live one.

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

- [ ] The `lint` target scopes the golangci-lint cache to the checkout | verify: `grep -c 'GOLANGCI_LINT_CACHE' Makefile` returns ≥ 1 (today: 0)
- [ ] The cache directory it points at is gitignored and untracked | verify: human — read the path the `lint` target assigns, then confirm `git check-ignore -q <path>` exits 0 and `git ls-files -- <path>` prints nothing. Bound to a human because the path is an Open Question below; do not hardcode a guess here.
- [ ] Lint still passes on a clean tree | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && make lint`
- [ ] The suppression that escaped is still present and still justified, i.e. the fix did not "resolve" this by rewriting the string | verify: `grep -c 'nolint:staticcheck' internal/runner/interaction_tree.go` returns 1
- [ ] The 4-step reproduction no longer reds at step 3 | verify: human — create a worktree, `make lint` in it, remove it, then `make lint` in the primary checkout; step 3 must report `0 issues.` without a prior cache clean
- [ ] `make test` passes | verify: `make test`
- [ ] `make doc-check` passes | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && make doc-check`

## References

- `Makefile` `lint:` target — the two golangci-lint invocation branches (mise and bare)
- `Makefile` `lint:` target comment — the TASK-130 `gopls` rc-check, the same class of gate-integrity guard
- `internal/runner/interaction_tree.go:83` — the `//nolint:staticcheck` that cannot be re-read once its path is gone
- `tasks/_archive/130-*.md` — gate reporting a verdict it did not earn, the pass-side instance

## Open Questions

- Should the cache live under `tmp/` (already gitignored, per the project's temp
  convention) or under `build/`? `tmp/` looks right, but the lint cache is a build
  artifact that benefits from surviving between runs, and `tmp/` may be swept.
- `go vet` and `gopls check` also run in this target and have their own caches
  (`GOCACHE`). Neither produced a dead-path finding in the observed instances, but
  neither was tested for it. Worth one measurement before deciding whether the fix
  should cover them too.

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
