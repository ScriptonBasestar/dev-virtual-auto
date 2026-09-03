---
id: TASK-280
title: "Name the live hookable command set in the JSON schema"
type: bug
priority: P2
effort: S
exec-tier: standard
created-at: 2026-09-03T15:10:00+09:00
source: "dva-48 finding surfaced while implementing TASK-273, independently verified against hookableCommands and internal/cli/validate.go"
scope: "internal/config/schema.json interaction_command.before description, internal/config/clean_hook_advice_test.go"
status: done
depends-on: []
---

# Task 280: name the live hookable set in the schema

## Summary

`schema.json` told config authors that `clean` accepts `before`/`replace`/`after` hooks. The
built-in was removed with the command-surface restructure (docs/43) and every Go rendering of
the hookable set was repaired at the time; the schema description was the one copy nobody
could reach from Go, so it kept advertising the removed command. This card corrects it and
binds it to the live set with a test.

## Problem

`definitions.interaction_command.properties.before.description` read:

```
Steps to run before a hookable built-in command (up, down, stop, restart, build, clean, logs)
```

`hookableCommands` (`internal/config/reserved.go:30`) holds six names — `build`, `down`,
`logs`, `restart`, `stop`, `up` — and `clean` is not among them. `internal/cli/validate.go`
records why in a comment: `clean` "was, until the command surface was restructured (docs/43);
teardown is `dva down <plan> --purge` now". `internal/config/hook_placement_test.go:41` pins
the user-facing error to the same six.

The two hook-placement error messages already avoid this drift by calling
`HookableCommandList()`, whose own comment says the literal list was removed precisely so that
changing the set could not leave a message advertising a name the same file rejects. JSON
cannot call that helper, so the schema description stayed a hand-written copy — and it is the
copy read most often, because a schema-aware editor shows it while the author is typing, before
any validation error exists to correct them.

## Completion Criteria

- [x] `interaction_command.before` describes the hookable set as the live six, with `clean` removed | verify: `/usr/bin/grep -q 'hookable built-in command (build, down, logs, restart, stop, up)' internal/config/schema.json`
- [x] A test derives the expected list from `HookableCommandList()` rather than restating it, so removing a command from `hookableCommands` fails on the schema too | verify: `go test ./internal/config -run TestSchemaDescriptionNamesTheLiveHookableCommands -count=1`
- [x] The test fails when the stale list is restored, rather than passing vacuously on a description that lost its parenthesised list | verify: `human — reviewer reinstates the old string, sees the failure name both lists, and reverts`
- [x] Repository gates pass | verify: `make lint && make test && make doc-check && make commit-check`

## Non-goals

- No change to `hookableCommands`, `HookableCommandList`, or hook placement validation. The
  code was already correct; only its description in the schema was not.
- No sweep of other schema descriptions. This one was found because a Go-side list of the same
  names existed to compare against; descriptions with no such counterpart are a different
  problem and are covered by PLAN-004's documentation-truth work.

## Evidence

| Claim | Command |
|---|---|
| `clean` is absent from the live hookable set | `/usr/bin/grep -A5 'var hookableCommands' internal/config/reserved.go` |
| The new test catches the stale list | `go test ./internal/config -run TestSchemaDescriptionNamesTheLiveHookableCommands -count=1` after reinstating it |
