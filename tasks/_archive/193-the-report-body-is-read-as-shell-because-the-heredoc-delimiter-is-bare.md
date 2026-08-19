---
id: TASK-193
title: "The report body is read as shell because the heredoc delimiter is bare"
type: bug
priority: P1
effort: S
created-at: 2026-08-18T19:20:00+09:00
completed-at: 2026-08-18T20:05:00+09:00
quality-review: pass
quality-reviewed-at: 2026-08-19T14:10:00+09:00
verified-at: 2026-08-19T14:20:47+09:00
archived-at: 2026-08-19T14:20:47+09:00
quality-review-evidence: |
  - kind: automated
    command-or-step: "AC1 — save_report.action parsed out of the YAML, template refs substituted, executed under /bin/sh"
    result: exit 0, prints `Execution report saved.`, writes tmp/improve-guided/40-execution-report.txt holding both `=== DVA Status ===` and `=== DVA Show ===`
  - kind: automated
    command-or-step: "AC1 structural — heredoc terminator position after YAML block-scalar dedent"
    result: `<<'EOF'` at dedented line 8 and `EOF` at column 0 on line 14; POSIX `<<` requires an unindented terminator, and the block-scalar dedent supplies it, so the heredoc closes
  - kind: automated
    command-or-step: "go run ./tools/flowcheck (AC2) + grep for bare `<<` delimiters across agent-mesh-flows/"
    result: exit 0 over 103 shell fields; the only `<<[A-Za-z]` hit repo-wide is the string `<<<SEARCH` inside flow.schema.json's description text — a here-string, which the rule discards by design, and not a shell field
  - kind: automated
    command-or-step: "go test ./tools/flowcheck/ -run TestHeredocDelimiter (AC3)"
    result: exit 0 — 5 subtests: bare delimiter fires, single- and double-quoted forms stay silent, `<<-` is the same defect, `<<<` here-string is not a heredoc
  - kind: automated
    command-or-step: "am validate agent-mesh-flows/dva-improve-guided/40-execute.yaml (AC4)"
    result: exit 0
  - kind: manual
    command-or-step: "cross-card check — the comment-apostrophe -> comment-quote rename this card performs"
    result: confirmed in git: 499bd6b introduced `comment-apostrophe`, 55c2eb0 renamed it to `comment-quote`. 16 rule ids now live in tools/flowcheck, matching the set TASK-195 is filed to document
source: "TASK-192 — the corpus-wide am sweep that closed 192 left exactly one blocked step"
scope: "dva repo — agent-mesh-flows/dva-improve-guided/40-execute.yaml, tools/flowcheck"
status: done
verification-summary: |
  quality-review pass, re-observed at disposition. Both AC bindings exit 0 and
  TestHeredocDelimiter still reports 5 passing subtests. The structural claim re-measured
  rather than re-read: 40-execute.yaml:126 opens `<<'EOF'` and :132 closes `EOF`, both at
  indent 6 — the block-scalar base — so after YAML dedent the terminator sits at column 0 and
  POSIX `<<` closes it. The only `<<[A-Za-z]` match left in agent-mesh-flows/ is the `<<<SEARCH`
  here-string inside flow.schema.json's description text, which is not a shell field.
---

# Task 193: The report body is read as shell because the heredoc delimiter is bare

## Summary

`save_report.action` on the guided execute stage wrote its report with `<<EOF`. am reads
the heredoc *body* as shell, so the first line of the report text was taken for a command:

```
blocked: shell policy: command "DVA" not in allowlist
```

The line is `=== DVA Status ===`. Nothing about it is code. The step is the last one in
`40-execute.yaml`, so the execution report the stage exists to produce had never been
written — and the run still ended `Done`, because a blocked step prompts, defaults to
continue, and produces nothing.

## Resolution

The delimiter is quoted: `<<'EOF'`. Quoting also stops the shell expanding `$VAR` inside
the body, which costs nothing here — the two values it interpolates are am template
references, substituted before the shell sees the text.

Two flowcheck changes hold it:

- `heredoc-delimiter` — a heredoc opened with a bare word. `<<-` is the same defect;
  `<<<` is a here-string with no body and is discarded explicitly, because RE2 has no
  lookahead to keep the pattern from matching it.
- `comment-apostrophe` became **`comment-quote`**. Closing TASK-192 established that an
  apostrophe in comment prose flips quote parity for the rest of the field; measuring the
  double quote while writing this task showed it does exactly the same to an `awk "..."`.
  The rule now reports both characters, and the four comment lines in the corpus carrying
  a double quote were rewritten without one.

The comment rules and their tests moved to `comment.go` / `comment_test.go`. `shell.go`
had reached 468 lines against a 500-line ceiling, and the split falls on a real seam:
those rules read the field before comments are blanked, every other rule reads it after.

## Completion Criteria

- [x] The step runs and writes its report | verify: human — extract the field and run it through `am`; the report file exists and holds both sections
- [x] No flow field opens a heredoc with a bare delimiter | verify: `go run ./tools/flowcheck`
- [x] flowcheck fails on a bare-delimiter heredoc | verify: `go test ./tools/flowcheck/`
- [x] Flows still validate | verify: `am validate agent-mesh-flows/dva-improve-guided/40-execute.yaml`

## Evidence

**The corpus-wide sweep is now clean.** All 79 shell fields in `agent-mesh-flows/`
extracted into one probe flow and run under am:

| | blocked steps |
|---|---|
| before TASK-191 | 4 steps / 6 fields (`comment-substitution`) |
| after TASK-191 | 3 (`yaml_block_keys`) |
| after TASK-192 | 1 (`DVA`, this task) |
| after TASK-193 | **0** |

The report the blocked step exists to write is produced by the same run:
`tmp/improve-guided/40-execution-report.txt`, 95 bytes, both sections present.

**flowcheck against the pre-fix corpus**: 1 `heredoc-delimiter` and 8 `comment-quote`
findings. Against the fixed corpus: `OK — no decision-path defects`.

**The tests bind.** Each change stubbed in turn:

| stubbed | failing cases |
|---|---|
| `heredoc-delimiter` rule | 2 |
| the `<<<` discard | 1 |
| the double quote in `comment-quote` | 1 |

## Technical Notes

- Measured against am cb8b4ce: `<<EOF` with the body `=== DVA Status ===` blocks on `DVA`,
  the body `hello world` blocks on `hello`, and `<<'EOF'` with the same body runs.
- Also measured, and the reason `comment-quote` covers both characters: a comment holding
  one double quote blocks a following multi-line `awk "..."` on `BEGIN`; a second double
  quote in the same comment makes it run; and an apostrophe cannot close a double quote,
  so the two parities are independent and neither is maintainable by hand.
