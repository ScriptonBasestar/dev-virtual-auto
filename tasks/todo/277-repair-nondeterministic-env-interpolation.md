---
id: TASK-277
title: "Repair nondeterministic env_file interpolation order"
type: bug
priority: P1
effort: S
exec-tier: standard
created-at: 2026-09-03T00:00:00+09:00
source: "TASK-246 gate run: TestLoadEnvFileKeepsSuccessfulPrecedence flakes on clean master"
scope: "internal/config/envinput.go parse->merge path, internal/config/environment.go MergeVars"
status: todo
---

# Task 277: repair nondeterministic env_file interpolation order

## Summary

A value that interpolates a variable declared earlier in the same `.env` file resolves
differently from run to run. The declaration order of a dotenv file is discarded during
parsing, so whether `${A}` is in scope when `B=${A}-derived` is stored depends on Go's
randomized map iteration. The user-visible effect is that the same `dva.yml` and the same
`.env` files produce different environment values on different invocations.

This is a correctness defect in the load path, not only a flaky test. It was found while
running TASK-246's gates and is unrelated to that card's changes — it reproduces on a clean
`master` working tree.

## Problem

`parseEnvFileStrict` (`internal/config/envfile.go`) returns `map[string]string`, which
carries no declaration order:

```
grep -n 'func parseEnvFileStrict' internal/config/envfile.go
```

`ApplyEnvFiles` (`internal/config/envinput.go`) then merges each entry's map and
interpolates once at the end:

```
grep -n 'env.MergeVars(entry.vars)' internal/config/envinput.go
```

but the effective interpolation happens *earlier*, inside `MergeVars`, per value, while
ranging over that unordered map:

```
grep -n 'e.Vars\[k\] = e.Interpolate(v)' internal/config/environment.go
```

So for `a.env` containing `A=first` and `B=${A}-derived`:

- if `A` is visited first, `B` stores `first-derived`;
- if `B` is visited first, `${A}` is unresolved and `B` stores `${A}-derived`, which the
  final `interpolateEnvVars` pass later resolves against whatever `A` ended up as — including
  a value from a *later* file.

Reproduced on a clean `master` (`1e36b15`, no local modifications), 30 sequential runs:

```
$ for i in $(seq 1 30); do go test ./internal/config \
    -run TestLoadEnvFileKeepsSuccessfulPrecedence -count=1 >/dev/null 2>&1 \
    && echo pass || echo fail; done | sort | uniq -c
  27 pass
   3 fail
```

The failing assertion is `envinput_test.go:168`:

```
B = "second-derived", want interpolation against the value in scope when it was declared
```

`TestLoadEnvFileKeepsSuccessfulPrecedence` therefore already documents the intended
semantics — "interpolation against the value in scope when it was declared" — and the
implementation only satisfies them by luck.

### The blast radius is wider than the one flaking test

`MergeVars` is the shared merge primitive, not an `env_file`-only helper. Non-test call
sites, measured on `c6aa64b`:

```
grep -rn 'MergeVars(' --include='*.go' . | grep -v _test
```

21 matching lines, of which one is the declaration (`internal/config/environment.go:71`)
and one is a comment reference (`internal/lifecycle/resolver.go:382`) — **19 real call
sites**. They span `internal/config/envinput.go:247`, `internal/config/environment.go:52`,
six in `internal/lifecycle/orchestrator.go`, four in `internal/cli/compose.go`, and
`internal/cli/build.go:123`, `internal/cli/run.go:84`/`:140`,
`internal/cli/validate.go:409`/`:446`, `internal/cli/plan_runtime.go:47`,
`internal/cli/root.go:469`.

`interpolateEnvVars` has exactly **one** non-test call site — `internal/config/envinput.go:250`,
immediately after the `ApplyEnvFiles` merge loop:

```
grep -rn 'interpolateEnvVars(' --include='*.go' . | grep -v _test
```

That asymmetry changes what the defect is. `Interpolate` returns the literal `${VAR}` match
when the name is not in scope (`internal/config/environment.go`, the `// Return original if
not found` branch), so a value that loses the map-iteration race is stored **unresolved**.
On the `env_file` path the trailing `interpolateEnvVars(env)` pass repairs it — badly, by
resolving against the final merged map instead of declaration scope, which is the (A)/(B)
divergence above. On the other 18 call sites there is no repair pass at all: plan `vars`,
entry `vars`, mode `environment`, and exports that reference a sibling key declared in the
same batch keep the literal `${VAR}` **permanently**.

So the observable failure is not "the value is sometimes one thing and sometimes the other".
On most paths one of the two outcomes is an uninterpolated string handed to a child process.

Independent reproduction on clean `master`: 40 runs → 5 failures (a second session), against
the 30 runs → 3 failures recorded above. Both sample the same defect.

Consequence for the fix: whichever semantics is chosen, fixing only `ApplyEnvFiles` leaves
the other 18 call sites nondeterministic. The repair has to land in `MergeVars` itself, or
`MergeVars` has to stop being handed an unordered map.

## Decision required before implementing

The test names one semantics; the code implements two, nondeterministically. Fixing this
means picking one and stating it:

- **(A) Declaration-order scope** — a value interpolates against what is in scope at the
  point it is declared, so an earlier file's derived value is not retroactively rewritten by
  a later file. This is what the existing test asserts and what dotenv/Compose users expect.
- **(B) Post-merge scope** — all files merge first, then everything interpolates against the
  final map. Simpler, but makes `B` in an early file silently depend on a later file, and
  contradicts the existing test.

(A) is the recommended reading: it is the one already written down, and (B) would require
rewriting a test that was added deliberately to pin the behavior.

## Completion Criteria

- [ ] Dotenv declaration order survives parsing — the parse result carries order, or merge consumes the file in declaration order rather than ranging over an unordered map, so no interpolation decision is made while ranging an unordered map | verify: `! /usr/bin/grep -q 'e.Vars\[k\] = e.Interpolate(v)' internal/config/environment.go`
- [ ] `MergeVars` no longer interpolates while ranging over an unordered map, or is fed an ordered sequence | verify: `go test ./internal/config -count=1`
- [ ] `TestLoadEnvFileKeepsSuccessfulPrecedence` passes deterministically — 50 consecutive runs, zero failures | verify: `go test ./internal/config -run TestLoadEnvFileKeepsSuccessfulPrecedence -count=50`
- [ ] A regression test pins the chosen semantics explicitly for the two-file case (`A` redefined by a later file, `B` derived from `A` in the earlier one) so a future refactor cannot silently switch between (A) and (B) | verify: `go test ./internal/config -count=1`
- [ ] Multi-key single-file interpolation is order-stable under repeated runs, not only the two-key case | verify: `go test ./internal/config -count=20`
- [ ] The whole package is clean and race-free | verify: `go test -race ./internal/config -count=1`

## Notes

Owner coordination: the parse/merge path in `envinput.go` arrived with TASK-248
(`b23780e`, "feat(cli): enforce required env policy per command route"). This card does not
change the failure-reporting behavior that card froze — only the order in which loaded
values are applied.
