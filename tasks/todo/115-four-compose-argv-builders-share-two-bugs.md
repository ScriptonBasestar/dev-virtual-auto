---
id: TASK-115
title: "Four copies of the compose argv builder share the same two bugs: a `compose` seed that is never dropped, and an unguarded `parts[0]`"
type: fix
priority: P3
effort: M
status: todo
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/cli/compose.go:787 buildComposeArgsForEntry, internal/cli/compose.go:821 buildComposeArgs, internal/runner/compose.go:16 composeArgv, internal/lifecycle/compose.go:149 (*ComposePlugin).buildArgs"
---

# Task 115: one bug written down four times

## The four copies

Located by `grep -n 'SplitCommand' -r internal --include="*.go"` plus `grep -n '"compose"'`:

| # | site | seed |
| --- | --- | --- |
| 1 | `internal/cli/compose.go:789` | `composeArgs := []string{"compose"}` |
| 2 | `internal/cli/compose.go:823` | `composeArgs := []string{"compose"}` |
| 3 | `internal/runner/compose.go:19` | `fullArgs = append(fullArgs, "compose")` |
| 4 | `internal/lifecycle/compose.go:153` | `args := []string{"compose"}` |

All four then run the same block, byte-for-byte apart from variable names:

```go
if cc.Command != "" {
	parts := dvaexec.SplitCommand(cc.Command)
	composeCmd = parts[0]
	if len(parts) > 1 {
		composeArgs = parts[1:]
	}
}
```

Two earlier probe reports each described "three builders". There are four — copy 4 lives in
`internal/lifecycle` and neither report reached it. Counted rather than recalled.

## Bug A — a single-token `command:` leaves the `compose` seed in place

The seed is only replaced when `len(parts) > 1`. So:

| `command:` in dva.yml | parts | argv produced |
| --- | --- | --- |
| `docker compose` | `[docker compose]` | `docker compose …` ✅ |
| `podman-compose` | `[podman-compose]` | `podman-compose **compose** …` ❌ |
| `docker-compose` | `[docker-compose]` | `docker-compose **compose** …` ❌ |

The single-token form is the natural way to name a v1-style or drop-in binary, and it is exactly
the form that breaks. The user gets `unknown command "compose"` from a tool they configured
correctly, and nothing in DVA's output explains where the extra word came from.

The `else` branch is missing: when `len(parts) == 1` the seed must be cleared, not kept.

## Bug B — a whitespace-only `command:` panics

`dvaexec.SplitCommand` (`internal/exec/exec.go:111-141`) appends to `parts` only under
`if current.Len() > 0`, in both the separator branch at `:129` and the tail at `:137`. So a string
made entirely of spaces or tabs produces **nil**, and so does `"''"` — the quote branch at `:125`
consumes both characters and writes nothing.

Meanwhile the guard upstream is `cc.Command != ""`, which is true for `"   "`. The two conditions
do not agree, and `parts[0]` on the next line indexes into nil:

```
panic: runtime error: index out of range [0] with length 0
```

A YAML author writing `command: " "` or `command: ''` — or leaving trailing whitespace after
deleting a value — gets a Go panic and a stack trace instead of a config error. Neither probe
report found this one; it turned up while reading `SplitCommand` to confirm bug A.

The same unguarded `parts[0]` appears in all four copies.

## Drift, already begun

Copies 1 and 2 join paths with `f = cfgDir + "/" + f`; copies 3 and 4 use `filepath.Join(cfgDir, f)`.
Behaviourally equivalent today for the inputs DVA produces — `filepath.Join` additionally cleans the
result, so `a` + `/` + `./b` differs from `a/b` — but the point is that four copies of one function
have already stopped being four copies of one function. That is the argument for consolidating, and
it is worth more than either individual bug.

## Proposed fix

1. Extract one builder. The signatures differ (`*config.Config` vs `*PluginContext`), so the shared
   piece is the smaller one: `(command string, files []string, projectName string) → (cmd, args)`.
2. In it, clear the seed when `len(parts) == 1`.
3. Reject a `command:` that splits to nothing — at config validation time, so it is a message and
   not a panic. `internal/config/validate.go` is where that belongs; the argv builder should then be
   able to trust its input.
4. Use `filepath.Join` everywhere.
5. Delete the other three.

Step 3 is worth doing even if 1 is deferred: a panic reaching the user is worse than the
duplication.

## Acceptance criteria

- [ ] The seed bug is reproduced before it is fixed | verify: `human — set 'command: podman-compose' in a fixture and record the argv DVA builds, via --dry-run or the debug log`
- [ ] A single-token command produces no stray `compose` | verify: `go test ./... -run 'ComposeArgs|ComposeArgv|BuildArgs' -v` — assert argv for `podman-compose` is exactly `podman-compose up …`
- [ ] A whitespace-only command does not panic | verify: `go test ./... -run 'ComposeArgs|ComposeArgv|BuildArgs' -v` — table cases `" "`, `"\t"`, `"''"`; each must be an error, not a panic
- [ ] Config validation rejects it | verify: `dva validate` against a fixture with `command: " "` — must exit non-zero with a message naming the field
- [ ] The four copies are one | verify: `grep -rn 'SplitCommand' internal/cli/compose.go internal/runner/compose.go internal/lifecycle/compose.go | wc -l` — print the count and state the target
- [ ] Path joining is uniform | verify: `grep -rn 'cfgDir + "/"' internal/ | wc -l` — must be 0
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-091](../done/091-compose-steps-stop-after-the-first-command.md) — the compose execution path
  audit that made these builders worth reading in the first place.
