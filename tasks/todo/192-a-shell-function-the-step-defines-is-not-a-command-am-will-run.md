---
id: TASK-192
title: "A shell function the step defines is not a command am will run"
type: bug
priority: P1
effort: S
created-at: 2026-08-18T17:46:43+09:00
source: "TASK-191 — uncovered once the span that blocked first was removed"
scope: "dva repo — agent-mesh-flows/dva-improve-guided/00-analyze.yaml, agent-mesh-flows/dva-improve.yaml, tools/flowcheck"
status: todo
---

# Task 192: A shell function the step defines is not a command am will run

## Summary

Three fields define `yaml_block_keys()` and then call it. am's allowlist knows commands,
not functions the step just defined, so the call blocks:

```
blocked: shell policy: command "yaml_block_keys" not in allowlist
```

| field | file |
|---|---|
| `scan_project.context.compose_services` | `dva-improve-guided/00-analyze.yaml` |
| `scan_compose.context.root_compose` | `dva-improve.yaml` |
| `scan_compose.context.infra_compose` | `dva-improve.yaml` |

Both steps enumerate compose services. `scan_project` feeds the analysis report the whole
guided pipeline reads; `scan_compose` feeds `dva-improve`'s prompt. Neither has produced
its output since the function was introduced — the block was masked by an earlier
`comment-substitution` block until TASK-191 removed it, so the runs looked the same
before and after: successful, with a step that never ran.

## Completion Criteria

- [ ] No flow field calls a function it defines | verify: `go run ./tools/flowcheck`
- [ ] flowcheck fails on a field that defines and calls one | verify: `go test ./tools/flowcheck/`
- [ ] The three fields run and emit the same block keys as the function did | verify: human — extract each field and run it through `am` against a fixture with compose services, networks and volumes; compare against the function's output on the same fixture
- [ ] Flows still validate | verify: `am validate agent-mesh-flows/dva-improve.yaml && am validate agent-mesh-flows/dva-improve-guided/00-analyze.yaml`

## Technical Notes

- The function exists for a reason worth preserving: a flat indent grep conflates services
  with networks and volumes, so a network read as an undeclared service. Whatever replaces
  it has to keep emitting each top-level block's keys separately.
- Two shapes to weigh: inline the `awk` at each of the three call sites (repetition, but
  each site is one command), or have one `awk` invocation emit all three blocks in a
  labelled form the caller splits. The second is less repetition and more parsing.
- `~/.config/agent-mesh/sandbox_override.yaml` can add commands to the allowlist. It is
  user-local config, so a shipped flow cannot rely on it — the same reasoning as TASK-191.
- The flowcheck rule wants care: a function *definition* is harmless, and the defect is the
  *call*. Matching `name() {` and then looking for `name` in command position is the
  smallest thing that separates them.
