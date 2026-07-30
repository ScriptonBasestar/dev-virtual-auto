---
id: TASK-081
title: "`dva show` names plans and interactions but never a stack entry, so the answer to \"what is declared\" needs two commands"
type: fix
priority: P4
status: todo
effort: S
created-at: 2026-07-30T00:00:00+09:00
scope: "internal/cli — show.go (stack section) and the ls/show/status division of labour"
---

# Task 081: One command that names everything the config declares

## Problem

This task exists to correct a claim, so the correction comes first.
[TASK-074](../done/074-app-subcommands-answer-an-absent-section-three-ways.md) recorded that
"no command answers *what does this config declare?*". **That is false**, and it was filed
without being checked. Two commands answer it:

```
$ dva show                       # stack-only fixture, 1 stack entry + 1 interaction
DVA v0.1.44
Config: …/dva.yml
  Required version: 0.1.44

Compose:
  Files:   compose.yml

Interaction Commands: 1 defined
  hello  say hello

$ dva status
Lifecycle:

  [infra] script
```

What is actually wrong is narrower. On the stack-only fixture `dva show` reports the entry's
*runner* as a heading — `Compose:` with the compose files under it — and never prints `infra`,
the name every other command takes as its argument. On a config with plans it prints
`Plans (dva up <name>):` with both plan names and then stops, printing no stack section at all.
`dva status` is the only command that names stack entries, and it presents them as runtime state
rather than as declarations.

So the answer is split: `show` for plans and interactions, `status` for stack entry names, and
neither is complete on its own.

## Evidence (verified 2026-07-30)

Two fixtures against `bin/dva` 0.1.44, both `✅ dva.yml is valid`:

| fixture | `dva show` names | `dva status` names |
| --- | --- | --- |
| 1 stack entry (`infra`, compose) + 1 interaction | `Compose:`, `compose.yml`, `hello` | `[infra] compose` |
| 1 stack entry (`infra`, script) + 2 plans | `ci`, `local-dev` | `[infra] script` |

In the second fixture `dva show` printed no stack section whatsoever.

`dva ls` is a third surface and lists `interaction:` only — verified: on the first fixture it
printed `hello  # say hello` and nothing about `infra`. That is a naming problem more than a
coverage one, and it has its own users, so it is out of scope here.

## Why it matters

P4 because nothing is broken and both commands are truthful about what they do print. It matters
at all because a user (or a flow) trying to learn a project's shape has to know in advance which
of three commands holds which section, and `show` claiming to be the
"registered configuration summary" while omitting the stack is the part that misleads.

It also weakens the message
[TASK-074](../done/074-app-subcommands-answer-an-absent-section-three-ways.md) shipped: that
message inlines its own counts (`stack (1), interaction (1)`) precisely because it could not
point at a command that lists them, which is a fourth place the same knowledge is now written.

## Fix shape

Give `show` a stack section that names entries and their runners, on every config that declares
one, next to the plans and interaction sections it already has. Then the TASK-074 message can
route to it instead of carrying counts of its own.

## Non-goals

- Do not rename or change `dva ls`. Listing interactions only is surprising, but it is a
  separate contract.
- Do not merge `show` and `status`. One describes declarations, the other runtime state, and
  that division is right.
- Do not add a new top-level command. `show` already claims this job.

## Acceptance criteria

- [ ] `dva show` names every stack entry and its runner | verify: `go test ./internal/cli/ -run TestShowNamesStackEntries`
- [ ] It does so on a config that also declares plans | verify: `human — dva show on the 2-plan fixture prints both the plans and the infra entry`
- [ ] The existing sections are unchanged | verify: `go test ./internal/cli/ -run TestShow`
- [ ] A config declaring nothing still prints something truthful | verify: `human — dva show on an empty config names no section it does not have`
- [ ] Full suite passes under -race | verify: `make test`
