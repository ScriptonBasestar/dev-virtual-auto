---
id: TASK-178
title: "`command:` as a list runs only its first line under the compose and kubectl runners"
type: bug
priority: P2
effort: M
created-at: 2026-08-03T23:20:00+09:00
completed-at: 2026-08-07
source: "TASK-175 self-review — the claim that every runner is total over every execution form was wrong"
scope: "dva repo — internal/runner/kubectl.go, internal/runner/docker_compose.go, internal/runner/execform.go, internal/runner/runner.go, internal/config/config.go"
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
    result: execEach/classifyForm; EveryRunnerRefuses PASS
verification-summary: |
  quality-review pass; re-checked deliverables. execEach/classifyForm; EveryRunnerRefuses PASS. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 178: Four of the five commands in a list are discarded, silently

## Problem

`command:` accepts a YAML sequence. `LocalRunner.Execute` honours it —
`if len(cmd.CommandLines) > 0 { return dvaexec.ExecSequential(...) }` at `local.go:42`, ahead of
the single-command branch. **No other runner reads the field**:

```
$ grep -c CommandLines internal/runner/docker_compose.go   # 0
$ grep -c CommandLines internal/runner/kubectl.go          # 0
```

`polymorphicCommand.UnmarshalYAML` (`config.go:405`) sets `scalar = lines[0]`, commented "first
line for display/backward-compat". Display is not where it ends up. For every runner but local,
that first line *is* the execution.

Measured on `b21cb65`, against a kubectl shim:

```yaml
podlist:
  pod: web
  command:
    - echo one
    - echo two
```

| | result |
|---|---|
| `dva run podlist` | `kubectl exec --tty --stdin web -- echo one` — `echo two` never runs |
| `dva run podlist --explain` | `Command: echo one` — the plan hides it too |
| `dva validate` | `✅ dva.yml is valid` |

Three chances to notice, none taken. The compose runner has the same hole by the same grep, so
this is not a kubectl corner: it is the default runner for a `service:` interaction.

`InteractionCommand.EffectiveCommand()` (`config.go:373`) already joins the lines with ` && ` for
exactly this situation and has **zero non-test callers** — the handling was written and never
wired up.

## Acceptance criteria

- [x] `command:` as a list runs every line under the compose runner, or fails loudly. Decide
      which and say why. `EffectiveCommand`'s ` && ` join is one candidate; a loop like
      `executeSteps` is another, and they differ on what happens after a failing line — `&&`
      stops, and so does `ExecSequential`, so match `local` rather than picking freely.
      Verify: `human — the decision and its reasoning are in the Result section`
- [x] The same under the kubectl runner, through the same decision rather than a second one.
- [x] `--explain` stops reporting `Command: <first line>` for a list. It currently describes an
      execution that is wrong on three runners and incomplete on the fourth.
- [x] The argv (or the sequence of argvs) is assertable without docker or a cluster — extend
      `DockerComposeRunner.executeArgs` and `KubectlRunner.execArgs` rather than adding a path
      neither can see.
      Verify: `go test ./internal/runner/ -count=1`
- [x] `EffectiveCommand` is either used or deleted. A helper for this case with no callers is how
      the gap stayed invisible.
- [x] Corpus: report how many configs under `examples/` declare a list `command:` on a
      non-local runner, including zero.
- [x] `make test` exits 0.

## Notes

Found while self-reviewing [TASK-175](../done/175-kubectl-runner-drops-script-and-script-file-and-runs-the-inherited-command.md),
whose Result section claimed every runner was total over "the four execution forms". There are
five. That claim was corrected rather than left standing, and this is the fifth.

This is the third instance of one shape: [TASK-094](../_archive/094-kubectl-runner-discards-steps.md)
(`steps:` on kubectl), TASK-175 (`script:`/`script_file:` on kubectl), and now a list `command:`
on both non-local runners. Worth considering, as part of this fix, whether `Execute` should be
made exhaustive over the execution forms by construction — a switch that will not compile when a
form is added, rather than a chain of `if`s that silently falls through to the last one. That
would end the series instead of shortening it by one more.

## Result

### Decision: one exec per line, stop on first failure (match local)

`EffectiveCommand`'s ` && ` join was **deleted**, not wired. Reasons:

1. **Local already sequences subprocesses** via `ExecSequential`, not a single shell `&&` string.
   Wiring ` && ` on compose/kubectl would make one dva.yml mean two things by runner.
2. **A joined string cannot share state the way people hope**: `cd build` then `make` as two list
   lines are two processes (and under compose/kubectl, two `exec`s). Pretending they are
   `cd build && make` describes an execution dva does not perform.
3. **Stop-on-failure** matches both `ExecSequential` and the existing `steps:` loop —
   first non-zero exit ends the interaction.

Compose and kubectl therefore reuse the same machinery as `steps:` (`execEach` → one
subprocess per line). Compose always uses `exec` for list lines (not the interaction's
`Compose.Method`): `run` would spin a fresh one-off container per line, so line two would not
see line one's side effects. Kubectl list invocations omit `--tty`/`--stdin` for the same
reason steps do.

### Implementation

| Piece | Role |
|-------|------|
| `internal/runner/execform.go` | Single `classifyForm` + `unhandledFormError` — fall-through becomes loud |
| `LocalRunner` / `DockerComposeRunner` / `KubectlRunner` `runForm` | Exhaustive switch on `execForm` |
| `eachArgs` on compose + kubectl | Observable multi-argv without docker/cluster |
| `Explain` | List → `Command: (N commands — see Commands below)` + every line; JSON `command_lines` |
| `EffectiveCommand` | Removed (was zero non-test callers) |

Go cannot make missing switch cases a compile error; `TestEveryRunnerRefusesAnUnhandledForm`
covers the class of silent fall-through that produced TASK-094 / 175 / 178.

### Corpus

`examples/` list `command:` sites: **0** (including zero on non-local runners).

### Verification

```
go test ./internal/runner/ -count=1   # ok
make test                            # exit 0
```

Code landed in `f3c7f47` (product fix); this card finalization is the disposition pass that
moves the task `doing → done` after re-verify on 2026-08-07.
