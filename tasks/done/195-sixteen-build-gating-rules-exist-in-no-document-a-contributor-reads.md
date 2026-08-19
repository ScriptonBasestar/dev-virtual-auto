---
id: TASK-195
title: "Sixteen build-gating rules exist in no document a contributor reads"
type: docs
priority: P2
effort: S
created-at: 2026-08-19T13:45:00+09:00
source: "measured 2026-08-19 — grep -rln --include='*.md' flowcheck . | grep -v tasks/ returns nothing"
scope: "docs/ page + a link from AGENTS.md; no change to tools/flowcheck behaviour"
status: done
completed-at: 2026-08-19T14:28:13+09:00
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

- [x] A page under `docs/` documents all 16 rules; each entry gives the rule id, the am behaviour that makes the mistake silent, and a wrong/right example | verify: human — read the page and confirm each rule has all three
- [x] Every rule id the binary can emit appears in that page | verify: `for id in $(grep -rhoE '(s\.add\(|rule :?= |rule: *)"[a-z-]+"' tools/flowcheck/*.go | sed 's/.*"\(.*\)"/\1/' | sort -u); do grep -q -- "$id" docs/*flowcheck*.md || echo "MISSING $id"; done` (no output = pass; the id list must come out to 16 today)
- [x] flowcheck is documented where a contributor looks, not only in `tasks/` | verify: `grep -rln --include='*.md' flowcheck docs/ AGENTS.md`
- [x] `make doc-check` passes | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && make doc-check`

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

## Resolution

`docs/51-flowcheck-rules.md` (118 lines, 8559 bytes — inside the doc gate's 500-line/10240-byte
limit) documents all 16 rules, and `AGENTS.md` links it from a new **Flow decision-path gate
(flowcheck)** section placed beside the documentation gate.

The id count was re-derived rather than copied from this card: the AC2 extraction returns
**16** ids today, and each appears in the page. Both the page and AGENTS.md carry that
extraction command rather than a hand-kept list, so the next rule that lands shows up as a
`MISSING` line instead of silently falling out.

Three shapes the page keeps, each for a measured reason:

- **Grouped by the am behaviour, not alphabetically** — the five `when:` rules share one
  contract, the three shell rules share one allowlist, and reading them apart loses why each
  exists.
- **`config-probe-drift` is prose, not a table row.** It reads the whole corpus rather than one
  field, so "wrong → right" for it is about a set of four copies agreeing, and the same cell
  shape as the others would have misdescribed it.
- **The summary counts are documented as part of the contract.** A rule that matches nothing
  reads exactly like a rule that passed, which is why the count sits beside the verdict.

Two accuracy fixes fell out while writing it. `AGENTS.md` described `make doc-check` as
"repo-wide markdown links + docs/workflows size" while the target has run three gates
(`doccheck`, `cilabels`, `flowcheck`) — corrected. And the `exit-if-empty` example's `||`
was unescaped inside a table cell, which markdown would have rendered as two empty columns;
every row now splits to exactly three cells under a pipe-escape-aware check.

Non-goals are stated on the page: flowcheck reads am *semantics*, `am validate` reads the
schema, and prompt bodies are deliberately out of scope.
