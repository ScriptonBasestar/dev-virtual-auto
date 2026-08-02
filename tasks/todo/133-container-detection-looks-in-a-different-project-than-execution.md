---
id: TASK-133
title: "Running-container detection ignores files:, project_name: and command:"
type: bug
priority: P2
status: todo
effort: M
created-at: 2026-08-02T00:00:00+09:00
scope: "internal/runner/docker_compose.go:267 (serviceRunningProject) against internal/exec/compose_argv.go"
---

# Task 133: detection and execution look at different projects

## What happens

`serviceRunningProject` decides whether a service is already up by running

```go
dvaexec.ExecSubprocessOutput("docker", "compose", "ps", "--filter", "status=running",
    "--format", "{{.Project}}", service)
```

with no `-f`, no `--project-name`, and a hardcoded `docker`. Every other compose invocation in
the codebase goes through `dvaexec.ComposeArgv`, which supplies all three from config. So the
question "is this service running?" is asked about a different project than the one the answer
is used on.

Three config fields are dropped:

| field | execution uses | detection uses |
|---|---|---|
| `files:` | the declared paths | whatever compose file is in the CWD, if any |
| `project_name:` | the declared name | the CWD directory name |
| `command:` | e.g. `podman-compose` | always `docker` |

`autoDetectComposeMethod` turns an empty answer into "not running", so a false negative is
silent: `method: run` stands, and `dva run` creates a throwaway container instead of exec'ing
into the one that is up.

## Measured

Fixture: a compose file at `compose/docker-compose.yml` reached through `files:`, with
`project_name: dva-task133`. Container started and confirmed running.

```
running container:  dva-task133-app-1        hostname b27734163afa

# what serviceRunningProject runs, from the project dir:
$ docker compose ps --filter status=running --format '{{.Project}}' app
no configuration file provided: not found          (exit 1)

# what the user gets:
$ dva --debug run whoami
[debug] compose: docker compose -f .../compose/docker-compose.yml \
        --project-name dva-task133 run --rm app sh -c hostname
 Container dva-task133-app-run-44a58f271a08 Creating
ebf2997233b0
```

`b27734163afa` was running the whole time; the command ran in `ebf2997233b0`. Detection
returned nothing because the bare `ps` could not find a compose file, so nothing reported a
problem — the command succeeded, in the wrong container.

The same fixture with the compose file at the project root does detect, which is why
TASK-129's e2e worked and why this went unnoticed: the default layout hides it.

## Why it is P2

It is not a cosmetic mismatch. A dev loop's whole premise is that `dva run` reaches the
container holding the installed dependencies, the warm caches and the mounted work. Instead it
gets a fresh one from the image, silently, and `--rm` deletes the evidence. Anything the
command was supposed to observe — a migration applied a minute ago, a file written by a
previous step — is simply absent.

TASK-129's `-e` forwarding still works here; it just lands on the wrong container, which makes
the environment look correct while the state is not.

Not P1 only because the failure needs a non-default compose file location, a `project_name`
that differs from the directory name, or a non-docker compose binary.

## What needs deciding

`ComposeArgv` returns `(cmd, args, error)` for a *prefix*, so detection can reuse it directly:
`args = append(prefix, "ps", "--filter", ...)`. That fixes all three fields at once and is the
same one-builder rule TASK-132 applied to `--project-name`.

The open question is what to do about the resulting `--project-name`: once detection asks about
the declared project, the answer it returns is that same declared name, so `detectedProject`
stops carrying new information and collapses into "is it running, yes/no". That is arguably the
right shape — but it changes what TASK-132's override means, so the two want reading together.

## Verification

A fix wants a test that detection's argv carries the configured files, project name and binary
— and it must fail when the fix is reverted. The behavioural half needs the fixture above:
a running container reached through `files:`, and an assertion that the interaction exec'd into
it rather than creating a second one. Comparing `hostname` output is enough, as measured here.

## Related

- [TASK-132](../done/132-project-name-is-passed-twice-on-the-detected-project-paths.md) — the other
  half of project identity being handled in two places; fixing that one made this one visible.
- [TASK-129](../done/129-container-env-reaches-one-compose-path-out-of-four.md) — the
  environment now reaching the container is what makes "which container" matter.
