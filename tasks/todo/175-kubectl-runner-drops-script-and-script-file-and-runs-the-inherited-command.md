---
id: TASK-175
title: "the kubectl runner discards script: and script_file:, the way it discarded steps: before TASK-094"
type: bug
priority: P2
status: todo
effort: S
created-at: 2026-08-03T21:10:00+09:00
source: "TASK-149 — found while tracing which runners consume default_args"
scope: "dva repo — internal/runner/kubectl.go Execute"
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

- [ ] `pod:` + `script:` either runs the script or fails loudly. Decide which, and say why the
      compose runner's local-fallback precedent does or does not apply — running a script on the
      host when the user asked for a pod is a different thing than running it in a container.
      Verify: `human — the decision and its reasoning are in the Result section`
- [ ] `script_file:` is covered by the same branch, not left as a third case.
- [ ] The argv is assertable without a cluster. `Execute` calls `ExecReplace` inline, so there is
      no seam — split the argv construction out the way `DockerComposeRunner.executeArgs` was
      (TASK-132) and test it.
      Verify: `go test ./internal/runner/ -run Kubectl -count=1`
- [ ] `steps:` behaviour is unchanged — TASK-094's test must still pass untouched.
      Verify: `go test ./internal/runner/ -run KubectlSteps -count=1`
- [ ] Consider whether `warnUnreachableCommands` should also warn when the declared execution
      form cannot be run by the selected runner. Record the decision; a fix here without one
      leaves `dva validate` exiting 0 on the next such gap.
- [ ] `make test` exits 0.

## Related

- [TASK-174](174-explain-names-the-parents-command-for-a-child-that-runs-a-script.md) — the same
  inherited `Command`, misreported rather than mis-executed. If 174 stops the inheritance, the
  subcommand row above changes from "runs the parent's command" to "runs nothing", which is
  still this bug and still needs this fix.
