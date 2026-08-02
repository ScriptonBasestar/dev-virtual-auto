---
id: TASK-129
title: "Host env reaches a container on exactly one of four compose paths, and nothing records whether that is the design"
type: decision
priority: P2
status: decision
effort: M
created-at: 2026-08-02T00:00:00+09:00
scope: "internal/runner/docker_compose.go — runVars:189, composeArguments:147-150, buildStepArgs:105, autoDetectComposeMethod:204; internal/runner/kubectl.go for the comparison"
---

# Task 129: Decide what `-e` forwarding means on an `exec`

## The blind spot

`docker compose` is handed `-e KEY=VALUE` in exactly one place: `composeArguments` calling
`runVars`, guarded by `if method == "run"` (`docker_compose.go:147-150`). Everything else that
runs a container command builds argv without it.

That guard reads `r.Cmd.Compose.Method` *after* `Execute` has already called
`autoDetectComposeMethod` (`docker_compose.go:37`), which rewrites `run` to `exec` whenever the
service is already up (`:215`). So the effective matrix is:

| path | argv verb | `-e` injected |
|---|---|---|
| non-step, `method: run`, container **not** running | `run` | **yes** |
| non-step, `method: run`, container already running | `exec` (rewritten) | no |
| non-step, `method: exec` as configured | `exec` | no |
| `steps:` item, any config | `exec` (`buildStepArgs:105`) | no |

One row out of four. The common dev-loop state — container already up — is not that row.

`env` does reach the docker CLI child process on every path, identically: both `ExecReplace` and
`ExecSubprocess` set the child's environment from `env.EnvSlice()`
(`internal/exec/exec.go:28,52`). But the docker CLI's own environment is not the container's.
`-e` is the only per-invocation mechanism that crosses that boundary, and `docker compose exec`
does support it (verified against Docker 29.5.3 on this machine: `-e, --env stringArray`, same
flag as `run`).

## Why this is a decision and not a bug report

There is a coherent reading in which the current behaviour is correct: `run` creates a fresh
container, so `-e` is how you parameterise its creation; `exec` enters a container that already
has the environment it was built with, and injecting host values into it is a per-invocation
override the tool deliberately does not perform.

There is an equally coherent reading in which it is a defect: the same `dva` command changes what
the process inside the container can see depending on whether someone happened to leave the stack
running.

Nothing in the repo records which was intended, so no one can call the current behaviour right or
wrong — which is the actual problem.

`runVars` has a second filter that belongs to the same question: it skips any variable not
already present in the host OS environment (`os.Getenv(k) == ""`, `:196`), and says so in its own
comment. A variable declared only in `dva.yml`'s `environment:` therefore reaches
`env.EnvSlice()` and the docker CLI, and is then dropped from `-e`. So even the one row that
forwards, forwards only host-exported values — config-declared ones never cross into the
container at all. That is documented, deliberate, and surprising.

## The three separable questions

1. **Should `exec` forward at all?** If yes, `buildStepArgs` and the rewritten-`run` case both
   need it, and `buildStepArgs` regains an `*config.Environment` parameter it was deliberately
   stripped of.
2. **Should `autoDetectComposeMethod`'s rewrite change what the command sees?** Today a
   background `docker compose up` silently changes the environment of a later `dva run`. Whatever
   is decided for (1), the answer should not depend on container uptime.
3. **Should `environment:` from `dva.yml` reach the container?** This is independent of (1) and
   (2) and arguably the sharpest of the three: the user wrote the variable in dva's own config
   file, and dva does not pass it in.

## The cost that needs deciding with it

Forwarding is a user-visible behaviour change to every `steps:` and `exec` invocation, not a pure
bugfix — commands that today see only the container's baked-in environment would start seeing
host values. It wants a changelog entry and probably a note in `docs/`.

Nothing in the test suite constrains the answer: **0 test files** in `internal/runner/` reference
`runVars`, `composeArguments`, or `buildStepArgs`. `compose_steps_test.go` and
`kubectl_steps_test.go` assert execution *order* and marker substrings, never argv content. So
whichever way this goes, it needs new tests — there are none to break and none to lean on.

## Comparison: kubectl is consistent and consistently silent

`KubectlRunner` has no env-forwarding on either path — `kubectl.go` contains zero `-e`/`--env`
argv logic, step or non-step. It has the broader limitation without the asymmetry. A decision to
forward on compose raises the question for kubectl; a decision not to forward makes kubectl the
model. Either way it should not be answered twice.

## Related

- [TASK-128](../done/128-the-recursion-was-right-the-nodes-it-walked-were-not.md) — found this
  while correcting `buildStepArgs`' doc comment, and its first correction of that comment
  overstated the shape: it framed the split as step vs non-step when the measured split is
  fresh-container `run` vs everything else. Corrected there; the accurate framing is the table
  above.
