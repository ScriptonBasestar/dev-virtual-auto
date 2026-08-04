---
id: TASK-178
title: "`command:` as a list runs only its first line under the compose and kubectl runners"
type: bug
priority: P2
status: todo
effort: M
created-at: 2026-08-03T23:20:00+09:00
source: "TASK-175 self-review — the claim that every runner is total over every execution form was wrong"
scope: "dva repo — internal/runner/kubectl.go, internal/runner/docker_compose.go"
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

- [ ] `command:` as a list runs every line under the compose runner, or fails loudly. Decide
      which and say why. `EffectiveCommand`'s ` && ` join is one candidate; a loop like
      `executeSteps` is another, and they differ on what happens after a failing line — `&&`
      stops, and so does `ExecSequential`, so match `local` rather than picking freely.
      Verify: `human — the decision and its reasoning are in the Result section`
- [ ] The same under the kubectl runner, through the same decision rather than a second one.
- [ ] `--explain` stops reporting `Command: <first line>` for a list. It currently describes an
      execution that is wrong on three runners and incomplete on the fourth.
- [ ] The argv (or the sequence of argvs) is assertable without docker or a cluster — extend
      `DockerComposeRunner.executeArgs` and `KubectlRunner.execArgs` rather than adding a path
      neither can see.
      Verify: `go test ./internal/runner/ -count=1`
- [ ] `EffectiveCommand` is either used or deleted. A helper for this case with no callers is how
      the gap stayed invisible.
- [ ] Corpus: report how many configs under `examples/` declare a list `command:` on a
      non-local runner, including zero.
- [ ] `make test` exits 0.

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
