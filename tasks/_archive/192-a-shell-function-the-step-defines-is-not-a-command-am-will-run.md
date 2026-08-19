---
id: TASK-192
title: "A shell function the step defines is not a command am will run"
type: bug
priority: P1
effort: S
created-at: 2026-08-18T17:46:43+09:00
completed-at: 2026-08-18T19:12:00+09:00
quality-review: pass
quality-reviewed-at: 2026-08-19T14:07:00+09:00
verified-at: 2026-08-19T14:20:22+09:00
archived-at: 2026-08-19T14:20:22+09:00
quality-review-evidence: |
  - kind: automated
    command-or-step: "go run ./tools/flowcheck (AC1) + grep -rn yaml_block_keys agent-mesh-flows/"
    result: exit 0 over 103 shell fields; zero occurrences of yaml_block_keys remain, so no field calls a function it defines
  - kind: automated
    command-or-step: "go test ./tools/flowcheck/ -run 'TestLocalFunction|TestCommentQuote' (AC2)"
    result: exit 0 — TestLocalFunction has 6 subtests carrying the design claims (a definition alone is not a call; the name as an argument is not a call; a call inside a double-quoted substitution is found, which is the tokenizer-descent fix); TestCommentQuote has 7
  - kind: automated
    command-or-step: "AC3 output equivalence — awk block extracted verbatim from dva-improve.yaml:249 and run on a compose fixture (3 services, 2 networks, 1 volume, nested ports:/image: keys)"
    result: emits `services: web api db` / `networks: backend frontend` / `volumes:  pgdata` — the three blocks separate with the documented %-9s padding, and the `ind == base` guard excludes nested keys, so a network cannot be read as an undeclared service. Matches the output the card records byte for byte
  - kind: automated
    command-or-step: "am validate agent-mesh-flows/dva-improve.yaml && am validate agent-mesh-flows/dva-improve-guided/00-analyze.yaml (AC4)"
    result: exit 0 on both
  - kind: manual
    command-or-step: "naming drift check on the rule id this card introduces"
    result: the card names the rule `comment-apostrophe`, which is what 499bd6b shipped; 55c2eb0 (TASK-193) renamed it to `comment-quote` when it widened to double quotes. The card is accurate for its own commit — later rename, not an error here
source: "TASK-191 — uncovered once the span that blocked first was removed"
scope: "dva repo — agent-mesh-flows/dva-improve-guided/00-analyze.yaml, agent-mesh-flows/dva-improve.yaml, tools/flowcheck"
status: done
verification-summary: |
  quality-review pass, re-observed at disposition. All four AC bindings exit 0. The defect this
  card removed stays removed: `yaml_block_keys` has zero occurrences anywhere in
  agent-mesh-flows/, so no shell field calls a function it defines. TestLocalFunction +
  TestCommentQuote report 13 passing subtests together (6 + 7, as reviewed), and both rule ids
  are emitted from tools/flowcheck/shell.go:50 and :94. The `comment-apostrophe` ->
  `comment-quote` rename recorded as a drift note was TASK-193's widening, not a defect here.
---

# Task 192: A shell function the step defines is not a command am will run

## Summary

Three fields defined `yaml_block_keys()` and then called it. The am allowlist knows
commands, not a function the step just declared, so every call blocked:

```
blocked: shell policy: command "yaml_block_keys" not in allowlist
```

| field | file |
|---|---|
| `scan_project.context.compose_services` | `dva-improve-guided/00-analyze.yaml` |
| `scan_compose.context.root_compose` | `dva-improve.yaml` |
| `scan_compose.context.infra_compose` | `dva-improve.yaml` |

Both steps enumerate compose services. `scan_project` feeds the analysis report the whole
guided pipeline reads; `scan_compose` feeds the `dva-improve` prompt. Neither had produced
its output since the function was introduced — the block was masked by an earlier
`comment-substitution` block until TASK-191 removed it, so the runs looked the same
before and after: successful, with a step that never ran.

## Resolution

**The function became one `awk` pass per file.** Same three top-level blocks, one command
per call site, no name for the allowlist to reject. The output is byte-identical to the
function it replaced, including the `%-9s` padding that aligns `volumes:` under the other
two labels.

**Replacing it was not enough: the awk program blocked in turn, on `BEGIN`.** am carries
quote state across a comment — `#` does not end a string, and a string is not confined to
a line — so the three apostrophes in the prose above the loop (`block's`, `cwrapper-devbox's`,
`am's`) inverted the quote parity of the field, and the opening quote of `awk '...'` was
read as a *closing* one. The program body then sat in command position. Measured on a
probe with nothing else different: one apostrophe in the comment blocks the step, two make
it run.

So every apostrophe was removed from comment prose inside a shell field — 10 fields, 4
files. Parity is not a property anyone can maintain: adding a word to one comment arms a
block in code three lines below that nobody touched.

**Two flowcheck rules and one tokenizer fix keep it from regrowing:**

- `local-function` — a field that defines a function and then calls it. The definition is
  harmless, so only the call is reported.
- `comment-apostrophe` — an apostrophe in comment text. Every one, not the odd ones, for
  the reason above.
- The tokenizer now descends into a `$(...)` inside a double-quoted string. It used to
  treat `"..."` as opaque, which is exactly why the three call sites were invisible to it:
  they arrived as `echo "  services: $(yaml_block_keys "$f" services)"`. Token depth keeps
  the substitution's own tokens from being read as arguments of the command the string
  belongs to — without it, `jq` and `date` are reported as unquoted arguments of `[` and
  `printf` in four fields that run.

## Completion Criteria

- [x] No flow field calls a function it defines | verify: `go run ./tools/flowcheck`
- [x] flowcheck fails on a field that defines and calls one | verify: `go test ./tools/flowcheck/`
- [x] The three fields run and emit the same block keys as the function did | verify: human — extract each field and run it through `am` against a fixture with compose services, networks and volumes; compare against the function's output on the same fixture
- [x] Flows still validate | verify: `am validate agent-mesh-flows/dva-improve.yaml && am validate agent-mesh-flows/dva-improve-guided/00-analyze.yaml`

## Evidence

**The three fields, extracted verbatim and run through am against a compose fixture**
(`services: web api db`, `networks: backend frontend`, `volumes: pgdata`, plus a second
file and an `infra/compose.yaml`):

| field | at HEAD | after |
|---|---|---|
| `scan_project.compose_services` | blocked: `command "yaml_block_keys" not in allowlist` | ran, `MARKER … RAN` |
| `scan_compose.root_compose` | blocked: same | ran, `MARKER … RAN` |
| `scan_compose.infra_compose` | blocked: same | ran, `MARKER … RAN` |

Replacing the function alone left all three blocked on `command "BEGIN" not in allowlist`;
they run only with the comment apostrophes gone as well.

**Output equivalence** — old function vs new awk, both fed the same fixture through
`/bin/sh`, compared as bytes: identical for all three fields.

```
services: web api db      networks: backend frontend   volumes:  pgdata
services: cache queue     networks: mesh               volumes:  spool
```

**Corpus sweep** — all 79 shell fields in `agent-mesh-flows/` extracted into one probe
flow and run under am: 1 blocked step remains, `40-execute.yaml save_report.action`, on
an unquoted heredoc delimiter. That is a different defect and is filed as TASK-193.

**flowcheck against the pre-fix corpus** (`git archive HEAD`): 9 `local-function` and 10
`comment-apostrophe` findings. Against the fixed corpus: `OK — no decision-path defects`.

**The tests bind.** Each change stubbed in turn, and the cases that fail:

| stubbed | failing cases |
|---|---|
| `comment-apostrophe` rule | 3 |
| `local-function` rule | 3 |
| descent into `$(...)` inside `"..."` | 2 |
| token depth | 3 |

## Technical Notes

- The function existed for a reason worth preserving: a flat indent grep conflates services
  with networks and volumes, so a network read as an undeclared service. The awk keeps
  emitting each top-level block separately.
- `~/.config/agent-mesh/sandbox_override.yaml` can add commands to the allowlist. It is
  user-local config, so a shipped flow cannot rely on it — the same reasoning as TASK-191.
- am measurements in this task, all against am cb8b4ce: a multi-line `awk '…'` runs fine on
  its own; one apostrophe in a comment above it blocks it on `BEGIN`; a second apostrophe
  makes it run again; a heredoc body is read as shell unless the delimiter is quoted.
