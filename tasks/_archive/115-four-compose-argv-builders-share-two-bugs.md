---
id: TASK-115
title: "Four copies of the compose argv builder share the same two bugs: a `compose` seed that is never dropped, and an unguarded `parts[0]`"
type: fix
priority: P3
effort: M
status: done
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/cli/compose.go:787 buildComposeArgsForEntry, internal/cli/compose.go:821 buildComposeArgs, internal/runner/compose.go:16 composeArgv, internal/lifecycle/compose.go:149 (*ComposePlugin).buildArgs"
verified-at: 2026-08-03T14:45:00+09:00
archived-at: 2026-08-03T14:45:00+09:00
verification-summary: |
  All seven criteria re-measured against the current tree; nothing taken from metadata.
  Counts: SplitCommand in the three former builder files = 0; `cfgDir + "/"` in internal/ = 0.
  Scoped test binding is not vacuous — 57 RUN / 57 PASS / 0 FAIL, and the named regression
  subtests are present by name.
  Binary evidence (bin/dva v0.1.44, scratchpad fixtures, repo untouched):
    podman-compose fixture -> command=podman-compose args="[-f … up -d --wait]" (seed gone);
    `command: "   "` -> `dva up --dry-run` exit 1 with "compose runner: command: \"   \" contains
    no command word" (no panic), `dva compose up --dry-run` exit 1 same message, `dva validate`
    exit 1 naming stack.db.runners.compose.command; control fixture validate exit 0.
  The three tests that previously encoded bug A are re-anchored on a leading newline
  (internal/runner/step_keys_test.go:77,167 and internal/runner/inert_step_test.go:113), so a
  resurrected seed cannot pass as a substring.
  Both Related links resolve: tasks/done/119-*.md, tasks/_archive/091-*.md. The deliberately
  left-open fifth copy is closed — internal/cli/doctor.go:571 calls ComposeArgv. No task in
  tasks/todo, tasks/blocked, tasks/decision or tasks/plan references TASK-115.
  Swept remaining unguarded index-zero uses of split results: internal/runner/kubectl.go:108
  is strings.SplitN (always length >= 1), not a defect; no other SplitCommand caller indexes.
  `git status --porcelain` empty after verification.
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

- [x] The seed bug is reproduced before it is fixed | verify: `human — set 'command: podman-compose' in a fixture and record the argv DVA builds, via --dry-run or the debug log`
- [x] A single-token command produces no stray `compose` | verify: `go test ./... -run 'ComposeArgs|ComposeArgv|BuildArgs' -v` — assert argv for `podman-compose` is exactly `podman-compose up …`
- [x] A whitespace-only command does not panic | verify: `go test ./... -run 'ComposeArgs|ComposeArgv|BuildArgs' -v` — table cases `" "`, `"\t"`, `"''"`; each must be an error, not a panic
- [x] Config validation rejects it | verify: `dva validate` against a fixture with `command: " "` — must exit non-zero with a message naming the field
- [x] The four copies are one | verify: `decls=$(/usr/bin/grep -rn 'func SplitCommand' --include='*.go' . | wc -l | tr -d ' '); sites=$(/usr/bin/grep -rn 'SplitCommand(' --include='*.go' . | /usr/bin/grep -v _test.go | /usr/bin/grep -vc 'func SplitCommand'); echo "declarations=$decls call sites=$sites"; [ "$decls" -eq 1 ] && [ "$sites" -gt 1 ]` — **`declarations=1 call sites=8`, exit 0 (TASK-199).** The binding was `grep -rn 'SplitCommand' internal/cli/compose.go internal/runner/compose.go internal/lifecycle/compose.go | wc -l` — print the count and state the target. Neither the count nor the target was ever written down, so the command had no verdict attached to it and could not be wrong. It also measured the wrong thing: three named files cannot show that *one* implementation exists. Run at this card's archival commit `2065066` it prints **4**; run today it prints **0** — not because the three named files were renamed, since all three still exist, but because none of them calls `SplitCommand` any more. The 8 call sites are in 6 other files: `internal/exec/exec.go` (beside the one declaration), `internal/exec/compose_argv.go`, `internal/runner/runner.go`, `internal/cli/validate.go`, and `internal/runner/docker_compose.go` and `internal/runner/kubectl.go` with two each. With no target stated, 4 and 0 read alike. The invariant the criterion means is now asserted directly: exactly one declaration, reached from more than one place. Sabotaged by running the same text against a fixture tree holding a second `func SplitCommand` — `declarations=2 call sites=0`, exit 1
- [x] Path joining is uniform | verify: `files=$(/usr/bin/find internal -name '*.go' | wc -l | tr -d ' '); [ "$files" -gt 0 ] || { echo "no Go files under internal/ — nothing was measured"; exit 2; }; n=$(/usr/bin/grep -rn 'cfgDir + "/"' internal/ | wc -l | tr -d ' '); echo "occurrences=$n over $files files"; [ "$n" -eq 0 ]` — **`occurrences=0 over 257 files`, exit 0 (TASK-199).** The binding was `grep -rn 'cfgDir + "/"' internal/ | wc -l` — must be 0. "Must be 0" was stated in prose and enforced by nothing: a pipeline's status is its last command's and `wc -l` exits 0 on empty input. Worse, `grep` over a path that does not exist also prints `0`, so the stated target was satisfied by measuring nothing at all — the denominator is printed for that reason. Sabotaged in both directions against a fixture tree: `occurrences=1 over 3 files`, exit 1 with a `cfgDir + "/"` join present, and `no Go files under internal/ — nothing was measured`, exit 2, where the old form would have printed `0` and passed
- [x] Full suite passes | verify: `make test`

## Resolution

One builder, `exec.ComposeArgv` (`internal/exec/compose_argv.go`), now produces the shared prefix —
binary, `-f` flags, `--project-name` — and every caller appends its own tail. It returns an error, so
a `command:` that splits to nothing is a message rather than an index into nil.

### Measured, against the built binary

| fixture | command | before | after |
| --- | --- | --- | --- |
| `tmp/task-115` | `podman-compose` | `command=podman-compose args="[compose -f … up -d --wait]"` | `command=podman-compose args="[-f … up -d --wait]"` |
| `tmp/task-115b` | `"   "`, `dva up --dry-run` | `panic: index out of range [0] with length 0` at `compose.go:157`, exit 2 | `ERROR: entry "db" up failed: entry "db": compose runner: command: "   " contains no command word`, exit 1 |
| `tmp/task-115b` | `"   "`, `dva validate` | `✅ dva.yml is valid`, exit 0 | `[error] compose: stack.db.runners.compose.command: "   " contains no command word`, exit 1 |

`tmp/task-115` (`dva validate`) still exits 0 — the new check does not fire on a real command.

Bug B panicked under `--dry-run`, the mode whose whole purpose is to be safe to run.

### Counts

| measure | command | result |
| --- | --- | --- |
| copies remaining | `grep -rn 'SplitCommand' internal/cli/compose.go internal/runner/compose.go internal/lifecycle/compose.go \| wc -l` | **0** (target 0) |
| concatenated paths | `grep -rn 'cfgDir + "/"' internal/ \| wc -l` | **0** (required 0) |
| suite | `make test` | exit **0**, 5 packages ok |
| lint | `make lint` | exit **0**, 0 issues |

`internal/lifecycle/podman_compose.go:87` also joined with `+ "/" +` and now uses `filepath.Join`.
It is a separate config type, so it keeps its own small builder.

### Mutation testing

Each mutant was applied to the restored file, run, then reverted and confirmed identical with
`diff -q`.

| mutant | effect | killed by |
| --- | --- | --- |
| `args = fields[1:]` → `if len(fields) > 1 { … }` (bug A restored) | seed survives a single-token command | `TestComposeArgv`, `TestComposeArgv_SingleTokenCommandDropsSeed`, `TestStepWithoutRunIsReported`, `TestComposeKeysOnInteractionPath` |
| `if len(fields) == 0` → `if false` (bug B restored) | `panic: index out of range [0] with length 0` | `TestComposeArgvRejectsCommandWithoutAWord`, `TestComposeArgv_RejectsCommandWithoutAWord` |
| `detectUnrunnableComposeCommands` → `return nil` | `dva validate` goes back to exiting 0 | `TestDetectUnrunnableComposeCommands_Reports` |

### Two departures from the proposed fix

**Step 3 named the wrong home.** `internal/config/validate.go` cannot host this check:
`internal/exec` imports `internal/config`, so `config` importing `exec` is a cycle, and the answer
has to come from `SplitCommand` itself. Writing an approximation in `config` — say
`strings.Trim(s, " \t'\"") == ""` — was rejected on the same grounds the task itself argues: that
predicate disagrees with `SplitCommand` on `"' '"`, and two conditions that disagree about the same
string is precisely the defect being fixed. The check lives in `internal/cli/validate.go`
(`detectUnrunnableComposeCommands`), which may import both, and it calls the runners' own splitter.

**Three tests were asserting the bug.** `TestComposeKeysOnInteractionPath`,
`TestStepWithoutRunIsReported` and the compose-short-circuit case in `step_keys_test.go` all use
`composeConfig("echo")` — a single-token command — and expected the echoed argv to read
`compose up -d postgres`. That leading `compose` was bug A's output, written down as the
requirement. The expectations now anchor with a leading `\n`, so the assertion is that the line
*starts* with `up -d …`; a plain substring would let a resurrected seed pass.

### Left open, deliberately

`internal/cli/doctor.go:540` (`checkComposeConfigResolves`) is a fifth copy that hardcodes `docker`
and ignores `cc.Command` entirely, so `dva doctor` checks a tool the user is not running. Folding it
in also means changing its `exec.LookPath("docker")` skip logic, which is a different decision from
this one. Filed as [TASK-119](119-doctor-compose-check-ignores-the-configured-command.md).

## Related

- [TASK-091](../_archive/091-compose-steps-stop-after-the-first-command.md) — the compose execution path
  audit that made these builders worth reading in the first place.
- [TASK-119](119-doctor-compose-check-ignores-the-configured-command.md) — the fifth copy,
  in `dva doctor`.
