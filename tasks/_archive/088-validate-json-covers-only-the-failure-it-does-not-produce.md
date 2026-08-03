---
id: TASK-088
title: "`dva validate --json` emits JSON only when it fails, and only via the generic envelope — its verdict and every warning stay prose"
type: fix
priority: P3
effort: M
status: done
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/cli/validate.go — 0 references to jsonOutput; the verdict at :70 and the warnings at :42/:84/:88"
verified-at: 2026-08-03T13:00:00+09:00
archived-at: 2026-08-03T13:00:00+09:00
verification-summary: |
  Re-measured all four paths on ./bin/dva with fresh fixtures built from the task's own YAML.
  clean/warnings/schema/load-failure each yield exactly 1 document by `jq -s 'length'`;
  `--json --strict` (a fifth path the task did not table) also yields 1. Warnings fixture gives
  `.warnings|length` = 2 with `migration_guide`/`affected_entries`/`hint` extracted into `fields`;
  schema fixture gives two `.errors` entries both keyed `provision.default.0`, exit 1, with
  TASK-079's `.error.message`/`.error.exit_code` preserved inside the new document.
  Human output is unmoved: stderr `diff`s IDENTICAL between plain and `--json` in all four rows,
  plain stdout stays 21/21/0/0 bytes, and the commit diff (2e0cfd6) shows composeNameWarningLines
  reproduces the old format strings verbatim. `go test ./internal/cli/ -run JSON -v` → 52 RUN+PASS
  over 26 real test names, including the anti-double-document subtest; package coverage now 65.4%.
---

# Task 088: the one command whose whole output is a machine answer does not produce one

## Problem

`internal/cli/CLAUDE.md` states `--json` exists for the LLM pipeline.
`internal/cli/validate.go` contains **zero** references to `jsonOutput` — the flag is accepted and
never consulted. What JSON a consumer does get comes entirely from
[TASK-079](../_archive/079-json-flag-does-not-cover-failures.md)'s failure envelope, which catches the
error `validate` returns without knowing anything about validation.

Measured on 0.1.44, all four paths:

| validate path | `--json` honoured | stdout | exit |
| --- | --- | --- | --- |
| load failure (missing/unparseable config) | **yes** | 1 JSON doc, `.error.message` | 1 |
| schema failure | **yes** | 1 JSON doc, `.error.message` | 1 |
| **warnings** | **no** | `✅ dva.yml is valid` (prose) | 0 |
| **success** | **no** | `✅ dva.yml is valid` (prose) | 0 |

Rows 1-2 are the envelope doing its job. Rows 3-4 are `validate` doing its own, and there the
`--json` output is **byte-identical to the output without the flag** — 18 bytes of prose.

The control that makes this readable: `dva ls --json` on the same fixture yields `{}`, one real
document. So the flag mechanism works; `validate` simply never asks about it. Document counts here
come from `jq -s 'length'`, never `jq -e .`, which exits 0 on concatenated documents and cannot
count.

## Why the warnings are the real loss

The failure rows already carry their message. The warning row throws away the richest output the
command produces — 1021 bytes of it on the fixture, on **stderr**, while the verdict goes to
stdout:

```
[warn] semantic: ⚠ 'stack.*.order' detected — execution order should move to 'plans.*.entries[].order'
  Migration guide: https://github.com/ScriptonBasestar/dva/blob/master/docs/40-declarative-stack-and-plans.md#11-migration
  Affected entries: infra
  Hint: stack is now a declaration store; execution order belongs in plans
```

That is a migration URL, an affected-entry list, and a remediation hint — structured data rendered
as prose, split across two streams, for an audience that is documented to be a program. An
assistant asked "is this config fine, and what should I fix?" gets a machine answer only when the
config is too broken to load, and prose in every case where there is something actionable to do.

## Proposed fix

Give `validate` a document of its own, printed when `jsonOutput` is set, covering all four rows:
a verdict, a `warnings` array with the fields the prose already encodes (message, affected
entries, hint, migration URL where present), and `errors` for the schema case so failure output
stops depending on a generic envelope that cannot describe *which* keys were wrong.

Two constraints the TASK-079 work already established and this must not break:

1. **Do not emit two documents.** `internal/output` records that a printer has written and the
   envelope yields when it has. If `validate` prints its own document and then returns an error,
   that machinery is what keeps stdout parseable — the same condition `doctor` already exercises.
2. **Do not move the human output.** Prose stays exactly where it is when the flag is absent;
   stderr stays byte-identical. This is an addition, not a rendering change.

## Non-goals

- Not changing warning *text*, thresholds, or which warnings fire.
- Not adding `--json` to other commands that lack it.
- Not resolving why warnings go to stderr while the verdict goes to stdout. Worth deciding, but
  the JSON document sidesteps it — see Left open.

## Acceptance criteria

- [x] Success under `--json` yields one document | verify: `dva validate --json 2>/dev/null | jq -s 'length'` on a clean fixture — must print 1; today the output is not JSON at all — **1** (216 bytes, was 21 bytes of prose)
- [x] Warnings appear as data | verify: `dva validate --json 2>/dev/null | jq '.warnings | length'` on the `stack.*.order` fixture — must be >0, and print the count so an empty array cannot pass as success — **2**, also printed by `TestValidateJSONCarriesWarningsAsData` via `t.Logf`
- [x] Schema errors are described, not just stringified | verify: `dva validate --json` on the two-key provision fixture — `.errors` must name `provision.default.0`; exit stays 1 — **two entries, both `provision.default.0`**, exit 1
- [x] Exactly one document in every path | verify: all four rows above through `jq -s 'length'` — each must print 1; `jq -e .` is not acceptable evidence — **1, 1, 1, 1**
- [x] Without `--json` nothing changes | verify: `human — capture stdout and stderr byte counts for all four paths before and after; both must match exactly` — all four rows identical against a HEAD binary built from `git archive`; stderr byte-identical by `diff` in all **eight** cells (see table)
- [x] The envelope does not double up | verify: `go test ./internal/cli/ -run JSON` — the concatenation test from TASK-079 must still pass, and print the number of tests selected — **52 RUN+PASS lines**, `TestFailureJSONEnvelopeReachesAProgram` green including `it yields to a document already on stdout`
- [x] Not vacuous | verify: `human — drop the already-printed guard and confirm the failure path emits two documents` — guard removed → schema-failure row emits **2**; guard present → **1**
- [x] Full suite passes | verify: `make test` — all packages ok, `internal/cli` coverage 61.9%

## Reproduction fixtures

Warnings + success (loads clean, warns about `stack.*.order`):

```yaml
version: "0.1.44"
stack:
  infra:
    order: 1
    default_runner: compose
    runners:
      compose:
        files: [does-not-exist.yml]
```

Schema failure (loads clean, violates `provision_item`'s legacy branch, which allows at most one
property):

```yaml
provision:
  default:
    - echo: "legacy echo form"
      cmd: "echo both-keys-at-once"
```

## Resolution

`validateReport` in the new `internal/cli/validate_json.go`, built by `validateCmd`'s RunE and
printed when `jsonOutput` is set. Measured against a HEAD binary built from `git archive HEAD`
into the scratchpad, so "before" is a real binary rather than a remembered number:

| row | mode | exit | stdout before → after | stderr before → after |
| --- | --- | --- | --- | --- |
| load failure | plain | 1 | 0 → 0 | 246 → 246 |
| load failure | `--json` | 1 | 297 → **297** | 246 → 246 |
| schema failure | plain | 1 | 0 → 0 | 172 → 172 |
| schema failure | `--json` | 1 | 224 → **660** | 172 → 172 |
| warnings | plain | 0 | 21 → 21 | 837 → 837 |
| warnings | `--json` | 0 | 21 → **1898** | 837 → 837 |
| success | plain | 0 | 21 → 21 | 0 → 0 |
| success | `--json` | 0 | 21 → **216** | 0 → 0 |

Only the `--json` rows moved, and stderr is byte-identical in all eight cells by `diff`, not
just by byte count. The load-failure row is unchanged on purpose: `mustLoadConfig` calls
`emitFailureJSON` and `os.Exit(1)` before RunE runs, so there is no report to build there and the
TASK-079 envelope remains the right answer.

Three design points that were not free:

- **`.error` survives inside the new document.** A consumer already reading `.error.message`
  from a failed `dva validate --json` keeps working, because printing the report suppresses the
  envelope — the guard is what would have silently broken them.
- **`Details` is the lossless copy, `Fields` is convenience.** Continuation lines are kept
  verbatim and *additionally* keyed when they follow the `Key: value` convention
  (`migration_guide`, `affected_entries`, `hint`). Measured on the no-plans warning: its nested
  YAML `Example:` block keeps its indent and correctly produces no fields, while its `Hint:` and
  `Migration guide:` lines do. A producer that stops following the convention loses a field and
  never loses text.
- **`composeNameWarningLines` was extracted** so the stderr prose and the JSON warning are the
  same sentences by construction. The alternative — a second set of format strings — is exactly
  the shape that produced [TASK-086](086-parallel-steps-discard-their-note.md)'s four divergent
  copies of one note.

Both lists are initialised non-nil, so a clean config serializes `"warnings": []` rather than
`null`; a consumer that iterates gets an empty loop instead of a null-iteration error.

### Non-vacuity

Each hunk reverted alone in a throwaway copy of the tree, one at a time:

| reverted | test | result |
| --- | --- | --- |
| RunE never prints the report | `TestValidateJSONDescribesASuccess` | fails |
| `report.add("semantic", …)` removed | `TestValidateJSONCarriesWarningsAsData` | fails |
| `fail()` stops building `.errors` | `TestValidateJSONNamesTheOffendingKey` | fails |
| compose warning text re-typed instead of shared | `TestComposeNameWarningPathsShareOneSource` | fails |

No test passes because of a different hunk. `decodeOneDocument` asserts *one* document with a
second `Decode` rather than a single `Unmarshal`, because `Unmarshal` cannot distinguish "one
object" from "garbage" the way the task's `jq -s 'length'` criterion requires.

## Left open

- **Warnings go to stderr, the verdict to stdout.** A consumer reading only stdout sees `✅` and
  learns nothing about four pending migrations. The JSON document makes that irrelevant for
  machines but leaves the human split unresolved. Same stream-choice question as
  [TASK-086](086-parallel-steps-discard-their-note.md)'s note on `hooks.go` vs `provision.go`.

## Related

- [TASK-079](../_archive/079-json-flag-does-not-cover-failures.md) — shipped the envelope that covers
  rows 1-2 here. Its *Left open* entry described this gap less precisely, before the envelope
  existed to measure against; corrected in the same commit as this file.
- [TASK-087](087-unrecognized-stack-args-become-entry-names.md) — the other half of the
  machine-consumer thread: an exit code that reports success over work that did not happen.
