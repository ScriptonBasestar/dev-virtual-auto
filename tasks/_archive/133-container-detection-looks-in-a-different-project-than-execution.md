---
id: TASK-133
title: "Running-container detection ignores files:, project_name: and command:"
type: bug
priority: P2
status: done
effort: M
created-at: 2026-08-02T00:00:00+09:00
resolved-at: 2026-08-02T00:00:00+09:00
resolution: "Detection routes through composeArgv, so it asks about the configured project with the configured files and binary. detectedProject keeps its shape; collapsing it to a boolean was considered and deliberately left out of a P2 bug fix."
scope: "internal/runner/docker_compose.go:267 (serviceRunningProject) against internal/exec/compose_argv.go"
verified-at: 2026-08-03T15:45:00+09:00
archived-at: 2026-08-03T15:45:00+09:00
verification-summary: |
  The fix is real and behaves as claimed, verified live (docker daemon WAS running, 29.2.1 /
  compose 5.1.4), not only from argv. Fixture with the compose file under `compose/` reached
  through `files:` and `project_name: dva-t133v`: bare `docker compose ps` still exits 1 there
  ("no configuration file provided"), while `dva --debug run whoami` now issues the ps query with
  `-f …/compose/docker-compose.yml --project-name dva-t133v`, rewrites method to `exec`, and
  prints 07f742e2f366 — the running `dva-t133v-app-1` itself — with zero `*-app-run-*` containers
  created. Stopping it flips back to `run --rm` (fresh container b06147e92617). `--project-name`
  appears exactly once on both lines.
  Criterion 6 was reproduced independently, not taken on trust: a `git archive HEAD` copy in the
  scratchpad with detectArgv reverted to the hardcoded bare argv fails exactly 3 of 4 cases and
  keeps WithoutConfig passing. `detectedProject` does still keep its shape (docker_compose.go:17,
  still a string) — the simplification really was not taken.
  bin/dva (v0.1.44, commit 15745f7) contains ec57eec; repo left untouched (`git status` shows only
  another agent's untracked tasks/todo/160-*.md), all fixture containers and networks torn down.
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

## Decision

Detection goes through `composeArgv` — the runner-side wrapper, not `dvaexec.ComposeArgv`
directly, so it picks up the same nil-config handling and debug logging every other invocation
gets. The query is now the shared prefix plus `ps --filter status=running --format {{.Project}}
<service>`, which fixes all three dropped fields at once. It is the same one-builder rule
TASK-132 applied to `--project-name`.

**`detectedProject` keeps its shape.** The open question was whether to collapse it to a
boolean, since detection now asks about the declared project and so can only report that same
name back. That reasoning holds, and the simplification is probably right — but it deletes what
TASK-132 had just added, on a P2 bug fix whose diff should be about the bug. Left as a separate
judgement, to be made on its own merits rather than as a side effect.

⚠️ The rationale above holds only when `project_name:` **is** declared. `dvaexec.ComposeArgv`
emits `--project-name` only under `if cc.ProjectName != ""`
(`internal/exec/compose_argv.go:56`), so a config with `files:` and no `project_name:` produces
no flag and compose infers the project from the directory; `composeArgv`
(`internal/runner/compose.go:31-44`) then writes the detected name into the copied config, which
makes `detectedProject` the *only* source of the flag on that path — a name that appears nowhere
in `dva.yml`. `internal/runner/compose_project_test.go:134`
(`TestProjectNameUsesDetectionWhenConfigDeclaresNone`) already pins that case, and collapsing
the field to a boolean would delete it. The deferral itself was also recorded only in this
paragraph: `detectedProject` had 0 hits across `tasks/todo|blocked|decision|plan`, so nothing
would have brought it back. Now filed as
[TASK-163](../decision/163-decide-whether-detectedproject-survives-as-a-name-or-collapses-to-a-flag.md).

An error from the query still means "not running". That is correct for a service that is simply
down, and it is also what happens when a configured non-docker compose binary rejects these
flags; falling back to `run` is the safe answer in both cases.

## Acceptance criteria

| # | Criterion | How it was verified | Result |
|---|---|---|---|
| 1 | Detection names the configured compose file | `TestDetectArgvCarriesTheConfiguredProject` | pass |
| 2 | Detection names the configured project | same case | pass |
| 3 | Detection runs the configured binary | `TestDetectArgvUsesTheConfiguredBinary` | pass |
| 4 | Detection passes no project override, so a stale value cannot leak into its own query | `TestDetectArgvPassesNoOverride` | pass |
| 5 | Without a config, detection stays the plain `docker compose ps` default | `TestDetectArgvWithoutConfig` | pass |
| 6 | The tests fail when the fix is reverted | probe: restore the hardcoded bare argv | 3 of 4 broke; case 5 correctly still passed, since old and new agree when there is nothing to configure |
| 7 | A running container reached through `files:` is exec'd into, not duplicated | e2e: container up as `dva-task133-app-1` (hostname `7e7b9282a02e`); `dva run whoami` printed `7e7b9282a02e`, and zero `*-app-run-*` containers were created | pass |
| 8 | A stopped service still falls back to `run` | same fixture with the container stopped: argv shows `run --rm`, a fresh container printed `e6e6ac5856a3` | pass |
| 9 | `--project-name` still appears once (TASK-132 holds on the new path) | both debug lines in the e2e carry it exactly once | pass |
| 10 | Four gates | `make test` / `lint` / `doc-check` / `check-generate` | all exit 0 |

The pre-fix control for criteria 7 is the measurement in the section above, taken on the same
fixture earlier in the same session: printed `ebf2997233b0` against a running `b27734163afa`,
with one throwaway container created.

## Related

- [TASK-132](132-project-name-is-passed-twice-on-the-detected-project-paths.md) — the other
  half of project identity being handled in two places; fixing that one made this one visible.
- [TASK-129](129-container-env-reaches-one-compose-path-out-of-four.md) — the
  environment now reaching the container is what makes "which container" matter.
