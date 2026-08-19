---
id: TASK-196
title: "The index of generated blocks undercounts the generator"
type: docs
priority: P3
effort: XS
created-at: 2026-08-19T13:50:00+09:00
source: "measured 2026-08-19 — README says two blocks, three AUTOGEN markers exist and libgen writes all three"
scope: "agent-mesh-flows/shared/library/README.md only — no change to tools/libgen"
status: todo
---

# Task 196: The index of generated blocks undercounts the generator

## Summary

`agent-mesh-flows/shared/library/README.md` states that **two** blocks in
`shared-guardrails.md` are regenerated, and names Rule 9 (`reserved_commands`) and Rule 14
(`section_order`).

There are **three**. `tools/libgen/main.go` calls `replaceBlock` at lines 41, 45 and 49, the
third being `version_rule` (Rule 4), and `shared-guardrails.md` carries all three
`AUTOGEN:*:start` markers.

The README's only job is to tell an editor which regions are machine-owned. An index that is
short by one is not a small inaccuracy — it actively points someone at Rule 4 as
hand-editable, and `make check-generate` will then reject their change with no explanation of
why that particular paragraph refused to stay edited.

## Completion Criteria

- [ ] The README names all three regenerated blocks, including `version_rule` / Rule 4 | verify: `grep -c 'version_rule\|Rule 4' agent-mesh-flows/shared/library/README.md`
- [ ] The README's count matches both the markers in the corpus and the generator's writes | verify: `grep -rho 'AUTOGEN:[a-z_]*:start' agent-mesh-flows/shared/library/*.md | sort -u | wc -l; grep -c 'replaceBlock(out,' tools/libgen/main.go`
- [ ] `make check-generate` still exits 0 | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && make check-generate; echo "exit: $?"`

## References

- `agent-mesh-flows/shared/library/README.md` — the "Two blocks … are regenerated" sentence
- `tools/libgen/main.go:41,45,49` — the three `replaceBlock` calls; this file is the authority
- `agent-mesh-flows/shared/library/shared-guardrails.md` — the three marker pairs
- `make generate` / `make check-generate` in the Makefile

## Open Questions

- Worth considering whether the README should stop counting at all and instead state the rule
  ("every `AUTOGEN:*` region is machine-owned"), so the next block added to libgen cannot make
  this document wrong again. That is a slightly larger edit than the factual fix.

## Technical Notes

- Effort is XS and the size gate would normally fold this into a related task, but the two
  other cards saved alongside it touch different subsystems (task-card validation, flowcheck
  docs) and this repo's convention is one file per task. Kept standalone deliberately.
- Do not edit the generated regions themselves while fixing the prose around them.
