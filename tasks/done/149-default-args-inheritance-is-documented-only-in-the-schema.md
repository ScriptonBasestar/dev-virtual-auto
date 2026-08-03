---
id: TASK-149
title: "The default_args inheritance rule appears in no document, and the JSON key that changed meaning has no test"
type: chore
priority: P3
status: done
effort: S
created-at: 2026-08-03T13:40:00+09:00
source: "TASK-101 finalize verification — 101's Left open items, untracked"
depends-on: [TASK-101]
scope: "dva repo — docs/30-config-merge-semantics.md, internal/config/interaction_tree.go, runner explain JSON"
---

# Task 149: Write down how a child inherits `default_args`, and pin the key that changed

## Problem

Three items TASK-101 recorded as Left open, all still true on 2026-08-03.

**1. The rule is undocumented.** `grep -rn default_args --include='*.md' docs/ USAGE.md`
returns **0** hits. The parent-node-to-child inheritance rule exists only in
`internal/config/schema.json:408`. `docs/30-config-merge-semantics.md` covers cross-layer
merging (`internal/config/merge.go`) and mentions subcommands once, in a table row at
`:228`. So the behaviour a user is most likely to be surprised by is the one behaviour with
no prose.

**2. The rule is narrower than it reads.** A child declaring `script:`, `script_file:` or
`steps:` instead of `command:` still inherits the parent's `default_args`;
`interaction_tree.go:309` tests only `Command`/`CommandLines`. This is currently
unobservable — those paths never call `commandArgs` — so it is latent, not broken. It stops
being latent the moment one of them starts honouring arguments.

**3. A JSON key changed meaning with nothing pinning it.** The `--json` plan's `arguments`
key went from *literal invocation* to *effective arguments*. No `*_test.go` asserts the
value. `runner_explain_test.go:102` exercises the branch for a write error, not the key.
The JSON surface is consumed by agents, which makes an unpinned semantic change the
expensive kind.

Nothing tracks any of these: grepping `tasks/todo/`, `tasks/blocked/`, `tasks/decision/`
and `tasks/plan/` for `30-config-merge-semantics|inheritance|default_args` returns 0 files.

## Acceptance criteria

- [x] `default_args` inheritance is documented where a user merging configs will find it —
      `docs/30-config-merge-semantics.md` or the document that owns subcommand resolution.
      State which, and why that is the canonical home rather than a second copy.
      *USAGE.md — see "Canonical home" below; docs/30 gets a pointer, not a copy.*
- [x] The doc says what inherits and what does not, matching `interaction_tree.go:309`
      rather than the schema's shorter phrasing.
- [x] Item 2 is resolved one way or the other: either the non-`command:` forms are excluded
      from inheritance to match the documented rule, or the rule is written to cover them.
      A comment at `interaction_tree.go:309` records the choice. *Excluded.*
- [x] A test asserts the `--json` plan's `arguments` value for both a bare child and a child
      inheriting `default_args`, so the meaning cannot shift again unremarked.
- [x] `make test` and `make doc-check` exit 0 — the latter because this adds prose to an
      enforced path.

## Notes

The doc gate (TASK-090) enforces ≤500 lines / ≤10240 bytes under `docs/`.
`docs/30-config-merge-semantics.md` measured 345 lines / 9472 bytes on 2026-08-03 — roughly
770 bytes of headroom. Adding a full section will likely require the same split treatment
`docs/40` received, so plan for that rather than discovering it at `make doc-check`.

## Result

### Canonical home: USAGE.md, and why not either doc the criterion offered

Neither candidate owns this rule.

`docs/30-config-merge-semantics.md` is about the **cross-layer** axis — base ← modules ←
subprojects, the `internal/config/merge.go` path. Its §5-6 `interactions.<name>` table has a
`subcommands: key별 deep merge` row, and that row is correct *on that axis*. `default_args`
inheritance is a different axis entirely: **parent node → child node inside one file**, resolved
by `runner.mergeInteraction`, which `merge.go` never calls. Putting it in §5-6 would file it
under the wrong mechanism and invite the reading that the deep-merge row already covers it.

`docs/31-execution-plan-resolution.md` owns `plans.<name>` resolution for `dva up`. Its §3 lists
`interactions` as a 보조 레이어 (auxiliary input). `dva run` appears exactly once, at `:218`,
and about a different thing — which subproject's `dva.yml` has to exist for
`dva run <subproject>:<command>` to resolve. Nothing there describes how a subcommand takes
fields from its parent.

USAGE.md is where `interaction:` is documented for users and where `dva run` is defined, so it
is the document a reader who just wrote `subcommands:` is already in. New section
`### interaction.subcommands (default_args 상속)`, immediately after the existing
`### interaction (예약어와 훅)`.

`docs/30` §5-6 gets a three-line pointer instead of a copy: it says the `subcommands` row covers
layers only, that node-to-node is a different axis, and links to USAGE.md. That keeps one source
of truth and stops the merge doc from being read as complete on a rule it does not describe.

This also dissolves the Notes' concern. The size gate covers `docs/` and `workflows/`
(`tools/doccheck/main.go:6`), not the repo root, so USAGE.md is unconstrained and no split was
needed. `docs/30` grew 4 lines / 364 bytes to 349 / 9836 — inside the gate with 404 bytes left,
which is worth knowing before the next edit to that file.

### Item 2 resolved: script/script_file/steps are excluded, and it was not latent

The task recorded item 2 as "currently unobservable — those paths never call `commandArgs` — so
it is latent, not broken." **That is wrong, and measuring it is what changed the fix.**
`commandArgs` *is* called for those children, unconditionally, at `runner.go:133` in the
`--json` branch. Measured before the change:

```
$ dva run rails scripted --explain --json      # child declares script:, parent has default_args
"arguments": ["-e", "development"]
$ dva run rails scripted
scripted child ran                              # the script runs; the arguments go nowhere
```

So the JSON surface — the one the task itself calls "the expensive kind" to leave unpinned — was
publishing arguments for an execution that consumes none. Same for `script_file:` and `steps:`.

Resolved by **excluding** them: `mergeInteraction`'s reset case now covers `Script`,
`ScriptFile` and `Steps` alongside `Command`/`CommandLines`. The rule's own stated intent already
covered it — the TASK-101 comment says "only redeclaring the command starts the arguments
clean", and a child declaring `script:` *has* redeclared what runs, just in a form the condition
did not enumerate. Widening the condition keeps one rule rather than adding a command-shaped
exception to it.

Runtime is unaffected on the local and compose paths: `LocalRunner.Execute` returns at the
steps/script_file/script branches (`local.go:23-39`) before reaching `commandArgs` at `:48`, and
`DockerComposeRunner.Execute` returns at its own steps branch or falls back to `LocalRunner` for
scripts. The corpus below confirms it empirically.

The comment at the switch records the choice and names what it deliberately does **not** fix.

### Two adjacent defects found, filed rather than absorbed

Tracing which runners consume `commandArgs` surfaced two bugs outside this task's scope:

- **[TASK-174](../todo/174-explain-names-the-parents-command-for-a-child-that-runs-a-script.md)** — `mergeInteraction` also inherits `Command`, and only a child's own `command:`
  overwrites it. So the same `scripted` child still reports `"command": "bundle exec rails"`,
  naming a command it will not run. This fix makes the plan half-right (no arguments, wrong
  command) rather than wholly wrong, and stopping there is deliberate: `Command` feeds
  `DetectRunnerType`'s siblings and `composeArguments`' `cmd != ""` gate, so emptying it is a
  behaviour change needing its own measurement.
- **[TASK-175](../todo/175-kubectl-runner-drops-script-and-script-file-and-runs-the-inherited-command.md)** (P2) — `KubectlRunner.Execute` branches on `Steps` and nothing else. `Script`
  and `ScriptFile` appear nowhere in `kubectl.go`, so a `pod:` + `script:` interaction falls
  through to `kubectl exec <pod> -- <Command>` and the script is discarded. This is exactly the
  defect TASK-094 closed in the same function for `steps:`, one branch short; the compose runner
  handles the case explicitly, so kubectl is the outlier.

### Test

`internal/runner/explain_json_args_test.go`. Seven rows through `NewInteractionTree(...).Find`
rather than hand-built `ResolvedCommand` literals, so the resolution path is what is pinned, not
a restatement of it:

| child | arguments |
|---|---|
| container (`description:` only) | `["-e","development"]` — inherits |
| redeclared (`command:`) | null |
| own_args (`default_args:`) | `["-e","test"]` — child wins |
| scripted / filed / stepped | null — **the TASK-149 rows** |
| the parent itself | `["-e","development"]` — control, so the null rows cannot pass by inheriting nothing |

Plus `TestExplainJSONArgumentsKeyIsAlwaysPresent`, guarding shape rather than value: a consumer
reading `plan["arguments"]` must not have to tell "absent" from "nothing to pass".

The existing `TestExplainReportsTheArgumentsThatWillBePassed` covers the *text* branch's
`Arguments:` line and was left untouched — it is the text sibling, not a duplicate.

### Falsification

Each revert was diffed against a saved copy before running, to confirm it was complete.

| # | revert | result |
|---|---|---|
| F1 | drop `Script`/`ScriptFile`/`Steps` from the reset case | 3 rows fail — `scripted`, `filed`, `stepped`, each `arguments = [-e development], want []` |
| F2 | `"arguments": cmd.Argv` (the pre-TASK-101 meaning) | 3 **different** rows fail — `container`, `own_args`, parent, each `arguments = [], want [...]` |
| F3 | delete the `arguments` key entirely | only `TestExplainJSONArgumentsKeyIsAlwaysPresent` fails |

F1 and F2 hit disjoint row sets, which is the point: F1 proves the inheritance rule, F2 proves
the key still means *effective* arguments and not the literal invocation — the semantic drift
that went unpinned after TASK-101. The one row neither touches is `redeclared`, null under both;
that is the negative control's job.

### Corpus

Both binaries — "before" built from `git archive HEAD` — run `dva ls --json` against all 19
YAML files under `examples/`, each in its own directory with siblings intact.

**19 files, 19 byte-identical, 0 changed.** No shipped example puts a `script:`/`steps:` child
under a parent carrying `default_args`. Positive control through the same comparison, on the
t149 fixture: `["-e","development"]` → `null`. The comparison detects the change; the corpus has
none to detect.

### Documented claims, each measured

Every rule in the new USAGE.md section was run against the built binary rather than read off the
source. One was wrong and the measurement caught it: the draft said
`dva run rails console -p 4000` passes `-p 4000` through. It does not — `-p` is `dva run`'s own
`--publish` flag, so the plan came back with `default_args` untouched. Corrected to a non-flag
example plus the `--` escape, and re-measured:

```
dva run rails test smoke       -> ["smoke"]           # argv replaces, does not append
dva run rails test -p 4000     -> ["-e","test"]       # dva ate the flag
dva run rails test -- -p 4000  -> ["-p","4000"]       # the escape
```

### Schema

`schema.json`'s `default_args` description said "does not declare a command of its own", which
is now narrower than the code. Updated to "does not redeclare what it runs — command, script,
script_file or steps" and pointed at USAGE.md. Leaving it would have been a second description
knowingly disagreeing with the first.

### Gates

| gate | result |
|---|---|
| `make test` | pass (`-race -cover`; `internal/runner` 68.3% → **69.2%**) |
| `make doc-check` | OK — 527 links, 0 broken, 0 oversized |
| `gofmt -l internal/ cmd/ tools/` | clean |
| `go vet ./...` | exit 0 |
| `golangci-lint run ./internal/runner/...` | 0 issues |

`make lint` remains blocked by the pre-existing mise GOTOOLCHAIN drift in `tools/doccheck`,
unrelated to these files and unchanged by them.

### Changed files

- `internal/runner/interaction_tree.go` — reset case widened to script/script_file/steps, with
  the decision and its known gap (TASK-174) recorded at the switch.
- `internal/runner/explain_json_args_test.go` — new; pins the `--json` `arguments` key.
- `USAGE.md` — new `### interaction.subcommands (default_args 상속)` section.
- `docs/30-config-merge-semantics.md` — pointer in §5-6 separating the two axes.
- `internal/config/schema.json` — `default_args` description matches the code again.
- `tasks/todo/174-*.md`, `tasks/todo/175-*.md` — the two adjacent defects.
