---
id: TASK-088
title: "`dva validate --json` emits JSON only when it fails, and only via the generic envelope — its verdict and every warning stay prose"
type: fix
priority: P3
effort: M
status: todo
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/cli/validate.go — 0 references to jsonOutput; the verdict at :70 and the warnings at :42/:84/:88"
---

# Task 088: the one command whose whole output is a machine answer does not produce one

## Problem

`internal/cli/CLAUDE.md` states `--json` exists for the LLM pipeline.
`internal/cli/validate.go` contains **zero** references to `jsonOutput` — the flag is accepted and
never consulted. What JSON a consumer does get comes entirely from
[TASK-079](../done/079-json-flag-does-not-cover-failures.md)'s failure envelope, which catches the
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

- [ ] Success under `--json` yields one document | verify: `dva validate --json 2>/dev/null | jq -s 'length'` on a clean fixture — must print 1; today the output is not JSON at all
- [ ] Warnings appear as data | verify: `dva validate --json 2>/dev/null | jq '.warnings | length'` on the `stack.*.order` fixture — must be >0, and print the count so an empty array cannot pass as success
- [ ] Schema errors are described, not just stringified | verify: `dva validate --json` on the two-key provision fixture — `.errors` must name `provision.default.0`; exit stays 1
- [ ] Exactly one document in every path | verify: all four rows above through `jq -s 'length'` — each must print 1; `jq -e .` is not acceptable evidence
- [ ] Without `--json` nothing changes | verify: `human — capture stdout and stderr byte counts for all four paths before and after; both must match exactly`
- [ ] The envelope does not double up | verify: `go test ./internal/cli/ -run JSON` — the concatenation test from TASK-079 must still pass, and print the number of tests selected
- [ ] Not vacuous | verify: `human — drop the already-printed guard and confirm the failure path emits two documents`
- [ ] Full suite passes | verify: `make test`

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

## Left open

- **Warnings go to stderr, the verdict to stdout.** A consumer reading only stdout sees `✅` and
  learns nothing about four pending migrations. The JSON document makes that irrelevant for
  machines but leaves the human split unresolved. Same stream-choice question as
  [TASK-086](086-parallel-steps-discard-their-note.md)'s note on `hooks.go` vs `provision.go`.

## Related

- [TASK-079](../done/079-json-flag-does-not-cover-failures.md) — shipped the envelope that covers
  rows 1-2 here. Its *Left open* entry described this gap less precisely, before the envelope
  existed to measure against; corrected in the same commit as this file.
- [TASK-087](../done/087-unrecognized-stack-args-become-entry-names.md) — the other half of the
  machine-consumer thread: an exit code that reports success over work that did not happen.
