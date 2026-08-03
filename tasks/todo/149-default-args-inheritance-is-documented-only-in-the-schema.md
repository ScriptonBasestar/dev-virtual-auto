---
id: TASK-149
title: "The default_args inheritance rule appears in no document, and the JSON key that changed meaning has no test"
type: chore
priority: P3
status: todo
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

- [ ] `default_args` inheritance is documented where a user merging configs will find it —
      `docs/30-config-merge-semantics.md` or the document that owns subcommand resolution.
      State which, and why that is the canonical home rather than a second copy.
- [ ] The doc says what inherits and what does not, matching `interaction_tree.go:309`
      rather than the schema's shorter phrasing.
- [ ] Item 2 is resolved one way or the other: either the non-`command:` forms are excluded
      from inheritance to match the documented rule, or the rule is written to cover them.
      A comment at `interaction_tree.go:309` records the choice.
- [ ] A test asserts the `--json` plan's `arguments` value for both a bare child and a child
      inheriting `default_args`, so the meaning cannot shift again unremarked.
- [ ] `make test` and `make doc-check` exit 0 — the latter because this adds prose to an
      enforced path.

## Notes

The doc gate (TASK-090) enforces ≤500 lines / ≤10240 bytes under `docs/`.
`docs/30-config-merge-semantics.md` measured 345 lines / 9472 bytes on 2026-08-03 — roughly
770 bytes of headroom. Adding a full section will likely require the same split treatment
`docs/40` received, so plan for that rather than discovering it at `make doc-check`.
