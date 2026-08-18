---
id: TASK-187
title: "Two of the five when-contract rules are documented but nothing enforces them"
type: feature
priority: P2
effort: M
created-at: 2026-08-18T15:24:47+09:00
source: "63ee185 — flowcheck enforces rules 1, 2 and 5 only"
scope: "dva repo — tools/flowcheck/rules.go, tools/flowcheck/rules_test.go, agent-mesh-flows/"
status: todo
---

# Task 187: Enforce the rest of the gate contract

## Summary

The `when:` contract recorded in `dva-improve-guided.yaml` has five rules. `flowcheck`
enforces three of them — the quoted operand (`gate-operand`), the absent filter
(`gate-filter`), and the `printf` producer (`gate-producer-newline`). Two are prose only:

- **Rule 3** — a skip propagates into a dependent only when that dependent carries its own
  `when:`. Propagation is per-step, not per-type. A step that should have been skipped and
  was not is invisible; it just runs.
- **Rule 4** — a skipped step's key renders as the literal text `{{ref}}`, not as empty.
  This is the more dangerous of the two: the literal template string is interpolated into
  whatever reads it, so a skipped producer feeding an `llm` step puts `{{step.key}}` into
  the prompt and the model answers around it. There is no error, and the output looks
  plausible.

Every failure in this family exits 0 and passes `am validate`. Prose has already proven
insufficient for rules 1, 2 and 5 — each was written down before it was violated.

## Completion Criteria

- [ ] A rule reports a consumer that reads a key from a gated step without a gate of its own | verify: `go test ./tools/flowcheck/...`
- [ ] A rule reports an `llm` or `file` field interpolating a key whose producer can be skipped | verify: `go test ./tools/flowcheck/...`
- [ ] Each new rule has a positive and a negative test case | verify: `go test ./tools/flowcheck/...`
- [ ] The existing corpus passes unchanged | verify: `go run ./tools/flowcheck`
- [ ] The contract comment marks all five rules as mechanically enforced | verify: `grep -q 'enforces rules 1, 2, 3, 4 and 5' agent-mesh-flows/dva-improve-guided.yaml`

## Technical Notes

- Rule 4 needs a reachability question answered first: which steps can be skipped. That is
  the transitive closure over `depends_on` from any step carrying a `when:`, restricted by
  rule 3 — a dependent without its own `when:` runs anyway, so the closure stops there.
- `backup_config` in `dva-improve.yaml` is a live example of correct rule-3 handling: it
  carries its own `when:` and was observed skipping when `backup_marker` skipped.
- Related: TASK-186 adds a ninth rule to the same tool; land whichever is ready first and
  rebase the other.
