---
id: TASK-188
title: "Decide what to do about the 638 bare `am run` log entries that record no flow name"
type: chore
priority: P3
effort: XS
created-at: 2026-08-18T15:24:47+09:00
completed-at: 2026-08-18T21:40:00+09:00
quality-review: pass
quality-reviewed-at: 2026-08-19T14:12:33+09:00
verified-at: 2026-08-19T14:22:19+09:00
archived-at: 2026-08-19T14:22:19+09:00
quality-review-evidence: |
  - kind: manual
    command-or-step: "AC1 — the decision and its reasoning are on the card"
    result: pass — "## Decision: accept" states the outcome and three independent reasons (attribution is lost to the argument form not to interactivity; documented usage already takes the logged branch; snapshots are stronger evidence than the missing log line), and says why it is not raised upstream
  - kind: manual
    command-or-step: "AC2 — the raised-upstream branch"
    result: pass — condition not met, and the card says so in the criterion itself rather than leaving an unlinked claim
  - kind: manual
    command-or-step: "AC3 — what evidence exists for future runs instead"
    result: pass — the snapshot in backups/dva/ plus `diff` against the live file, bounded at ten by TASK-184. Both halves verified in this queue run's reviews of TASK-183 and TASK-184
  - kind: automated
    command-or-step: "invocation-form claim re-measured against ~/.agent-mesh/usage.log, same session, without installing anything"
    result: reproduced. `am run ./probe.yaml -y` at 14:10:33 and 14:10:39 each wrote a bare `am run`; `am run validate probe.yaml` at 14:12:08 wrote `am run validate`. A registry name is recorded, a filesystem path is dropped — the card's table, a different pair of invocations
  - kind: automated
    command-or-step: "corroborating log counts"
    result: 0 of 5838 lines carry a third TSV field, so the log offers attribution and nothing else. `am run validate` 4623, `am run dva dva-discover` 8, `dva-improve` 0 matches anywhere in the file. Bare `am run` now 891 against the title's 638 — the card already warned the figure is not a stable population, and this queue run's own probes added to it
  - kind: manual
    command-or-step: "documented-usage claim, checked against the repo"
    result: pass with a clarification. The six guard `Fix:` lines print the namespaced form (`am run dva dva-improve`, `am run dva dva-improve-guided <stage>`). CLAUDE.md:55-59 documents the bare form (`am run dva-improve`), which `am run --help` and `am flow list` confirm is a registered subcommand — so it resolves from the registry and takes the logged branch too. The claim holds; "registry form" covers both spellings
  - kind: manual
    command-or-step: "finding beyond the card (not a defect)"
    result: an invocation rejected at argument parsing left no line at all — the log records neither the flow name nor the attempt. Strengthens the accept decision rather than weakening it
source: "config-damage investigation — usage.log cannot attribute interactive runs to a flow"
scope: "decision only — no dva code; possible upstream issue against agent-mesh"
status: done
verification-summary: |
  quality-review pass, re-observed at disposition. All three criteria are human-verified against
  the card text: `## Decision: accept` (:51) carries the outcome and its three reasons, the
  raised-upstream branch states its condition was not met, and the replacement evidence is
  named. Log claims re-measured on the moving population: 5850 lines, still 0 with a third TSV
  field, 4624 `am run validate` and 8 namespaced `am run dva ...` recorded by name against 891
  bare `am run` — the form-not-interactivity finding reproduces. The evidence that replaces the
  missing attribution is still shipped: `backups/dva` appears 4x in each of the two improve
  flows, bounded at ten by TASK-184.
---

# Task 188: Close or accept the attribution gap

## Summary

Investigating whether past `dva-improve` runs had silently rewritten valid configs ended in
an evidence gap that cannot be closed from this repository. `am`'s `usage.log` records
`am run` with no flow name for a large share of its entries, so the question "did an improve
run alter a config that was already valid" has no answer in the record, and never will for
runs already made.

The gate defects that made those runs plausible are fixed (`63ee185`), and every future run
snapshots the config first (`4ec336b`). The gap is strictly historical.

## Decision: accept

Not raised upstream. Measuring the log before deciding changed what the gap actually is, and
that measurement is why accepting is the right answer rather than the cheap one.

**The name is lost to path invocation, not to interactivity.** The card assumed interactive
flow selection was what dropped it. It is the argument form. Measured by installing a
one-step probe flow into the registry, running it both ways, and reading the two lines it
wrote:

| invocation | line in `usage.log` |
| --- | --- |
| `am run probe188:probe188 -y` | `am run probe188 probe188` |
| `am run ./probe188.yaml -y` | `am run` |

A registry name is recorded; a filesystem path is dropped. The log corroborates that
reading on its own: `am run dva dva-discover` appears eight times and `am run validate`
4623 times, while nothing named `dva-improve` appears at all.

**Documented usage is already attributed.** Every invocation `CLAUDE.md` documents, and
every `Fix:` line the flows themselves print when a guard stops a run, uses the registry
form — `am run dva dva-improve`, `am run dva dva-improve-guided 30-configure`. A user
following either lands in the branch that logs the name. The bare entries are path runs and
ad-hoc probes, including every probe this repository's own task work made today.

**Future runs carry stronger evidence that does not depend on am at all.** Since `4ec336b`
each improve snapshots the config into `backups/dva/` under a timestamped name, bounded at
the newest ten by TASK-184. That beats the log line it would have replaced: a log line
proves a flow ran, while a snapshot proves what the config held immediately before it ran,
and `diff` against the live file proves exactly what changed.

So the upstream change would buy dva nothing it does not already have. It stays a fair ask
for agent-mesh on its own merits — a path invocation recording no reference makes later
audit impossible for every consumer — but filing it against another project is the
maintainer's call, not something to spend this card on.

## Completion Criteria

- [x] The decision is recorded with its reasoning | verify: human — the outcome (raise upstream / accept) and why is written in this card or under `tasks/decision/`
- [x] If raised upstream, the issue is linked | verify: human — issue URL recorded — condition not met, nothing raised; see Decision
- [x] If accepted, the card states what evidence would exist for future runs instead | verify: human — read the recorded decision

## Technical Notes

- `usage.log` is TSV: an RFC3339 timestamp, a tab, then the command with its positional
  arguments. No line in the file has a third field — 0 of 5778 — so flags, working
  directory, and outcome are absent from the record whatever the invocation form.
  Attribution is the only thing on offer, and only by name.
- The 638 in this card's title was counted when it was filed. The figure is not stable and
  should not be read as a population: bare entries accrue from any path invocation, and this
  repository's own probe runs added hundreds during the 191–184 work.
- Precedent that this class of damage is real, not hypothetical: `tasks/_archive/135-*`
  records `fix_version` unconditionally `sed -i`-rewriting `version:` in every config it
  touched.
- Nothing in dva can reconstruct the missing attribution retroactively, and nothing needs
  to: the configs the investigation was worried about are readable now, and git holds their
  committed history.
