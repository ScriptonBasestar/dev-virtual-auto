---
id: TASK-074
title: "The seven `dva app` subcommands answer an absent `applications:` section three different ways, none of which routes the user anywhere"
type: fix
priority: P2
effort: S
status: todo
created-at: 2026-07-30T00:00:00+09:00
scope: "internal/cli — app.go (the empty-applications path in ls/up/stop/down/restart/build/log) + app tests"
---

# Task 074: One answer for the empty `applications:` section, and make it a route

## Problem

A user ran `dva app` in a stack-only devbox project, saw seven subcommands offered, ran
`dva app up`, and got:

```
ERROR: no applications defined in dva.yml
```

That is a dead end. It states what is missing and stops — no next command, no statement of
what the config *does* declare. `dva app` had just advertised `up` as available, so the
failure reads as a defect in dva rather than as "this project has no apps, use the stack."

Underneath it, the same condition is handled three incompatible ways:

| subcommand | site | behaviour |
| --- | --- | --- |
| `ls` | `internal/cli/app.go:39-42` | stderr `No applications defined in dva.yml`, **exit 0** |
| `up` `stop` `down` `restart` `build` | `app.go:61,78,99,180,219` | `ERROR: no applications defined in dva.yml`, **exit 1** |
| `log` | no guard at all | falls through the status loop to `app.go:359` → `ERROR: application 'myapp' not found`, **exit 1** |

Capital `No` versus lowercase `no`, exit 0 versus exit 1, and `log` asserting something
false: it reports the named app as missing from a set of apps, when the set itself does not
exist. Five copies of one string literal, one divergent copy, one gap.

## Evidence (verified 2026-07-30)

Reproduced against `bin/dva` (0.1.44). The whole fixture is one file — `dva.yml` in an
empty directory, declaring `stack:` and `interaction:` and no `applications:`:

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
  hello:
    description: say hello
    steps:
      - step: echo hi
```

`dva validate` on it prints `✅ dva.yml is valid`, so nothing below is an artefact of a
malformed config. Run outside a git repo, so the `.gitignore` warning does not interleave:

```
=== dva app ls ===          exit=0   stderr: No applications defined in dva.yml
=== dva app up ===          exit=1   stderr: ERROR: no applications defined in dva.yml
=== dva app up myapp ===    exit=1   stderr: ERROR: no applications defined in dva.yml
=== dva app build ===       exit=1   stderr: ERROR: no applications defined in dva.yml
=== dva app log myapp ===   exit=1   stderr: ERROR: application 'myapp' not found
```

Three facts found while checking what the message *could* point at, all of which constrain
the fix:

1. **`dva ls` is not the discovery command it sounds like.** On the same fixture it printed
   `hello  # say hello` and nothing else — it lists `interaction:` only. `dva ls --json`
   agrees: the document contains the one interaction, no `stack`. So a message that says
   "run `dva ls` to see what is declared" would be wrong.
2. **`dva stack` has no `ls`.** Its subcommands are `down log status stop up`;
   `dva stack ls` prints the parent help and exits 0.
3. **`dva stack up` is not the command to route to.** `USAGE.md:157` — "`stack`은 선언
   저장소이므로 `dva stack up`은 더 이상 권장 모델이 아닙니다". The route is `dva up`,
   which works whether or not the config declares `plans:`. Verified on this fixture, which
   declares none: `dva up --dry-run` resolved the stack entry and exited 0. On a config with
   plans it prints a `[plan: <name>]` header first and is otherwise identical.

## The section this command serves is itself migration-only

`shared-guardrails.md` rule 33: "Use `dva up <plan>`/`down <plan>` for declarative
lifecycle. **Legacy `dva app` and `--mode` behavior is migration-only.**" Rule 27 says
native app processes should be modelled as native/process stack runners selected from plans,
and rule 2 says new configs MUST use named `plans:`. `ARCHITECTURE.md:78`'s domain-boundary
table has no `applications` row at all.

So a config with no `applications:` is not an incomplete config — it is the shape the
project's own generation rules require. `dva app` advertises seven subcommands for a section
its own guardrails forbid generating, and the failure message treats the compliant state as
the user's mistake. That raises the priority of the routing half of this task above the
consistency half.

## Why it matters

This is the same class as [TASK-073](../done/073-version-error-blames-the-config-for-a-build-defect.md)
— a message that sends the reader at the wrong thing — with the aggravating factor that the
reader here is not confused about a dormant branch. They are stopped, in a project where
nothing is wrong.

The evidence for "this path was never walked" is inside the same file: the port-conflict
error at `app.go:162` ends with `run 'dva app down' to reclaim the port(s), then retry`, and
`root.go:236` appends a `dva init` hint when the config is missing entirely. Both of those
paths were met by someone and got a route. The empty-section path was not.

`ls` returning exit 0 is the tell that the contract was never decided rather than decided
inconsistently — someone writing `ls` concluded that nothing-to-list is not a failure, and
that conclusion never propagated.

## The contract (decided)

Branch on whether the user named an app.

```
$ dva app up
  no applications declared; this config declares: stack (1), interaction (1)
  → dva up                # start the declared lifecycle
  exit 0

$ dva app up myapp
  ERROR: no applications declared, so 'myapp' cannot be started
  → dva up                # start the declared lifecycle
  exit 1
```

Rationale: "start everything" with zero targets is not a failure, so a Makefile chain that
includes `dva app up` in a stack-only project must not break. Naming a target that cannot
exist *is* a failure, and swallowing it would let `dva app up myapp-typo` pass silently.
`ls` (exit 0) and `log NAME` (exit 1) already fall out of this rule correctly; only their
wording changes.

Route to `dva up`, never to `dva stack up` or `dva app <anything>`. `dva up` is the current
model's entry point (guardrail 33), it needs no argument, and it works with or without
`plans:`. When the config declares plans, name one: `→ dva up local-dev`.

## Fix shape

One helper in `app.go` that both branches call, replacing the five literals, the `ls`
special case, and the missing `log` guard. It needs to answer three things the current
message answers none of:

- whether the invocation named a target (decides the exit code)
- what the config *does* declare, with counts, so the user learns the project's shape
- the current-model route (`dva up`, plus a plan name when `plans:` is non-empty)

Do not name `dva.yml` as the place to add apps. Config is the merge of `modules:` and
`subprojects:`, so the file that would need the edit is not knowable from the loaded
config — the same misdirection TASK-073 removed. Say "this config", not a filename.

Tests: one per branch, asserting exit code and that the message names a real alternative
command. Assert the absence of `dva.yml` in the message, and mutation-check that assertion
the way TASK-073 did — a not-contains assertion passes trivially if the string was never
going to be there.

## Non-goals

- Do not add an `applications:` section to any target project. The absence is correct;
  the defect is in how dva answers it.
- Do not change `dva ls` to list stack entries. It is wrong that `ls` shows only
  interactions, but that is a separate contract with its own users — see below.
- Do not add a `--json` error envelope here. Separate, larger, see below.
- Do not touch the port-conflict message at `app.go:162`. It is the model, not the target.

## Acceptance criteria

- [ ] Bare `dva app up` on a config with no `applications:` exits 0 and names a runnable command | verify: `go test ./internal/cli/ -run TestAppUpBareOnAbsentApplications`
- [ ] `dva app up NAME` on the same config exits 1 and names NAME | verify: `go test ./internal/cli/ -run TestAppUpNamedOnAbsentApplications`
- [ ] `dva app log NAME` no longer reports the app as "not found" | verify: `go test ./internal/cli/ -run TestAppLogOnAbsentApplications`
- [ ] No message on this path names a config filename | verify: `go test ./internal/cli/ -run TestAbsentApplicationsMessageNamesNoFile`
- [ ] The not-contains assertion is not vacuous | verify: `human — reinstate "in dva.yml" in the helper; the test must FAIL`
- [ ] The five duplicated literals are gone | verify: `/usr/bin/grep -c 'no applications defined' internal/cli/app.go` — print the count, expect 0
- [ ] All seven subcommands answer through the one helper | verify: `human — read the seven RunE bodies; each empty-section exit goes through the helper`
- [ ] The route is the current model, not another legacy command | verify: `go test ./internal/cli/ -run TestAbsentApplicationsRoutesToCurrentModel` — asserts the message offers `dva up` and offers neither `dva stack` nor `dva app`
- [ ] The suggested command actually works on a plan-less config | verify: `human — dva up --dry-run on the repro fixture above, expect exit 0`
- [ ] Full suite passes under -race | verify: `make test`
- [ ] Binary builds through the project's own path | verify: `make build`

## Related findings, not covered here

Surfaced by the same three commands. Recorded so they are not lost; each needs its own task.

- **`--json` does not cover failures.** `root.go:220` prints every error as plain
  `ERROR: …` on stderr regardless of the flag, and `gitignore.go:178` does the same for its
  warning. Verified: `dva app up --json` emits nothing on stdout, so a consumer piping it to
  `jq` gets an empty document and no signal. `internal/cli/CLAUDE.md` describes `--json` as
  "LLM 파이프라인용".
- **The gitignore warning fires on every config load and preempts the real output.**
  `root.go:281` calls it from inside `loadConfig()`, so it printed above the `app up`
  failure — the user reads advice about `.gitignore` before the error they caused. No
  once-per-run, no suppression under `--json`. Verified by `git init` on the repro fixture.
- **No command answers "what does this config declare?"** `ls` covers `interaction:` only;
  `stack` has `status` but no `ls`; `applications` has `ls`. The fix above works around this
  by inlining counts into one message, which is a patch over the missing command.

## Related — the loop that should have caught this

`workflows/dva-dogfood/ref-evaluation.md` routes cases by declared surface, and states that
a surface with no instance in the target is **not** a case — the absence is filed under
`evaluation.not_applicable_surfaces` as evidence. So the quality of dva's response to an
absent section is never scored, which is why nineteen cycles passed over this.

Candidate manifest addition, deferred: an `absent_section_route` surface with
`instances: per_absent_section`, scored on whether the command (a) states a next action,
(b) states what is declared, (c) is parseable under `--json`. Changing the manifest bytes
changes `case_manifest_hash`, so stage 60 must treat the first run after it as a cross-run
promotion rather than a regression.
