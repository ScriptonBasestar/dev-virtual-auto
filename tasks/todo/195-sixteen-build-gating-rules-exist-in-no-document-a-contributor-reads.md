---
id: TASK-195
title: "Sixteen build-gating rules exist in no document a contributor reads"
type: docs
priority: P2
effort: S
created-at: 2026-08-19T13:45:00+09:00
source: "measured 2026-08-19 — grep -rln --include='*.md' flowcheck . | grep -v tasks/ returns nothing"
scope: "docs/ page + a link from AGENTS.md; no change to tools/flowcheck behaviour"
status: todo
---

# Task 195: Sixteen build-gating rules exist in no document a contributor reads

## Summary

`tools/flowcheck` now fails the build on **16 distinct rules**: `dead-gate`, `gate-operand`,
`gate-filter`, `gate-producer-newline`, `exit-if-empty`, `param-type`, `phantom-command`,
`unguarded-report`, `bare-word-arg`, `gate-skip-leak`, `gate-skip-prompt`,
`comment-substitution`, `local-function`, `comment-quote`, `heredoc-delimiter`,
`config-probe-drift`.

Outside `tasks/`, **no markdown file in this repo mentions flowcheck at all**. The only
prose explaining any rule lives in the `main.go` package comment and in closed task cards
under `tasks/done/`, which is where finished work goes to stop being read.

That matters more here than for a typical linter. Every one of these rules exists because am
fails *silently* in the corresponding case — a gate that can never open, a step whose comment
swallows the command below it, a probe that quietly stops matching. Someone who hits the
failure gets a rule id and a one-line message and has nowhere to go to learn what am actually
does. Worse, the next person writing a flow has no list of the traps, so the rules only teach
after the mistake, never before.

This is the same gap TASK-185 closed for the snapshot/restore path: the behaviour was
correct, the place to read about it did not exist.

## Completion Criteria

- [ ] A page under `docs/` documents all 16 rules; each entry gives the rule id, the am behaviour that makes the mistake silent, and a wrong/right example | verify: human — read the page and confirm each rule has all three
- [ ] Every rule id the binary can emit appears in that page | verify: `for id in $(grep -rhoE '(s\.add\(|rule :?= |rule: *)"[a-z-]+"' tools/flowcheck/*.go | sed 's/.*"\(.*\)"/\1/' | sort -u); do grep -q -- "$id" docs/*flowcheck*.md || echo "MISSING $id"; done` (no output = pass; the id list must come out to 16 today)
- [ ] flowcheck is documented where a contributor looks, not only in `tasks/` | verify: `grep -rln --include='*.md' flowcheck docs/ AGENTS.md`
- [ ] `make doc-check` passes | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && make doc-check`

## References

- `tools/flowcheck/main.go` — package comment; the rule ids are the string literals in each `finding{...}`
- `tools/flowcheck/corpus.go` — `config-probe-drift`, the only corpus-wide rule (it reads every flow at once, not one field), which the page must explain differently from the rest
- `tasks/done/183-…` through `tasks/done/193-…` — each card carries the measured evidence for the rule it added; the wrong/right examples can be lifted from them
- `AGENTS.md` — where the link belongs, alongside the other build gates
- Naming/size limits for docs: `skill:docs:doc-standards` (guide 100 lines/10KB, 500 lines/45KB)

## Open Questions

- Filename: `docs/51-flowcheck-rules.md` follows the numbered convention and sits next to
  `docs/50-…`, but confirm 51 is unused at the time of writing.
- Whether the page should also state the non-goals — flowcheck checks am *semantics*, not
  YAML schema (`am validate` does that) — so contributors stop expecting it to catch typos.

## Technical Notes

- The rule list changes every time a card lands, so prefer a verify binding that derives the
  id list from source (as above) over a hand-kept count. A doc that silently falls behind the
  binary is worse than none.
- The id extraction above is syntactic and covers the three forms the source uses today —
  `s.add("id", …)`, `rule := "id"`, and the corpus finding's `rule: "id"`. Measured while
  writing this card: matching only the first form yields 14, only `rule:` yields 1. A fourth
  emission form would drop out of the list silently and the check would then pass while under-
  counting, so re-derive the count when the page is written and state it there.
- `make doc-check` already knows about flowcheck; adding a page must not break it.
