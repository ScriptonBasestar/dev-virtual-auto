---
id: TASK-041
title: "DECISION: should 'dva stack status' exit non-zero when an entry can never run?"
type: bug
priority: P3
status: decision
needs-human: true
effort: S
created-at: 2026-07-17T07:10:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: TASK-038 implementation (decision raised, not assumed)
source-severity: LOW
moved-at: 2026-07-17T10:55:00+09:00
---

# Task 041: Status Now Says "BROKEN" To Humans And "Fine" To Scripts

## Summary

TASK-038 fixed `dva stack status` silently omitting entries whose plugin cannot be constructed.
Those entries are now printed, marked `unknown`, with a `BROKEN: <reason>` line and a `Warn`.

**The exit code was deliberately left at 0**, and that decision was explicitly deferred rather than
made by the implementer. This task carries it so it stays in the queue instead of being buried in
TASK-038's archived Outcome.

The question: should `dva stack status` exit non-zero when an entry's plugin cannot be constructed?

## Current behavior (measured at `fff7f71`, `dva validate` EXIT=0 on the probe)

```
$ dva stack status
  [s1] script
  [s2_tworunners] unknown
  BROKEN: unknown lifecycle plugin "" (implemented: [...], planned: [])
EXIT=0                          # <-- the open question

$ dva stack up            ->  EXIT=1  ERROR: entry "s2_tworunners": unknown lifecycle plugin ""
```

So `status` and `up` still disagree about the same config: one says fine, the other refuses.

## The case FOR non-zero

- TASK-038's own justification is an exit-code argument: a CI gate or readiness check "passes green
  on a stack that cannot start". The `BROKEN:` line fixes the **human** reading of status; a script
  running `dva stack status || alert` is still told everything is fine.
- This is not a transient runtime observation like "container is down". It is a **static config
  defect** — the entry can *never* run, on any invocation. All three sibling commands already treat
  it as fatal.

## The case AGAINST (why this was not decided unilaterally)

- It changes `status`'s contract. Today the exit code answers *"did the query run?"*. The proposal
  makes it answer *"is the stack sound?"* — a different question. Existing callers may depend on the
  current meaning.
- A distinct code (e.g. `2` = config defect, `1` = query failure) may serve callers better, but that
  is a CLI-wide convention decision, not a local one. DVA has no documented exit-code convention to
  follow — that absence is itself part of what needs deciding.

### Blast radius — measured, and it splits cleanly in two

**Seven** call sites reach `orch.Status()`, not five, and they are not equivalent. The split is the
whole shape of this decision:

| | Site | Role |
|---|---|---|
| **Dedicated status** | `internal/cli/stack.go:225` | `stack status` — `return err` on failure |
| | `internal/cli/plan_lifecycle.go:228` | `runPlanStatus` — `return err` |
| | `internal/cli/status.go:87` | `dva status`, text path |
| | `internal/cli/status.go:43` | `dva status`, **JSON** path |
| **Post-`up` summary** | `internal/cli/compose.go:169` | prints "regardless of up errors" |
| | `internal/cli/stack.go:104` | post-`stack up` summary |
| | `internal/cli/plan_lifecycle.go:165` | post-`runPlanUp` summary |

The three summary sites all use `if statusErr == nil { PrintStatus(...) }` — they **swallow** the
error deliberately, so users still see connection info for services that did start. That is the trap
this decision must not spring: making the *command* exit non-zero there would turn a successful `up`
into a reported failure, and the existing comment at `compose.go:166` says that is intentional.

### A machine-readable answer already exists — measured, and it weakens the FOR case

`status.go:43` is `dva status --json`: `statusData["stack"] = status.Entries`. Measured on the
Probe A config (`dva validate` EXIT=0), with the HEAD binary as control:

```
# HEAD (before TASK-038)                       <-- the broken entry is ABSENT from JSON too
{"Name": "s1", "Plugin": "script", "Services": null, "Health": null}

# fff7f71 (after TASK-038)
{"Name": "s1", "Plugin": "script", "Services": null, "Health": null, "Error": ""}
{"Name": "s2_tworunners", "Plugin": "", "Services": null, "Health": null,
 "Error": "unknown lifecycle plugin \"\" (implemented: [...], planned: [])"}
```

Two things follow, and both matter to this decision:

1. **TASK-038 fixed the JSON channel too**, which its Outcome did not claim. Before it, a machine
   consumer parsing `dva status --json` was not merely told "fine" — the entry did not exist in the
   output at all. There was no signal to key off.
2. **A machine-readable answer now exists without any exit-code change.** A script can read
   `.stack[].Error` today. So "scripts are told everything is fine" is now true only of the
   *exit-code channel*: a JSON consumer is told the truth, a `dva stack status || alert` consumer is
   not. Whether that remaining gap justifies changing `status`'s contract is precisely the judgment
   being asked for — and it is a narrower question than it looked before this was measured.

Side effect worth a maintainer's eye in its own right: `EntryStatus` carries no json tags, so the
field serializes as `"Error"` (capitalized, Go-style) and now appears as `"Error": ""` on **every**
healthy entry. If that JSON is a public surface, its shape changed additively without being
designed. That is not a defect today; it is a decision nobody made.

## Recommendation (weak — the implementer's proposal, not a maintainer's ruling)

Lean **non-zero, scoped to the dedicated status commands only** (`dva status`, `dva stack status`),
never on the post-`up` summary path.

Decide this **together with** the pre-flight observation recorded in TASK-038: `stack up` printed
`S1_UP` *before* failing on the broken entry, so a broken stack is left half-started. A pre-flight
construction pass over all entries would fix that *and* make this condition detectable before
execution — plausibly subsuming this question entirely. Deciding the exit code in isolation risks
being redone.

## Severity: LOW / P3

The harmless direction, and knowingly so: `status` under-reports to machines. Nothing is mutated or
destroyed, and after TASK-038 a human reading the output sees the truth. It is filed because the
gap between the human-readable and machine-readable answer is exactly the class of defect this run
exists to close — a green surface that certifies nothing.

## Completion Criteria

- [ ] DECISION recorded: does an unrunnable entry make `dva stack status` exit non-zero, and with which code? | verify: `human — maintainer picks one and records why; a documented "no, exit 0 is correct" closes this task legitimately`
- [ ] If NON-ZERO: `dva stack up` still exits 0 on a successful up whose post-up status summary contains a broken entry | verify: `human — this is the trap; assert a successful up is not turned into a failure by its own summary`
- [ ] If NON-ZERO: the choice is applied consistently across all seven orch.Status() callers, or the exceptions are recorded with reasons | verify: `human — dedicated: stack.go:225, plan_lifecycle.go:228, status.go:43 (JSON), status.go:87; summary: compose.go:169, stack.go:104, plan_lifecycle.go:165`
- [ ] The `dva status` JSON path is considered explicitly — it already exposes the new Error field, so it may need no exit-code change at all | verify: `human — run 'dva status' JSON output on the Probe A config from TASK-038; confirm .stack[].Error is present and decide whether that suffices for machine consumers`
- [ ] If NON-ZERO: a regression test asserts the exit code, proven to fail without the change | verify: `human — revert the change, confirm the new test FAILS for the right reason, restore, confirm it passes`
- [ ] TASK-038's surfacing behavior does not regress — the entry and its reason are still printed | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && go test ./internal/lifecycle/`
- [ ] `make test` and `go vet ./...` pass | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && make test && go vet ./...`

## References

- [038-stack-status-silently-hides-unconstructible-entries.md](../_archive/038-stack-status-silently-hides-unconstructible-entries.md) — the fix this decision was deferred from; contains the full proposal, the probes, and the half-started-stack observation this should be decided alongside
- [017-runners-docker-native-semantics.md](./017-runners-docker-native-semantics.md) — the reason entries reach this state at all: `runners:` shapes whose plugin never resolves
- [039-plan-entry-runner-resolved-then-discarded.md](./039-plan-entry-runner-resolved-then-discarded.md) — same run, same theme: validate-green, runtime-wrong
