---
id: TASK-163
title: "Decide whether detectedProject survives as a name or collapses to a boolean"
type: decision
priority: P4
status: todo
effort: S
created-at: 2026-08-03T15:40:00+09:00
source: "TASK-133 finalize verification — the simplification it deferred, filed so the deferral is tracked rather than only narrated"
depends-on: [TASK-132, TASK-133]
scope: "dva repo — internal/runner/docker_compose.go:17/:39/:74/:253, internal/runner/compose.go:23-56"
---

# Task 163: The deferred `detectedProject` simplification

## What was deferred

[TASK-133](../_archive/133-container-detection-looks-in-a-different-project-than-execution.md)
fixed detection so it asks about the project the interaction will actually execute against, and
left one question open on purpose: now that detection asks about a known project, does
`detectedProject` still need to carry a *name*, or can it collapse to a "a container is already
running" boolean?

Deferring it was right — it deletes what [TASK-132](../_archive/132-project-name-is-passed-twice-on-the-detected-project-paths.md)
had just added, on a P2 bug fix whose diff should be about the bug. But the deferral lived only
in a closing paragraph. `grep -rl detectedProject tasks/todo tasks/blocked tasks/decision tasks/plan`
returned **0** before this file existed, so nothing would have brought it back.

## The measurement that decides it

The rationale TASK-133 recorded — detection "can only report that same name back" — holds only
when `project_name:` is declared. When it is not:

- `dvaexec.ComposeArgv` emits `--project-name` only under `if cc.ProjectName != ""`
  (`internal/exec/compose_argv.go:56`), so a config without `project_name:` produces no flag and
  compose infers the project from the directory.
- `composeArgv` (`internal/runner/compose.go:31-44`) then copies the config and writes
  `override.ProjectName = projectOverride`, so on the detected path the **detected** name is the
  only source of the flag.

So in that case detection reports a name that appears nowhere in `dva.yml`, and it reaches
execution solely through `detectedProject`. A boolean would drop it.
`internal/runner/compose_project_test.go:134`
(`TestProjectNameUsesDetectionWhenConfigDeclaresNone`) already pins exactly this case, and the
proposed simplification would delete that test.

## The decision

- **A — keep the name.** Correct for the undeclared-`project_name:` case above; the cost is one
  string field that is redundant whenever `project_name:` *is* declared.
- **B — collapse to a boolean**, and make the undeclared case explicit some other way (for
  instance by resolving the project name once at config load, so there is only ever one source).
  Larger, and it changes what `ComposeArgv` means.
- **C — keep the name and record why**, so the next reader does not re-derive the
  undeclared-`project_name:` case from scratch. The field currently carries no comment saying it
  is load-bearing only in that case.

C is the cheap answer and probably the right one; B is the only one that actually removes the
redundancy, and it is not a P4-sized change.

## Acceptance criteria

- [ ] The decision is recorded with the undeclared-`project_name:` case named explicitly, since
      that is the case that makes the field non-redundant.
- [ ] If B is chosen: `TestProjectNameUsesDetectionWhenConfigDeclaresNone` is replaced by
      something that still pins the behaviour, not deleted. State what covers it afterwards.
- [ ] Either way, re-measure both fixtures — one declaring `project_name:`, one with `files:`
      only — and paste the exec line each produces. The claim is about which argv reaches
      docker, so the argv is the evidence.
- [ ] `make test` and `make lint` exit 0.

## Related

- [TASK-132](../_archive/132-project-name-is-passed-twice-on-the-detected-project-paths.md) —
  added the single-builder rule and the test this would delete. If `detectedProject`'s shape is
  132's to own, this decision belongs to 132's line of work.
- [TASK-133](../_archive/133-container-detection-looks-in-a-different-project-than-execution.md) —
  deferred this, and stated the rationale in terms that hold only for declared project names.
  Corrected inline there.
