---
id: TASK-193
title: "The report body is read as shell because the heredoc delimiter is bare"
type: bug
priority: P1
effort: S
created-at: 2026-08-18T19:20:00+09:00
source: "TASK-192 — the corpus-wide am sweep that closed 192 left exactly one blocked step"
scope: "dva repo — agent-mesh-flows/dva-improve-guided/40-execute.yaml, tools/flowcheck"
status: todo
---

# Task 193: The report body is read as shell because the heredoc delimiter is bare

## Summary

`save_report.action` on the guided execute stage writes its report with `<<EOF`. am reads
the heredoc *body* as shell, so the first line of the report text is taken for a command:

```
blocked: shell policy: command "DVA" not in allowlist
```

The line is `=== DVA Status ===`. Nothing about it is code. The step is the last one in
`40-execute.yaml`, so the execution report the stage exists to produce has never been
written — and the run still ends `Done`, because a blocked step prompts, defaults to
continue, and produces nothing.

This is the only blocked field left in the corpus: all 79 shell fields were extracted into
one probe flow and run under am while closing TASK-192, and this is the one that failed.

## Completion Criteria

- [ ] The step runs and writes its report | verify: human — extract the field and run it through `am`; the report file exists and holds both sections
- [ ] No flow field opens a heredoc with a bare delimiter | verify: `go run ./tools/flowcheck`
- [ ] flowcheck fails on a bare-delimiter heredoc | verify: `go test ./tools/flowcheck/`
- [ ] Flows still validate | verify: `am validate agent-mesh-flows/dva-improve-guided/40-execute.yaml`

## Technical Notes

- Measured against am cb8b4ce: `<<EOF` with the body `=== DVA Status ===` blocks on `DVA`,
  the body `hello world` blocks on `hello`, and `<<'EOF'` with the same body runs. Quoting
  the delimiter is the whole fix.
- Quoting the delimiter also stops the shell expanding `$VAR` inside the body. This body
  interpolates `{{verify_status.stack_status}}` and `{{verify_status.dva_show}}`, which am
  substitutes before the shell sees the text, so nothing here depends on shell expansion.
  Any future body that does would have to be written differently.
- The flowcheck rule wants the same shape as `local-function`: find `<<` followed by an
  unquoted word, and report it. `<<-` and `<<<` are separate spellings worth matching, and
  the tokenizer currently reads `<<` as a redirection operator that ends an argument list,
  so the rule is easier to write on the raw text than on the token stream.
