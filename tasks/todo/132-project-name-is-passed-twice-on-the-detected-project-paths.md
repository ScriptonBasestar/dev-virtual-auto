---
id: TASK-132
title: "--project-name is passed twice whenever config and detection agree"
type: bug
priority: P3
status: todo
effort: S
created-at: 2026-08-02T00:00:00+09:00
scope: "internal/runner/docker_compose.go:46,95 against internal/exec/compose_argv.go:56-57"
---

# Task 132: `--project-name` is emitted twice

## What happens

Two places add the flag and neither knows about the other:

- `dvaexec.ComposeArgv` adds it from the config's `project_name`
  (`internal/exec/compose_argv.go:56-57`).
- `DockerComposeRunner` adds it again from `r.detectedProject`, once in `Execute`
  (`docker_compose.go:46`) and once in `buildStepArgs` (`:95`).

`detectedProject` is set by `autoDetectComposeMethod` from `docker compose ps`, so it is
populated exactly when the service is already running — the common dev-loop state. If the
config also declares `project_name`, both fire.

Observed while verifying TASK-129, in a failing step's error message:

```
docker compose -f .../docker-compose.yml --project-name dva-task129 --project-name dva-task129 exec app sh -c printenv RAILS_ENV
```

## Why it is P3 and not P1

Docker takes the last occurrence, and the last occurrence is `detectedProject` — which is the
value that should win, since it is what the running container actually belongs to. So the
behaviour is correct today and the defect is that the command line says so twice.

It is worth fixing anyway because the duplication is user-visible in error output, and because
the correctness rests on an argv-ordering coincidence rather than on either caller deciding.
The moment something reorders those appends, or a caller inserts the detected name before the
config one, the wrong project silently wins.

## What needs deciding

Whether the runner should suppress `ComposeArgv`'s copy (pass the detected name *into* it), or
whether `ComposeArgv` should stop emitting the flag when the caller will supply one. The first
keeps the precedence in one place; the second keeps `ComposeArgv` free of runner concepts.

## Verification

There is no test on this today. A fix wants one asserting the flag appears exactly once, on
both the `Execute` and `buildStepArgs` paths, with a config `project_name` and a detected
project both set — and it must fail when the fix is reverted.

## Related

- [TASK-129](../done/129-container-env-reaches-one-compose-path-out-of-four.md) — found while
  measuring the `steps:` path; the duplicated flag is visible in that task's e2e evidence.
