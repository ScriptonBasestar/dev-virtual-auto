---
id: TASK-096
title: "`dva manifest` documents 13 of 27 commands, and its doc comment says the audience is LLMs"
type: fix
priority: P3
effort: S
status: todo
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/cli/manifest.go:103-124 — StaticCommands, a hand-maintained literal; internal/config/reserved.go:12-20 — the 27-entry list it should agree with"
---

# Task 096: the machine-readable command list is a hand-copied subset

## Measured

```
$ dva manifest --format json | jq '.static_commands | length'
13
```

against 27 real top-level commands (`reservedCommands`, `internal/config/reserved.go:12-20`,
which `USAGE.md:617-623` also documents as 27).

Present: `run, ls, compose, up, down, stop, build, clean, provision, validate, manifest, ktl,
version`.

**Missing (14):** `help, ssh, infra, console, completion, init, status, config, logs, restart,
show, doctor, app, stack`.

The mismatch is one-directional — `StaticCommands ⊂ reservedCommands`, no phantom entries — which
is the signature of a list that was written once and not updated as commands were added.

## Why it matters

`manifest.go`'s own doc comment states the output is for LLMs. An agent that reads
`static_commands` to decide what dva can do concludes there is no `dva doctor`, no `dva status`,
no `dva stack`, and no `dva logs`. `dva help` documents all 27 for humans, so the machine-readable
surface is strictly worse than the human one — the inverse of the flag's purpose, and the same
theme as [TASK-088](../done/088-validate-json-covers-only-the-failure-it-does-not-produce.md).

## Options

- **A — derive it.** Walk `rootCmd.Commands()` at build time so the list cannot drift again. The
  per-command `Description`/`Type`/`Options` metadata in the literal has no cobra equivalent, so
  it would need to move onto the commands (e.g. annotations) or into a lookup keyed by name that
  a test asserts is total.
- **B — complete the literal and pin it with a test** asserting
  `len(StaticCommands) == len(reservedCommands)` with the diff printed on failure. Keeps the
  curated descriptions, costs one test to stop the drift.

B is the smaller change and directly prevents recurrence; A is the one that makes the class of bug
impossible. **Decision needed.**

## Acceptance criteria

- [ ] Every real command appears | verify: `dva manifest --format json | jq '.static_commands | length'` must equal 27; print both numbers
- [ ] No phantom commands | verify: every `static_commands` key must be in `reservedCommands`; print the count checked
- [ ] Drift cannot recur silently | verify: `go test ./internal/cli/ -run Manifest` — a test must fail if a command is added to root without a manifest entry, and print the diff
- [ ] The 13 existing descriptions are unchanged | verify: human — diff the descriptions for the original 13 keys before and after
- [ ] Not vacuous | verify: human — delete one entry and confirm the new test names it
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-088](../done/088-validate-json-covers-only-the-failure-it-does-not-produce.md) — the same
  audience getting a worse answer than the human one.
- [TASK-097](../done/097-interaction-usage-mishandles-keys-with-spaces.md) — the other manifest-correctness
  defect; both surface through `dva manifest`, which is documented as the agent-facing entry point.
