---
id: TASK-212
title: "The schema reference tells readers a bare up runs everything when it is refused"
type: bug
priority: P3
effort: S
created-at: 2026-08-20T16:10:00+09:00
source: "surfaced by the TASK-207 impact re-sweep as the one line in the 106-file doc corpus that misdescribes the no-name plan gate; measured here on three fixtures"
scope: "skills/dva-config/references/schema-reference.md:721 and its generated copy internal/cli/library_reference.txt:897. Documentation only — no code change is proposed, and the behaviour described is correct as it stands."
status: done
completed-at: 2026-08-26T12:36:09+09:00
completion-summary: "Document bare up across no-plan, lone-plan, explicit-default, and ambiguous multi-plan configurations."
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "make generate && dva check-generate"
    result: "passed; only the canonical schema reference and its generated library copy changed"
  - kind: automated
    command-or-step: "make doc-check"
    result: "passed all Markdown, CI label, and flow decision-path gates"
  - kind: manual
    command-or-step: "compare replacement against the task's three measured configuration shapes"
    result: "states whole-stack fallback only with no plans, sole-plan implicit default, and refusal for several plans without default_plan"
quality-review: pass
quality-reviewed-at: 2026-08-26T12:37:11+09:00
quality-review-evidence:
  - "independent reviewer confirmed canonical and generated copies cover all no-name plan-selection shapes"
  - "only the schema reference and its generated embedded copy changed; no runtime source changed"
  - "generation, doccheck, stale-phrase, default_plan-count, and diff validations passed"
quality-review-receipt: tmp/task-management/direct/queue-run/task-212-review-receipt.json
archived-at: 2026-08-26T12:37:37+09:00
verified-at: 2026-08-26T12:37:37+09:00
verification-summary: "The schema reference and embedded library now describe bare up correctly for absent, sole, defaulted, and ambiguous plan sets."
---

# Task 212: The schema reference tells readers a bare up runs everything when it is refused

## Summary

`skills/dva-config/references/schema-reference.md:721` opens the lifecycle example
block with:

```bash
dva up                    # Start the default plan (or every declared entry if none)
```

The parenthetical is true for a config with no `plans:` section and false for the
shape it most looks like it is describing: several plans, none of them marked
`default_plan`. There a bare `dva up` is refused, and nothing runs.

The line is read by two audiences. `skills/dva-config/references/schema-reference.md`
is a skill page, and `agent-mesh-flows/shared/library/dva-schema.md` is a symlink
to it, so `make generate` copies the same sentence into
`internal/cli/library_reference.txt:897`, which the `am` flows read when they
propose a `dva.yml`. A flow that believes a bare `dva up` always starts something
can generate a multi-plan config and describe it as runnable.

## Measured

Three fixtures, `stack:` entries `s1`/`s2` whose `script.up` touches a marker.
Bare `dva up`, no arguments:

| `dva.yml` | rc | ran | the line predicts |
|---|---|---|---|
| no `plans:` at all | 0 | `s1_up`, `s2_up` | every declared entry — correct |
| one plan `p1`, no `default_plan` | 0 | `s1_up` only | unstated; the lone plan is an implicit default |
| plans `p1`+`p2`, no `default_plan` | 1 | nothing | every declared entry — **wrong** |

The refusal in the third row reads:

```
ERROR: multiple plans configured; specify one: dva up <p1|p2>
```

Same on both binaries — master `8c48687` and the TASK-207 branch — so this is
pre-existing and not a consequence of that card.

## Why the second row matters to the wording

`requirePlanSelection` (`internal/cli/plan_lifecycle.go`) gates on
`c.HasPlans()`, which is `len(c.Plans) > 0`, so reading that function alone
suggests one plan without a `default_plan` is refused too. It is not:
`Config.DefaultPlan()` (`internal/config/config.go:585-591`) falls back to the
sole plan — *"Otherwise a lone plan is the implicit default"*. Any rewrite of
this line has to carry that rule or it will be wrong in a new direction.

## What to change

One sentence that names all three shapes, or a shorter one that stops claiming
the case it gets wrong. The behaviour is not in question here — only the
description of it. Whatever wording is chosen, `make generate` has to run so the
`am`-facing copy agrees; editing `internal/cli/library_reference.txt` by hand
would be undone by the next generate.

Check while there: the same block's other lines (`dva up web`, `dva stop web`,
…) all pass a plan name and are unaffected, and `USAGE.md:200` states the gate
correctly already, so it can serve as the phrasing to converge on rather than a
second wording to maintain.

## Completion Criteria

- [x] The claim that a bare `dva up` runs every declared entry is gone from the skill page | verify: `grep -c 'every declared entry if none' skills/dva-config/references/schema-reference.md` returns 0 (today: 1)
- [x] The generated copy agrees, i.e. `make generate` was run rather than the file hand-edited | verify: `grep -c 'every declared entry if none' internal/cli/library_reference.txt` returns 0 (today: 1)
- [x] The replacement states the several-plans-no-default refusal | verify: `grep -c 'default_plan' skills/dva-config/references/schema-reference.md` returns ≥ 2 (today: 1 — the single occurrence is the `default_plan:` key in the schema table, not a statement about the gate, so a rewrite that mentions the gate must add one)
- [x] The replacement does not contradict the lone-plan implicit default | verify: human — read the new sentence against the three-row table above and say which row it covers
- [x] `make generate` leaves no other diff | verify: `git status --porcelain internal/cli/library_reference.txt` names only the expected file

## Resolution

The canonical reference now distinguishes all three no-name shapes: no `plans:` starts every
declared entry, one plan is the implicit default, and several plans require `default_plan` or
an explicit name. `make generate` propagated the same text to the embedded library reference;
no runtime behavior changed.

## References

- `skills/dva-config/references/schema-reference.md:721` — the line
- `internal/cli/library_reference.txt:897` — the generated copy read by `am` flows
- `agent-mesh-flows/shared/library/dva-schema.md` — the symlink that carries one into the other
- `internal/config/config.go:576-593` — `DefaultPlan`, including the lone-plan fallback
- `internal/cli/plan_lifecycle.go` — `requirePlanSelection`, the refusal being described
- `USAGE.md:200` — the same gate, stated correctly

## Technical Notes

The doc corpus sweep that found this counted 106 files and 0 stale statements
about `dva restart`; this line survived that count because it is about `up`, not
`restart`. It is worth noting that the same sentence now also misdescribes
`restart`, which since TASK-207 refuses an empty name list under the identical
gate — one wording error, two commands.
