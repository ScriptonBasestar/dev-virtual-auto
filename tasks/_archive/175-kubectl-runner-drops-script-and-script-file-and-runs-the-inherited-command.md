---
id: TASK-175
title: "the kubectl runner discards script: and script_file:, the way it discarded steps: before TASK-094"
type: bug
priority: P2
effort: S
created-at: 2026-08-03T21:10:00+09:00
source: "TASK-149 — found while tracing which runners consume default_args"
scope: "dva repo — internal/runner/kubectl.go Execute"
status: done
quality-review: pass
quality-reviewed-at: 2026-08-07T18:05:08+09:00
verified-at: 2026-08-07T18:05:08+09:00
archived-at: 2026-08-07T18:05:08+09:00
quality-review-evidence: |
  - kind: test
    command-or-step: make test && make doc-check (mise go 1.26.4)
    result: exit 0; shared suite green
  - kind: recheck
    command-or-step: acceptance criteria re-observed
    result: kubectl script/script_file forms; KubectlExecArgs PASS
verification-summary: |
  quality-review pass; re-checked deliverables. kubectl script/script_file forms; KubectlExecArgs PASS. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 175: `pod:` plus `script:` runs neither the script nor an error

## Problem

`KubectlRunner.Execute` (`internal/runner/kubectl.go:17`) branches on exactly one execution
form:

```go
if len(r.Cmd.Steps) > 0 {
    return r.executeSteps(env, r.Cmd.Steps)
}
// …then unconditionally builds `kubectl exec <pod> -- <Command> <args>`
```

`Script` and `ScriptFile` appear nowhere in the file. So an interaction with `pod:` and
`script:` falls straight through to the `kubectl exec` path, which appends `r.Cmd.Command` —
empty for a script-only node, or the *parent's* command for a subcommand — and the declared
script is never mentioned again.

This is the defect [TASK-094](../_archive/094-kubectl-runner-discards-steps.md) closed
for `steps:`, in the same function, one branch short. Its comment even records the shape:
"Without this branch a config with `pod:` and `steps:` but no `command:` fell straight through
to the ExecReplace below, which appended nothing after `--`". Two of the four execution forms
were fixed; `script:` and `script_file:` were not.

`DockerComposeRunner.Execute` handles the same case explicitly — it falls back to `LocalRunner`
with the comment "script/script_file in docker context: not supported natively; fall back to
local execution as a convenience" — so kubectl is the outlier, and the precedent for what to do
already exists in a sibling file.

Two distinct outcomes, both wrong:

| config | what runs |
|---|---|
| top-level `pod:` + `script:`, no `command:` | `kubectl exec <pod> --` with nothing after it |
| subcommand `script:` under a parent with `command:` | the **parent's** command, via inheritance |

`dva validate` exits 0 for both: `hasExecutionTarget` counts `script:` as a target without
asking which runner would run it — the same blind spot TASK-094 named.

## Acceptance criteria

- [x] `pod:` + `script:` either runs the script or fails loudly. Decide which, and say why the
      compose runner's local-fallback precedent does or does not apply — running a script on the
      host when the user asked for a pod is a different thing than running it in a container.
      Verify: `human — the decision and its reasoning are in the Result section`
      *Runs it, in the pod, as `sh -c` — see "Decision" below.*
- [x] `script_file:` is covered by the same branch, not left as a third case.
- [x] The argv is assertable without a cluster. `Execute` calls `ExecReplace` inline, so there is
      no seam — split the argv construction out the way `DockerComposeRunner.executeArgs` was
      (TASK-132) and test it.
      Verify: `go test ./internal/runner/ -run Kubectl -count=1`
- [x] `steps:` behaviour is unchanged — TASK-094's test must still pass untouched.
      Verify: `go test ./internal/runner/ -run KubectlSteps -count=1`
- [x] Consider whether `warnUnreachableCommands` should also warn when the declared execution
      form cannot be run by the selected runner. Record the decision; a fix here without one
      leaves `dva validate` exiting 0 on the next such gap. *No warning — see "Criterion 5".*
- [x] `make test` exits 0.

## Related

- [TASK-174](174-explain-names-the-parents-command-for-a-child-that-runs-a-script.md) — the same
  inherited `Command`, misreported rather than mis-executed. If 174 stops the inheritance, the
  subcommand row above changes from "runs the parent's command" to "runs nothing", which is
  still this bug and still needs this fix.

## Result

### Reproduced first

A fixture with `pod: web` and each execution form, run against a fake `kubectl` on `PATH` that
records its argv. On `b59ab6d`, before the change:

| `dva run …` | argv |
|---|---|
| `podscript` (`script:`) | `kubectl exec --tty --stdin web --` |
| `podfile` (`script_file:`) | `kubectl exec --tty --stdin web --` |
| `rails scripted` (child `script:`) | `kubectl exec --tty --stdin web -- bundle exec rails` |
| `podcmd` (`command:`) — control | `kubectl exec --tty --stdin web -- bundle exec rails -e development` |
| `podsteps` (`steps:`) — control | `kubectl exec web -- sh -c echo step-one` |

`dva validate` exited 0 on that same file. Both wrong shapes the task predicted, confirmed: at
the top level nothing after `--`, and for a subcommand the parent's command in place of the
script.

### Decision: run it in the pod, as `sh -c`

**The compose precedent does not carry.** `DockerComposeRunner` falls back to `LocalRunner` for
`script:` "as a convenience", and a compose container shares the host the CLI is running on, so
the fallback lands somewhere adjacent. A pod is in a cluster that usually is not this machine. A
`script:` that migrates a database or writes a file would hit the developer's own database and
own filesystem — silently against the wrong target, which is the defect this task exists to fix,
relocated rather than removed. Failing loudly was the alternative, and it is worse than running
it: `kubectl exec <pod> -- sh -c <body>` is a thing kubectl does, so refusing would decline work
the tool can do.

**`sh -c`, not the shebang's interpreter.** The local path honours a shebang — `ExecScriptInline`
writes a temp file, prepends `#!/bin/sh` when absent, chmods and execs it — so honouring it in
the pod looks like the higher-fidelity choice. Two measurements say otherwise:

1. `schema.json` documents `script:` as an "Inline **shell** script" and `script_file:` as "an
   external **shell** script". `grep -rn "shebang\|#!/bin" USAGE.md docs/*.md` returns **0**. `sh`
   is the documented contract; the shebang is an artifact of needing a file to exec, not a
   promised feature.
2. It is the choice with **no silent-wrong mode**, and this inverts the intuition. A
   `#!/usr/bin/perl` body under `sh -c` dies on its first line, loudly. The same body under the
   shebang-honouring `perl -c` — the design that looks more faithful — runs a *syntax check*,
   prints nothing and exits 0. Fidelity to the shebang buys a silent success; `sh -c` cannot
   produce one, because every non-sh script is a syntax error to sh.

A bash-ism under `sh` fails the same loud way, so the cost of this choice is an error message,
never a wrong result.

### What changed in `kubectl.go`

- `execArgs()` — the one-shot argv, split out of `Execute` so it is observable without a cluster.
  `Execute` ends in `syscall.Exec`, which is why there was nothing to assert on before. Same seam
  `DockerComposeRunner.executeArgs` opened in TASK-132.
- `scriptBody()` — `script_file:` contents when set, else `script:`, matching LocalRunner's
  precedence. The file is read here because it lives on the host: its path means nothing on the
  other side of `kubectl exec`, so the contents have to travel instead. Relative paths resolve
  against `dva.yml`'s directory, as they do locally. An unreadable file is an error naming the
  path, not an empty body — `sh -c ""` exits 0, and a missing script reported as a successful run
  is this task's own failure shape.
- `execPrefix(tty bool)` — `exec [--tty --stdin] [--container c] <pod> --`, now shared with
  `buildStepArgs` instead of duplicated. The flag split is the distinction that already existed
  (one-shot gets the terminal, a step does not), not a third policy invented for scripts: it
  means `dva run podcmd` and `dva run podscript` differ only in what follows `--`.

Arguments are deliberately not appended to a script. A script consumes none locally either, and
since TASK-149 a subcommand declaring `script:` no longer inherits the parent's `default_args`.

After, on the same fixture: all three script rows become
`kubectl exec --tty --stdin web -- sh -c <body>`; both control rows byte-identical.

### Criterion 5: no `warnUnreachableCommands` warning, and the hole found while deciding

Building the runner × form table to justify the answer is what found the next bug. The table has
**five** columns, not four — `command:` also accepts a YAML list, kept in `CommandLines` — and it
is not full:

| | `steps:` | `script:` | `script_file:` | `command:` scalar | `command:` list |
|---|---|---|---|---|---|
| local | ✓ | ✓ | ✓ | ✓ | ✓ |
| compose | ✓ | ✓ via local | ✓ via local | ✓ | ✗ |
| kubectl | ✓ (TASK-094) | ✓ **this task** | ✓ **this task** | ✓ | ✗ |

`grep -c CommandLines` returns 0 for both `docker_compose.go` and `kubectl.go`. Measured:
`pod:` + `command: [echo one, echo two]` runs `kubectl exec … -- echo one` and drops the rest,
`--explain` prints `Command: echo one`, and `dva validate` says the file is valid. Filed as
[TASK-178](178-a-command-list-runs-only-its-first-line-on-every-runner-but-local.md), P2 — it
hits the compose runner too, which is the default for any `service:` interaction.

**Still no warning**, but for a different reason than the one this section was going to give. The
first draft argued the warning could never fire because the table was full; the table is not
full, so that argument was wrong and is recorded here rather than quietly replaced. The reason
that survives measurement: what the table shows is a bug to fix, not a state to warn about. A
warning would ship alongside the defect and need deleting when TASK-178 closes it, and it would
have to carry its own copy of this table — which is exactly the thing that drifted. If TASK-178
is declined, the warning becomes the fallback, and that is the point to revisit this.

What no warning addresses either way is a *sixth* form arriving with a kubectl branch missing.
That is a structural property of `Execute` — a chain of `if`s that falls through to the last one
rather than a switch that fails to compile — and TASK-178 carries the note. `validate` exiting 0
on a node with nothing to run is separately tracked as
[TASK-173](173-a-node-with-nothing-to-run-runs-sh-c-empty-and-reports-success.md).

### Test

`internal/runner/kubectl_script_test.go`, four tests. They assert `execArgs` rather than
`Execute`: `Execute` ends in `syscall.Exec`, and under `go test` the TASK-144 guard panics rather
than let it replace the test binary.

| test | pins |
|---|---|
| `…CoversEveryOneShotForm` (5 rows) | script in the pod; the `pod:container` qualifier; a script beside a `command:` not running the command; `script_file` > `script`; the command form untouched |
| `…SendsScriptFileContentsResolvedAgainstDvaYml` | contents travel, path resolves against `dva.yml` |
| `…ReportsAnUnreadableScriptFile` | a missing file is an error naming the path, not an empty body |
| `…SubcommandScriptDoesNotRunTheParentsCommand` | the reported defect, through `NewInteractionTree.Find` so the inheritance that produces it is part of the test |

The last one asserts `cmd.Command != ""` before the real assertion: the defect only exists because
the child inherits a command, so a fixture that stopped inheriting one would pass while testing
nothing.

### Falsification

Each revert was diffed against the original before running, and the harness now distinguishes
"no test failed" from "did not compile" — the first attempt at F3 dropped the only `fmt` use,
so the package stopped building and a grep for `--- FAIL` read that as a clean pass. A revert
that does not compile falsifies nothing.

| # | revert | failing tests |
|---|---|---|
| F1 | no script branch at all (the original defect) | 7 — every script test |
| F2 | `script_file:` ignored, inline `script` used instead | 3 — only the file-specific rows |
| F3 | unreadable `script_file` swallowed | **1** — `…ReportsAnUnreadableScriptFile` alone |
| F4 | one-shot path built with `execPrefix(false)` | 6, and the **only** one that breaks "the command form is untouched" |

F1 breaks nothing in `TestKubectlSteps*`, which is the disjointness that matters: TASK-094's
coverage never protected these forms, and this task's coverage does not overlap it. F3's single
failing test and F4's unique control row show the error path and the terminal-flag policy are
each pinned by something of their own rather than swept up by the branch test.

### Corpus

`dva ls --json` over all 19 files under `examples/`, "before" built from `git archive HEAD`:
**19 identical, 0 changed**. Expected, and the reason is worth recording rather than treating as
a null result: `grep -rln "script:\|script_file:" examples/` returns **0**, and only one example
declares `pod:` at all (`examples/kubernetes.yml`, command-form only). The shipped corpus never
exercised the broken path — which is a large part of why it stayed broken through TASK-094.
`examples/kubernetes.yml` was checked directly and is byte-identical.

### Three more adjacent defects filed

- **[TASK-176](176-explain-shows-a-blank-command-and-never-names-the-script.md)** — `--explain`
  on a script interaction prints a blank `Command:` and never mentions the script;
  `grep -n Script internal/runner/runner.go` finds only imports, so the plan has no equivalent of
  `explainSteps`. TASK-146's switch, one arm short. Pre-existing (measured against the `b59ab6d`
  binary), and this task makes it matter more: the plan now understates a real execution rather
  than an ignored one.
- **[TASK-177](177-pod-is-the-only-interaction-field-with-no-prose-anywhere.md)** — `grep -rn
  "pod:" USAGE.md docs/*.md` returns 0. This change had no document to land in, and writing a
  paragraph about `script:`-in-a-pod while `pod:` itself is undocumented would explain a detail
  of a feature the reader has never been told exists, so it was filed instead of half-written.
- **[TASK-178](178-a-command-list-runs-only-its-first-line-on-every-runner-but-local.md)** (P2) —
  a list `command:` runs only its first line on compose and kubectl. Found by self-review, after
  this task was already committed, while checking whether its own runner × form claim held. It
  did not. See "Criterion 5".

### Gates

| gate | result |
|---|---|
| `make test` | pass (`-race -cover`; `internal/runner` 69.2% → **72.3%**) |
| `go test ./internal/runner/ -run KubectlSteps -count=1` | pass — TASK-094 untouched |
| `make doc-check` | OK — 0 broken, 0 oversized |
| `gofmt -l internal/ cmd/ tools/` | clean |
| `go vet ./...` | exit 0 |
| `golangci-lint run ./internal/runner/...` | 0 issues |

`make lint` remains blocked by the pre-existing mise GOTOOLCHAIN drift in `tools/doccheck`,
unrelated to these files.

### Changed files

- `internal/runner/kubectl.go` — `execArgs`/`scriptBody`/`execPrefix`; `buildStepArgs` now shares
  the prefix instead of duplicating it.
- `internal/runner/kubectl_script_test.go` — new.
- `tasks/todo/176-*.md`, `tasks/todo/177-*.md`, `tasks/todo/178-*.md` — the adjacent gaps.
