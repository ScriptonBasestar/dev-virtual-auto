---
id: TASK-269
title: "Promote in-prose usage examples into cobra Example fields"
type: chore
priority: P2
effort: M
exec-tier: standard
created-at: 2026-09-03T00:56:00+09:00
source: "CLI discoverability audit, 2026-09-03 session (docs vs help gap review)"
scope: "cobra Example fields across lifecycle, run, doctor, init commands; restructure Long prose that currently embeds usage tables"
status: todo
depends-on: [TASK-268]
---

# Task 269: promote help examples into cobra Example fields

## Summary

Exactly one command sets cobra's `Example:` field (`composeCmd`, `internal/cli/compose.go:28`).
Every other command that shows example invocations embeds them inside its `Long:` prose as
hand-formatted "Plan usage:" / "Stack flags:" blocks (see `upCmd`, `compose.go:78`). The
information exists, but help output lacks the standard `Examples:` section, so shell
completion frameworks, help parsers, and LLMs reading `--help` structurally cannot
distinguish examples from description — and the hand-rolled flag tables in Long duplicate
what cobra's own `Flags:` section should render.

## Problem

1. `grep -n "Example:" internal/cli/*.go` (non-test) hits only `compose.go:28`; lifecycle
   commands (`up`, `down`, `stop`, `restart`, `build`, `logs`), `run`, `doctor`, and `init`
   render no `Examples:` section.
2. `upCmd`'s Long embeds flag documentation ("Plan usage:", "Stack flags:", "Plan-path
   flags:") as prose. Some of those flags are real cobra flags rendered again under `Flags:`,
   some are prose-only — the reader cannot tell which, and drift between the two surfaces is
   unchecked.
3. Each example should exist in exactly one surface: invocation examples in `Example:`,
   flag semantics on the flag's own usage string, conceptual behavior in `Long:`.

## Completion Criteria

- [ ] `up`, `down`, `stop`, `restart`, `build`, `logs`, `run`, `doctor`, `init` each set `Example:` with 2–5 representative invocations, rendered under cobra's `Examples:` heading | verify: `go test ./internal/cli -count=1`
- [ ] `upCmd`'s Long no longer hand-renders flag tables for flags cobra already lists; flag semantics move to the flags' usage strings, and prose-only pseudo-flags are either registered or removed from help | verify: `go test ./internal/cli -count=1`
- [ ] A regression test asserts the lifecycle commands and `run` have a non-empty Example, pinning the floor established here | verify: `go test ./internal/cli -count=1`
- [ ] Example invocations agree with USAGE.md's Command Quick Reference (spot-checked in review; no new generator) | verify: `make doc-check`
- [ ] Repository gates pass | verify: `make lint && make test && make doc-check`

## Non-goals

- No new Long content — TASK-268 owns Long coverage; this card only relocates/structures examples.
- No automated help↔USAGE.md consistency generator; that is a separate discovery if drift recurs.
- `manifest` usage_example fields (`internal/cli/manifest.go`) are out of scope — owned by TASK-267/263 work.
