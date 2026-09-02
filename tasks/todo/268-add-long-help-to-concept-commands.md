---
id: TASK-268
title: "Add Long help to concept-bearing commands that only have Short"
type: chore
priority: P2
effort: S
exec-tier: standard
created-at: 2026-09-03T00:56:00+09:00
source: "CLI discoverability audit, 2026-09-03 session (docs vs help gap review)"
scope: "cobra Long fields on runCmd, lsCmd, statusCmd, provisionCmd, versionCmd; no behavior change"
status: todo
depends-on: []
---

# Task 268: add Long help to concept-bearing commands

## Summary

`dva --help` and the flagship lifecycle commands carry excellent Long help (`up` explains
default_plan resolution and even leading-`--` semantics), but several commands ship only a
`Short:` line. The worst gap is `dva run --help`: it prints one sentence plus flags, while the
concepts a first-time user needs — what an *interaction* is, why the `run` prefix can be
omitted, how subcommands and `--project` targeting resolve — live only in USAGE.md. The stated
product goal is that a user (human or LLM) can learn the tool from the CLI alone; these
commands break that.

## Problem

`grep -n "Long:" internal/cli/*.go` (non-test) shows Long help on root, compose, up/down/
stop/restart/build/logs, doctor, init, kubectl, show, console, config subcommands — and on
nothing else. Confirmed Short-only against the built binary:

1. `runCmd` (`internal/cli/run.go`) — the single most concept-heavy command has the thinnest help.
2. `lsCmd` (`internal/cli/list.go`) — does not say where the listed commands come from
   (`interaction:` in dva.yml, imported subprojects) or how it relates to `manifest`.
3. `statusCmd` (`internal/cli/status.go`) — does not say what is inspected or how plans scope it.
4. `provisionCmd` (`internal/cli/provision.go`) — "Execute the provisioning steps" with no
   pointer to the `provision:` section semantics.
5. `versionCmd` — trivial; include only if a Long adds anything (output format for scripting).

## Completion Criteria

- [ ] `dva run --help` explains: interaction commands come from `dva.yml`'s `interaction:` section, the `run` prefix is optional for non-reserved names, subcommand/default_args resolution in one sentence, and `--project` targeting; content agrees with USAGE.md rather than duplicating its detail | verify: `go test ./internal/cli -count=1`
- [ ] `lsCmd`, `statusCmd`, `provisionCmd` each carry a Long that states what the command reads, what it prints, and where the authoritative reference lives (USAGE.md section name) | verify: `go test ./internal/cli -count=1`
- [ ] A regression test asserts every command registered on rootCmd (excluding cobra built-ins `help`/`completion`) has a non-empty Long, so future commands cannot regress to Short-only | verify: `go test ./internal/cli -count=1`
- [ ] Repository gates pass | verify: `make lint && make test && make doc-check`

## Non-goals

- No `Example:` field work — TASK-269 owns promoting examples into cobra Example fields.
- No USAGE.md restructuring; Long text links to it, it stays canonical.
- No flag additions or behavior changes on any command.
