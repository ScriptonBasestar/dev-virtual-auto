---
id: TASK-134
title: "Naming presets and forbidden ports are hand-maintained in the flow library with no Go source of truth"
type: chore
priority: P3
status: done
effort: S
decision: B
created-at: 2026-08-03T12:05:00+09:00
source: "TASK-061 finalize verification — the deferred Phase 2 existed only as README prose"
depends-on: [TASK-061]
scope: "dva repo — agent-mesh-flows/shared/library/, tools/libgen, internal/config"
---

# Task 134: Give the last two hand-authored flow-library facts a Go source

## Problem

TASK-061 removed the hand-copying of Go facts into the agent-mesh flow library for
two rules, and left two more explicitly out of scope:

- **Rule 23** — naming presets
- **Rule 7** — forbidden ports

Both are still typed by hand into `agent-mesh-flows/shared/library/`, with no Go
source of truth and no `AUTOGEN` block. They can drift from the validator exactly
the way rules 9 and 14 did before TASK-061, and nothing would catch it — the
`make check-generate` gate only diffs what the generator writes.

The deferral is recorded, but only as narrative prose:
`agent-mesh-flows/shared/library/README.md:29` — "Facts still authored here (Phase
2 migration candidates)". A finalize sweep of TASK-061 on 2026-08-03 searched
`tasks/todo`, `tasks/plan`, `tasks/blocked` and the whole repo for any ROADMAP or
BACKLOG entry and found none. This file exists so the work is discoverable rather
than living in a paragraph.

## Two acceptable outcomes

This is a decision task first, work second.

**A — migrate.** Give each rule a Go source in `internal/config` (as `reserved.go`
and `validate_warnings.go` are for rules 9 and 14), teach `tools/libgen` to emit
its `AUTOGEN` block, and add both to the `check-generate` diff set.

**B — drop the framing.** If neither rule is worth a Go source — e.g. they are
guidance rather than anything the validator enforces — remove the "Phase 2
migration candidates" heading from the README so it stops advertising work nobody
intends to do, and say plainly that these two are authored here on purpose.

Do not leave the third state, which is what exists today: a stated intention with
no owner and no tracking.

## Acceptance criteria

- [x] Outcome A or B is chosen and recorded in this file's frontmatter as `decision:`
      — **B**.
- [ ] n/a (outcome B) — If A: rule 23 and rule 7 each have a Go source of truth under
      `internal/config`.
- [ ] n/a (outcome B) — If A: `tools/libgen` emits both blocks, and `make generate`
      twice in a row leaves the tree clean (`make check-generate` exits 0).
- [ ] n/a (outcome B) — If A: `agent-mesh-flows/shared/library/README.md` moves both
      rules from the "still authored here" list to the generated list.
- [x] If B: the "Phase 2 migration candidates" section is gone from README.md:29
      and replaced by an explicit statement that these facts are authored by hand
      deliberately.
- [x] `make test` exits 0.

## Notes

Phase 2-B is already settled and needs nothing here: `dva-schema.md` moved to
`skills/config/references/schema-reference.md` and is symlinked back into the
library (README.md:32-33).

## Result

**Decision: B.** Both rules stay hand-authored, and the README now says so instead of
advertising a migration.

### Why B, measured rather than assumed

Rules 9 and 14 earned generation because Go *is* the authority for them:
`IsReservedCommand` really rejects an interaction key, `CanonicalSectionOrder` really
orders the file. A hand-copied list can therefore be **wrong about what DVA does**, and
`make check-generate` catches that.

Rules 7 and 23 have no behaviour to be wrong about. Probe: a throwaway config whose
compose binds 5432, 6379, 8080, 9092 and 9200 as **host** ports, with one service tagged
`nonsense-tag`, `default_plan: wildly-nonstandard-plan-name`, `environment: banana`, and
elasticsearch (`tags: [data]` — Tier 4) selected into that default plan:

```
$ dva validate
[warn] semantic: ⚠ 'stack.*.order' detected — execution order should move to 'plans.*.entries[].order'
  ...
✅ dva.yml is valid            exit 0, 4 lines of output
matches in that output: port 0 · 5432 0 · 6379 0 · 8080 0 · 9092 0 · 9200 0 ·
                        tier 0 · elasticsearch 0 · data 0 · banana 0 · wildly 0 · nonsense 0

$ dva doctor
  4 passed, 1 failed            10 lines, "port" mentions: 0
```

Zero enforcement, and the output was not empty — the validator did emit a real warning
about something else, so the zero counts are a measurement, not a silent no-op.

Giving either rule a Go const would put the source of truth downstream of its only
reader: `check-generate` would diff markdown against a Go copy of that same markdown,
which catches nothing. They become generation candidates the day the validator enforces
them — a product decision, not cleanup. Projects bind 5432 on the host deliberately, and
rule 23 itself says plan names should follow the project's own vocabulary.

### What the measurement turned up on the way

The drift the task worried about had already happened — but between two hand-authored
copies, not between markdown and Go. Rule 7 was stated twice with different content, and
`make generate` concatenated both into the same file a flow reads:

```
internal/cli/library_reference.txt:31   ...as host ports: 2181, 3000, 3306, 5432, 6379,
                                        8080, 8443, 9090, 9092, 9200, 15672, 27017.   (12)
internal/cli/library_reference.txt:181  ...(5432, 6379, 8080, 3000, etc.) as host ports.
                                        Use project-specific port ranges (11100-11199).  (4)
```

A Go const would have made that three copies, not one truth. Both now carry the same
sentence — the 12-port list plus the range advice the schema copy had and the guardrail
lacked — and `TestPortConventionRuleStatedOnce`
(`internal/config/library_corpus_test.go`) keeps them that way. It names no port numbers:
it groups every statement of the rule found in `generatorCorpus()` by wording and fails
when there is more than one group. Hand-authored, but not unowned.

Two further defects in `naming-presets.md`, both the same class (prose asserting things
nothing checks):

| Line | Was | Now |
| ---- | --- | --- |
| 31 | "`dva config validate`가 default plan에 Tier 2+ 서비스가 포함되면 경고한다" | measured false — the probe's `default_plan` carries kafka and validate is silent. Reworded to say Tier is a generation-time judgement DVA does not check. |
| 49 | "`plans:` 섹션 필수(규칙 4)" | 규칙 4 is `version:`; the plans rule is 2. Line 5 of the same file already cited 규칙 2 correctly. |

Checked and left alone: `dva-schema 15` (line 52) and `dva-schema 212-213` (line 111) are
both correct, though the first is a *rule* number and the second a *line* number under
identical notation.

### Falsification

| # | Break | Result |
| - | ----- | ------ |
| F1 | Revert `schema-reference.md` rule 5 to its old 4-port wording — i.e. reproduce the repo state before this change | FAIL at `library_corpus_test.go:96`, one grouped error naming both wordings and all 5 sites |
| F2 | Rename the heading to `**Host port selection**` in one authored file, putting it out of the regex's reach | FAIL at `library_corpus_test.go:70` — "found 3 statements, want at least 4 … this test stopped guarding it" |

Disjoint failure paths: F1 hits the grouping assertion, F2 hits the vacuous-pass guard.
Neither substitutes for the other. Both restored via `Edit` (never `git checkout --`,
which would have discarded the uncommitted work in this tree).

The first version of the test compared every copy against `found[0]` and reported the
same single divergence three times, implicitly treating whichever file was walked first
as correct. Reading F1's *output* rather than its pass/fail is what surfaced that; the
grouped form replaced it before the second falsification run.

### Session review of 0d46a66

Reviewing the commit found three claims in it that the measurement did not yet back —
the same overclaiming shape TASK-172 was about, in my own prose this time:

| Claim as committed | Reality | Resolved by |
| ------------------ | ------- | ----------- |
| README: "binding 5432, **6379**, 8080 and 9092 as host ports" | 6379 appeared only as an `endpoints:` URL in the probe; it was never a host port binding | added redis to the probe, re-measured — the claim is now true, and 9200 named too |
| `naming-presets.md`: "default plan에 **Tier 4** 서비스를 넣어도 통과한다" | the probe's default plan carried kafka, which is Tier **2** | added elasticsearch (`tags: [data]`) and put `data` in the default plan's tag filter |
| Result: "Probe config (`scratchpad/134-probe/`)" | a session-local path that will not exist when anyone reads this | described inline instead |

Also corrected `wantAtLeast`'s comment, which enumerated five sites next to a constant of
four without saying why they differ. The floor is deliberately one below the real count so
that dropping the `dva-schema.md` symlink is not a failure while losing an authored copy —
worth at least two sites — is.

Re-measurement output is the block above; the probe now binds all five ports and carries a
Tier 4 service in its default plan, and `dva validate` is still silent about every one of
them.

### Gates

```
make test          ok — cli 68.6% · config 68.3% · exec 67.0% · lifecycle 62.5% ·
                        output 100% · runner 67.8% · doccheck 72.7%
make check-generate exit 0
make doc-check      exit 0 — 229 markdown, 520 links, 0 broken, 0 oversized
gofmt -l            0 files
go vet ./...        exit 0
```

### Changed

- `agent-mesh-flows/shared/library/README.md` — "Phase 2 migration candidates" section
  replaced by "Facts authored here on purpose", with the measurement, the trigger that
  would reopen the question, and a pointer to the consistency test.
- `agent-mesh-flows/shared/library/shared-guardrails.md` — rule 7 gains the range advice.
- `skills/config/references/schema-reference.md` — rule 5 now states the same sentence.
- `agent-mesh-flows/shared/library/naming-presets.md` — false Tier warning claim, stale
  rule cross-reference.
- `tools/libgen/main.go` — the "intentionally NOT touched here" comment now carries the
  reason and cites this task.
- `internal/config/library_corpus_test.go` — new.
- `internal/cli/library_reference.txt` — regenerated.
