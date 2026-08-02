---
id: TASK-132
title: "--project-name is passed twice whenever config and detection agree"
type: bug
priority: P3
status: done
effort: S
created-at: 2026-08-02T00:00:00+09:00
resolved-at: 2026-08-02T00:00:00+09:00
resolution: "The runner stopped emitting its own flag and now hands the detected name to composeArgv, which substitutes it into the compose config it passes to ComposeArgv — the one place that writes --project-name. Precedence is stated where the flag is built instead of resting on argv order."
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

## Decision

The first option: the detected name is passed *into* the builder. `composeArgv` takes a
`projectOverride`, copies the compose config and substitutes `ProjectName` before calling
`dvaexec.ComposeArgv`, which stays the only place that writes the flag. The second option was
rejected because it makes `ComposeArgv` reason about what its callers are going to append — the
coupling TASK-115 created it to remove.

## Resolution

Three things turned up while implementing it that the original write-up did not anticipate.

**The override has to be conditional, and the empty case is the one that bites.** Applying it
unconditionally deletes a declared `project_name` whenever nothing was detected — which is most
invocations, since detection only fires on an already-running service. The obvious "just
overwrite" is a regression, and it is silent: compose falls back to the directory name.

**Copying the config is not a style preference.** `cfg` is shared by everything holding that
config, so writing the detected name through would let one interaction's detection result
become another's declared `project_name`. A shallow copy is enough — `ComposeArgv` only reads
`Files` and `Command` and writes nothing — but it is load-bearing, so it has its own test.

**`Execute` had no observable argv.** It assembled args inline and ended in `syscall.Exec`, so
there was no way to assert on what it built; that is a large part of why the duplication
survived. The assembly is now `executeArgs`, which is what the `execute` case below inspects.
`composeProfiles` must still run before `Method` is read — it rewrites `Method` to `"up"` — so
that ordering is stated in the new function rather than left as a property of statement order.

The measured pre-fix behaviour, same fixture and same session as the post-fix run:

```
pre-fix:   ... --project-name dva-task129 --project-name dva-task129 exec -e RAILS_ENV=test app ...
post-fix:  ... --project-name dva-task129 exec -e RAILS_ENV=test app ...
```

Both still print `test`, so TASK-129's forwarding is intact on both paths.

## Acceptance criteria

| # | Criterion | How it was verified | Result |
|---|---|---|---|
| 1 | The flag appears once on the `Execute` path when config and detection both supply a name | `TestProjectNameNotDuplicatedWhenDetectionAgreesWithConfig/execute` | pass |
| 2 | The flag appears once on the `steps:` path under the same conditions | `.../steps` | pass |
| 3 | The surviving value is the detected one, not the declared one | both cases assert the value, not only the count | pass |
| 4 | A declared `project_name` still reaches argv when nothing was detected | `TestProjectNameFallsBackToConfigWithoutDetection` | pass |
| 5 | A detected name reaches argv when config declares none | `TestProjectNameUsesDetectionWhenConfigDeclaresNone` | pass |
| 6 | The override does not write through to the shared config | `TestProjectOverrideDoesNotMutateConfig` | pass |
| 7 | No flag at all when there is neither a config nor a detection | `TestProjectNameAbsentWithoutConfigOrDetection` | pass |
| 8 | Each test fails when its own half of the fix is reverted | 4 mutation probes: re-add the append in `executeArgs`; re-add it in `buildStepArgs`; make the override unconditional; write through instead of copying | each broke exactly its own case; sources restored byte-identical |
| 9 | The duplicate is gone in a real invocation | `dva --debug run` against a running container, control binary rebuilt with the appends restored | 2 occurrences → 1 |
| 10 | Four gates | `make test` / `lint` / `doc-check` / `check-generate` | all exit 0 |

## Related

- [TASK-129](129-container-env-reaches-one-compose-path-out-of-four.md) — found while
  measuring the `steps:` path; the duplicated flag is visible in that task's e2e evidence.
- [TASK-133](133-container-detection-looks-in-a-different-project-than-execution.md) —
  the other half of project identity living in two places. Fixing this one made it visible:
  `detectedProject` is trustworthy only if detection asked about the right project, and it did
  not. Fixed there; whether `detectedProject` should survive at all is left open there too.
