---
id: TASK-076
title: "`dva ls` and `dva manifest` advertise a conflicting interaction unmarked, and the manifest prints the one invocation that provably cannot reach it"
type: fix
priority: P2
effort: M
status: todo
created-at: 2026-07-30T00:00:00+09:00
scope: "internal/cli — manifest.go (dynamic command emission) + list.go (ls output) + tests; USAGE.md:643 severity claim"
---

# Task 076: Do not advertise an interaction that the advertised command will not run

## Problem

An `interaction:` key that collides with a reserved command is dropped from the top-level
command set. Every discovery surface still lists it as if it were available, and one of them
prints the exact invocation that is guaranteed to run something else.

`dva manifest` is the LLM-facing document — `internal/cli/CLAUDE.md` describes `--json`
output as "LLM 파이프라인용". For an interaction named `build` it emits, in one document,
with no conflict marker and exit 0:

```json
"static_commands":  { "build": { "type": "compose_shortcut", "description": "Build service images" } },
"dynamic_commands": { "build": { "command": "make build", "usage_example": "dva build" } }
```

Two entries under the same key, two different meanings, and `usage_example: "dva build"` is
the single form measured below that does **not** reach `make build`. A consumer that reads
the manifest and executes the usage example runs the compose shortcut instead, silently.

`internal/cli/manifest.go:163` builds that string as `fmt.Sprintf("dva %s", k)` for every
key, unconditionally. Neither `manifest.go` nor `list.go` calls `IsReservedCommand` at all —
the check exists only in `validate.go` and `reserved.go`.

## Evidence (verified 2026-07-30)

Fixture — one `dva.yml`, no `applications:`, two interactions that differ only in whether the
name is reserved:

```yaml
version: "0.1.44"

stack:
  infra:
    description: PostgreSQL
    default_runner: compose
    runners:
      compose:
        files:
          - compose.yml

interaction:
  build:
    description: build the thing
    command: "make build"
  my-build:
    description: build the thing, non-reserved name
    command: "make build"
```

Measured against `bin/dva` (0.1.44):

| command | exit | what happened |
| --- | --- | --- |
| `dva validate` | **1** | `ERROR: reserved command conflict in dva.yml` |
| `dva config validate` | **1** | same error |
| `dva ls` | 0 | prints `build  # build the thing`, unmarked |
| `dva ls --json` | 0 | `build` present, no conflict field |
| `dva manifest` | 0 | `build` in both command maps, `usage_example: "dva build"` |
| `dva build` | 1 | **builtin compose shortcut** — `open …/compose.yml: no such file or directory` |
| `dva run build` | 2 | **the interaction ran** — `make: *** No rule to make target 'build'` |
| `dva my-build` | 2 | the interaction ran, as expected for a non-reserved name |

Every load also logs, on stderr, regardless of subcommand:

```
level=WARN msg="interaction command 'build' conflicts with reserved DVA command(s) and
will be ignored. Rename to avoid shadowing (e.g., 'my-build' or 'custom-build')"
```

## One condition, four contracts

| source | says |
| --- | --- |
| `internal/config/reserved.go:113` (WARN) | "will be ignored" |
| `internal/config/validate.go:137` (ERROR) | fatal, exit 1 |
| `USAGE.md:643` | "충돌은 에러가 아니라 경고이므로 `dva config validate`로 확인하세요" |
| measured runtime | ignored as a **top-level shortcut**; still reachable via `dva run build` |

None of the four is a superset of the others.

- "will be ignored" is false as written: `dva run build` executed `make build`. The key is
  dropped from top-level registration only.
- `USAGE.md:643` says the conflict is a warning and not an error, and directs the reader at
  `dva config validate` to see it. That command exits 1. The doc is wrong about the severity
  of the very command it recommends.
- `validate` is fatal, but nothing on the load path enforces it — `ls`, `manifest` and `run`
  all load the same config and exit 0. So the config is simultaneously invalid and running.

This matters for the fix, not just as trivia: a manifest cannot mark a conflict until the
project decides what the conflict *means*. Marking `build` "ignored" would repeat the false
claim. The accurate statement is narrower — the name is unavailable as a top-level command
and reachable only through `dva run`.

## Why it matters

`dva manifest` exists to be read by a machine that will then run something. A field named
`usage_example` carries an implicit promise that executing it invokes the entry it sits
inside. Here it invokes a different code path with a different description, and the
divergence is invisible in the document: no marker, no severity, exit 0.

`dva ls` has the same shape with a human reader and lower stakes — the user sees `build`
offered and types `dva build`.

The WARN does reach stderr on every load, which is why this has not bitten a human hard yet.
It does not reach a JSON consumer's stdout, and it is the message that is wrong about what
happens.

This is the same family as [TASK-074](../done/074-app-subcommands-answer-an-absent-section-three-ways.md):
a discovery surface that describes a capability the next command does not honor. TASK-074's
`dva app` advertises seven subcommands for an absent section; this one advertises a present
interaction that the advertised invocation skips.

## Fix shape

Decide the contract sentence first, then make all surfaces repeat it:

> A reserved-name interaction is not registered as a top-level command. It remains
> executable as `dva run <name>`.

Then:

- `manifest.go:163` — when the key is reserved, `usage_example` must be `dva run <name>`,
  and the entry must carry a machine-readable conflict field (a `shadowed_by` or
  `conflict` key naming the static command that wins). A consumer must be able to detect
  this without string-matching a description.
- `list.go` — mark the row in both text and `--json`. The JSON shape is a consumer contract;
  add a field, do not decorate the description string.
- `reserved.go:113` — "will be ignored" becomes what was measured. Keep the rename hint.
- `USAGE.md:643` — the severity claim is false; `dva validate` exits 1. Fix the sentence, and
  check the table above it (lines 634-640) whose "**충돌** — 무시됨" rows carry the same
  imprecision.

Open decision, not settled here: whether `validate` should stay fatal while the load path
proceeds. Fatal-and-ignored is defensible (the config is malformed; running commands are
best-effort) but it is currently accidental rather than chosen. Whichever way it lands, the
four sources above must agree afterward.

Do not name `dva.yml` in any message this task touches — config is the merge of `modules:`
and `subprojects:`, so the file needing the edit is not knowable from the loaded config.
`validate.go:137` currently hardcodes it. Same misdirection TASK-073 removed.

## Non-goals

- Do not change which command wins. The builtin shadowing the interaction is the designed
  behaviour and the rename hint is correct.
- Do not remove the conflicting entry from `ls`/`manifest`. A user who declared it needs to
  see that dva received it and what became of it; silence would be worse than a wrong label.
- Do not add a `--json` error envelope. Still separate, still larger — recorded in TASK-074.
- Do not touch the namespaced-prefix branch (`app:build`) beyond whatever falls out of the
  shared helper. It has its own hint at `validate.go:126`.

## Acceptance criteria

- [ ] A reserved-name interaction is marked in the manifest with a machine-readable field | verify: `go test ./internal/cli/ -run TestManifestMarksReservedInteraction`
- [ ] Its `usage_example` is the form that reaches it | verify: `go test ./internal/cli/ -run TestManifestUsageExampleReachesTheInteraction` — asserts `dva run build`, not `dva build`
- [ ] A non-reserved interaction is unchanged | verify: `go test ./internal/cli/ -run TestManifestLeavesNonReservedInteractionAlone` — `my-build` keeps `usage_example: "dva my-build"` and carries no conflict field
- [ ] `dva ls --json` exposes the same mark | verify: `go test ./internal/cli/ -run TestLsJSONMarksReservedInteraction`
- [ ] `dva ls` text output marks the row | verify: `go test ./internal/cli/ -run TestLsTextMarksReservedInteraction`
- [ ] The mark tests are not vacuous | verify: `human — drop the reserved check from manifest.go and list.go; all five tests above must FAIL`
- [ ] No message on this path claims the interaction is ignored outright | verify: `/usr/bin/grep -c 'will be ignored' internal/config/reserved.go` — print the count, expect 0
- [ ] No message on this path names a config filename | verify: `/usr/bin/grep -c 'conflict in dva.yml' internal/config/validate.go` — print the count, expect 0
- [ ] The doc no longer calls the conflict a warning | verify: `/usr/bin/grep -c '에러가 아니라 경고' USAGE.md` — print the count, expect 0
- [ ] The documented severity matches the binary | verify: `human — run dva validate on the fixture above and read USAGE.md:632-644 together; exit code and prose must agree`
- [ ] Full suite passes under -race | verify: `make test`
- [ ] Binary builds through the project's own path | verify: `make build`

## Related — the loop that should have caught this

`workflows/dva-dogfood/ref-evaluation.md` declares a `lifecycle_boundary` surface with
`instances: per_overlap`, discovering "a service or process owned by more than one of stack,
plans, applications, interaction".

The overlap here is `interaction:` against the **builtin command namespace**, which is not one
of the four listed owners. So this collision class never instantiates a case, and the quality
of dva's answer to it has never been scored — structurally the same gap TASK-074 found, from
the opposite direction: TASK-074's surface produced no case because a section was absent, this
one produces no case because the second owner is not a config section at all.

Candidate manifest amendment, deferred: widen `lifecycle_boundary`'s discover clause to include
the reserved command set as an owner, which makes every reserved-name interaction an instance.
Changing the manifest bytes changes `case_manifest_hash`, so stage 60 must treat the first run
after it as a cross-run promotion rather than a regression.

## Related

- [TASK-074](../done/074-app-subcommands-answer-an-absent-section-three-ways.md) — the absent-section
  half of the same discovery problem; shares the "do not name a config filename" constraint.
- [TASK-073](../done/073-version-error-blames-the-config-for-a-build-defect.md) — precedent for
  the mutation check on not-contains assertions.
- [TASK-067](../done/067-version-field-rule-stated-three-incompatible-ways.md) — precedent for
  one rule stated in mutually incompatible ways across code and docs.
