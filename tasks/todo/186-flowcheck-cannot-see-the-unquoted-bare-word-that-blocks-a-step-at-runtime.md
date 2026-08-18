---
id: TASK-186
title: "flowcheck cannot see an unquoted bare word, the one flow-shell defect that only appears at runtime"
type: feature
priority: P1
effort: M
created-at: 2026-08-18T15:24:47+09:00
source: "TASK-183 implementation — the backup step was blocked twice before the rule was found"
scope: "dva repo — tools/flowcheck/rules.go, tools/flowcheck/rules_test.go"
status: todo
---

# Task 186: Catch the quoting defect before the run does

## Summary

am's shell policy analyzer reads an **unquoted bare word as a command name wherever it
appears**, not only in command position. Measured in `context` steps:

| shell | result |
| --- | --- |
| `[ -f dva.yml ]` | blocked: `command "dva.yml" not in allowlist` |
| `[ -f 'dva.yml' ]` | runs |
| `printf yes` | blocked: `command "yes" not in allowlist` |
| `printf true` | runs — only because `true` is allowlisted |

`am validate` reports the flow valid. The block appears at run time, names the offending
word but not the step's line, and gives no hint which `context:` key produced it — a
multi-key step has to be bisected one key at a time in a throwaway flow to find it.

The coincidence in the last row is what makes this worth a rule. `printf true || printf
false` is the required form for a `when:` gate producer, and it passes for an unrelated
reason: `true` and `false` happen to be allowlisted commands. That hides the actual
constraint, so the next flag someone writes as `printf yes` blocks, and the failure looks
like a gate defect rather than a quoting one.

This is the exact defect class `tools/flowcheck` exists for: silent under `am validate`,
expensive to diagnose, mechanically detectable.

## Completion Criteria

- [ ] A rule reports an unquoted bare word in a flow `shell`/`context` field | verify: `go test ./tools/flowcheck/...`
- [ ] The rule fires on `[ -f dva.yml ]` and stays silent on `[ -f 'dva.yml' ]` | verify: `go test ./tools/flowcheck/...`
- [ ] The rule does not fire on shell variables, expansions, or allowlisted command names | verify: `go test ./tools/flowcheck/...`
- [ ] The existing corpus passes unchanged | verify: `go run ./tools/flowcheck`

## Technical Notes

- Current rule ids: `dead-gate`, `gate-operand`, `gate-filter`, `gate-producer-newline`,
  `exit-if-empty`, `param-type`, `phantom-command`, `unguarded-report`. This adds a ninth.
- False positives are the risk. A word in command position is a command, and the allowlist
  lives outside the repo (`~/.config/agent-mesh/sandbox_override.yaml`), so the rule cannot
  resolve which names are legal. Reporting only *arguments* that are bare words — not the
  leading word of a command — keeps it decidable without reading the operator's config.
- Allowlisting a filename is not the fix; quoting it is.
