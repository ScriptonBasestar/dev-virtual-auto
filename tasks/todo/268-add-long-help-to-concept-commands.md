---
id: TASK-268
title: "Add Long help to concept-bearing commands that only have Short"
type: chore
priority: P2
effort: M
exec-tier: standard
created-at: 2026-09-03T00:56:00+09:00
source: "CLI discoverability audit, 2026-09-03 session (docs vs help gap review); scope corrected 2026-09-03 after a full rootCmd tree walk"
scope: "cobra Long fields on the 21 Short-only commands in the rootCmd tree (see Problem inventory) plus a regression test; no behavior change"
status: todo
depends-on: []
---

# Task 268: add Long help to concept-bearing commands

## Summary

`dva --help` and the flagship lifecycle commands carry excellent Long help (`up` explains
default_plan resolution and even leading-`--` semantics), but 21 of the commands in the tree
ship only a `Short:` line. The worst gap is `dva run --help`: it prints one sentence plus flags, while the
concepts a first-time user needs — what an *interaction* is, why the `run` prefix can be
omitted, how subcommands and `--project` targeting resolve — live only in USAGE.md. The stated
product goal is that a user (human or LLM) can learn the tool from the CLI alone; these
commands break that.

## Problem

A recursive walk of `rootCmd` (in-package test that prints every command whose `Long` is
empty) finds **21** Short-only commands, not the five a `grep -n "Long:" internal/cli/*.go`
sweep suggests — grep tells you which *file* mentions `Long:`, not which command in it carries
one. `internal/cli/skill.go` and `internal/cli/ssh.go` define eleven commands between them and
none has a Long; `internal/cli/validate.go` defines `validateCmd` with no Long at all.

Concept-bearing (the reason this task exists — each hides a `dva.yml` section or a resolution
rule that only USAGE.md explains):

1. `runCmd` (`run.go`) — the most concept-heavy command has the thinnest help: what an
   *interaction* is, why the `run` prefix is optional, subcommand/`default_args` resolution,
   and `--project` targeting are all absent.
2. `lsCmd` (`list.go`) — does not say where the listed commands come from (`interaction:` in
   dva.yml, imported subprojects) or how it relates to `manifest`.
3. `statusCmd` (`status.go`) — does not say what is inspected or how plans scope it.
4. `provisionCmd` (`provision.go`) — "Execute the provisioning steps" with no pointer to the
   `provision:` section semantics.
5. `validateCmd` (`validate.go:129`) — reached as both `dva validate` and `dva config
   validate`; neither explains what is checked or what `--strict` / `--fix` change.
   Note `validate_alias.go:9` copies `validateCmd.Long` **by value inside `init()`**, so the
   Long must be set in the struct literal — assigning it from a later `init()` would leave the
   top-level alias empty while `dva config validate` looks fixed.
6. `manifestCmd` (`manifest.go`) — the LLM-facing surface; needs to say what the manifest
   contains and that it is the machine-readable twin of `ls`.
7. `skillCmd` and `sshCmd` — group parents whose `--help` currently lists subcommands with no
   statement of what the group is for (which AI runtimes; which agent container).

Mechanical (no concept to explain, but the regression test below covers them, which is what
turns this from an S into an M):

8. `versionCmd`, `configCmd`, `consoleCmd`, `console start`.
9. `skill install`, `skill status`, `skill uninstall`, `skill backup`, `skill backup list`.
10. `ssh up`, `ssh down`, `ssh status`.

Full missing list, as printed by the tree walk: `config`, `config validate`, `console`,
`console start`, `ls`, `manifest`, `provision`, `run`, `skill`, `skill backup`,
`skill backup list`, `skill install`, `skill status`, `skill uninstall`, `ssh`, `ssh down`,
`ssh status`, `ssh up`, `status`, `validate`, `version`.

## Completion Criteria

- [ ] `dva run --help` explains: interaction commands come from `dva.yml`'s `interaction:` section, the `run` prefix is optional for non-reserved names, subcommand/default_args resolution in one sentence, and `--project` targeting; content agrees with USAGE.md rather than duplicating its detail | verify: `go test ./internal/cli -count=1`
- [ ] `lsCmd`, `statusCmd`, `provisionCmd`, `manifestCmd`, `validateCmd` each carry a Long that states what the command reads, what it prints, and where the authoritative reference lives (USAGE.md section name); `validateCmd`'s Long is set in its struct literal so `dva validate` and `dva config validate` both show it | verify: `go test ./internal/cli -count=1`
- [ ] `skillCmd` and `sshCmd` carry a Long stating what the group manages, and every one of their subcommands plus `configCmd`, `consoleCmd`, `console start`, `versionCmd` has at least a one-line Long | verify: `go test ./internal/cli -count=1`
- [ ] A regression test walks the rootCmd tree recursively (not just direct children) and asserts every command has a non-empty Long, so future commands cannot regress to Short-only; cobra adds `help`/`completion` at Execute time so they do not appear in the walk and need no special-casing — if the test is written to run after `Execute`, exclude them explicitly | verify: `go test ./internal/cli -count=1`
- [ ] Repository gates pass | verify: `make lint && make test && make doc-check`

## Non-goals

- No `Example:` field work — TASK-269 owns promoting examples into cobra Example fields.
- No USAGE.md restructuring; Long text links to it, it stays canonical.
- No flag additions or behavior changes on any command.
