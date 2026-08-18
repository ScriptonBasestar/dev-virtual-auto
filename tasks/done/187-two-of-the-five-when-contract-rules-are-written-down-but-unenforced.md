---
id: TASK-187
title: "Two of the five when-contract rules are documented but nothing enforces them"
type: feature
priority: P2
effort: M
created-at: 2026-08-18T15:24:47+09:00
completed-at: 2026-08-18T21:10:00+09:00
source: "63ee185 — flowcheck enforces rules 1, 2 and 5 only"
scope: "dva repo — tools/flowcheck/, agent-mesh-flows/dva-improve-guided.yaml"
status: done
---

# Task 187: Enforce the rest of the gate contract

## Summary

The `when:` contract recorded in `dva-improve-guided.yaml` has five rules. `flowcheck`
enforced three of them — the quoted operand (`gate-operand`), the absent filter
(`gate-filter`), and the `printf` producer (`gate-producer-newline`). Two were prose only:

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

- [x] A rule reports a consumer that reads a key from a gated step without a gate of its own | verify: `go test ./tools/flowcheck/...`
- [x] A rule reports an `llm` or `file` field interpolating a key whose producer can be skipped | verify: `go test ./tools/flowcheck/...`
- [x] Each new rule has a positive and a negative test case | verify: `go test ./tools/flowcheck/...`
- [x] The existing corpus passes unchanged | verify: `go run ./tools/flowcheck`
- [x] The contract comment marks all five rules as mechanically enforced | verify: `grep -q 'enforces rules 1, 2, 3, 4 and 5' agent-mesh-flows/dva-improve-guided.yaml`

## What the runtime actually does

Measured against am cb8b4ce with two probe flows before writing any rule. `off_flag`
produces `false`, `gated` carries `when: "{{off_flag.v}} == 'true'"` and is therefore
skipped, and every case below exits 0.

| step under test | shape | result |
|---|---|---|
| `dep_ungated` | no `when:`, `depends_on: [gated]` | **ran**, printed `k=[{{gated.k}}]` |
| `dep_own_when_true` | `when:` true, `depends_on: [gated]` | skipped — `dependency 'gated' was skipped` |
| `reads_without_depends` | `when:` true, reads `{{gated.k}}`, **no** `depends_on` edge | **ran**, printed the literal |
| `mid` → `chain2` | both gated, chained through `gated` | both skipped — propagation is transitive |

The second row is the one that matters: `dep_own_when_true`'s own gate evaluated **true**
and it was skipped anyway. Propagation overrides a step's own gate rather than being
consulted alongside it.

## Resolution

The card's first criterion said a consumer is at fault when it lacks "a gate of its own".
Row 3 shows that is not sufficient — `reads_without_depends` had a true gate and ran with
the literal in hand, because propagation travels `depends_on`, not gate presence. The
implemented condition is therefore stricter than the card asked for:

> A consumer reading `{{G.key}}`, where `G` carries a `when:`, is safe **iff** some
> `depends_on` path `G → … → consumer` exists on which every step after `G` is gated.

That also collapses the technical note's "transitive closure over `depends_on` from any
step carrying a `when:`": *skippable* is simply *carries its own `when:`*, and the closure
is a reachability walk that stops at the first ungated step.

Two rule ids, one detector, split by where the literal ends up:

| rule | fires on | why it is separate |
|---|---|---|
| `gate-skip-leak` | any unprotected consumer field | the literal stays inside the pipeline |
| `gate-skip-prompt` | `instruction`, `prompt`, `file.path`, `file.content`, `file.from`, `file.to`, `src` | the literal is handed to a model or written to disk, where it outlives the run |

`tools/flowcheck/rules.go` had grown past the 500-line limit, so it was split along the
lines the rules already followed: `rules.go` (walk and the step graph), `shell.go`
(shell-text rules and the tokeniser), `gate.go` (the five `when:` rules). Test cases moved
with them into `gate_test.go`.

## Evidence

| claim | evidence |
|---|---|
| the rule reproduces am's measured behaviour | run against the two probe flows: fires on `dep_ungated` and `reads_without_depends`, silent on `dep_own_when_true` and the all-gated chain — the same four verdicts the runs produced |
| the tests discriminate rather than pass | mutation check: `skipReaches → true` kills exactly the 6 positive cases, `skipReaches → false` kills exactly the 3 negatives that reach it |
| the corpus verdict is not vacuous | `3 skippable reference(s)` now printed on the summary line beside the verdict; without a count, "no findings" cannot be told apart from "nothing in scope" |
| the corpus is genuinely clean | the 3 are `deterministic_check_1.structural_errors` and `deterministic_check_2.structural_errors` (direct gated dependents) and `validate_pass2.result`, protected two links out through `deterministic_check_2` — the transitive case, checked by hand against `dva-improve.yaml:843-900` |

Repo checks: `go test ./tools/flowcheck/...` ok · `go vet` clean · `gofmt -l` empty ·
`go run ./tools/flowcheck` → `10 flow file(s), 101 shell field(s), 12 when-gate(s), 3
skippable reference(s)` / `OK — no decision-path defects`.

## Technical Notes

- `{{analyze.output}}` in `dva-improve-guided/00-analyze.yaml` looks like a cross-file read
  of the gated `analyze` step in the parent flow. It is not: `00-analyze.yaml` defines its
  own ungated `analyze` at line 317. References the scanned file does not define — `param.*`
  and genuine cross-file pipeline reads — are skipped rather than guessed at, so a flow
  that reads a gated step in *another* file stays outside what this tool can see.
- `backup_config` in `dva-improve.yaml` is a live example of correct rule-3 handling: it
  carries its own `when:` and depends on `backup_marker`, so a skip reaches it.
