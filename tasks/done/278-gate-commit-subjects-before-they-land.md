---
id: TASK-278
title: "Gate commit subjects before they land"
type: chore
priority: P1
effort: M
exec-tier: standard
created-at: 2026-09-03T12:30:00+09:00
source: "PLAN-004 review: `make commit-check` failed on master while 7 open cards bind it as a completion gate"
scope: "tools/commitcheck message-file mode, .githooks/commit-msg, Makefile install-hooks and (CI) label, .github/workflows/ci.yml commit-subjects job"
status: done
depends-on: []
---

# Task 278: gate commit subjects before they land

## Summary

`make commit-check` existed only as a Makefile target — not in CI, not in a git hook — so it
could report a violation but never prevent one. Five violating subjects landed on master in a
single day, and because 7 open cards bind `make commit-check` as a completion gate, none of
them could be closed. This card pairs the only remaining repair for published subjects
(waivers) with a gate at the one moment the answer is still "edit this line" (a `commit-msg`
hook), plus a CI backstop.

## Problem

1. **The check could only ever run after the fact.** `tools/commitcheck` swept history from the
   baseline commit and reported what was already there. By the time it spoke, the commit object
   existed; once pushed, this repository's git policy forbids rewriting it. The only remaining
   move was a waiver — so the tool's own exception table was the mechanism by which the rule
   eroded.

2. **Violations arrived faster than they could be repaired.** On 2026-09-03 five subjects landed
   from separate agent sessions: three omitted the required scope, two exceeded 72 chars
   (74 and 79). Two of those were written by the session that was auditing the gate.

3. **The failure blocked unrelated work.** 7 open cards carry
   `verify: make lint && make test && make test-integration && make doc-check && make commit-check`
   as their last completion criterion
   (`/usr/bin/grep -rln 'make commit-check' tasks/todo/`). A repository-wide gate failing on
   master makes every one of them unclosable regardless of its own merit.

4. **The Makefile recorded a decision that no longer held.** `commit-check` was deliberately
   unlabelled `(CI)` and deliberately absent from `ci.yml`, on the stated ground that CI clones
   are shallow and the history sweep would see a truncated range. `fetch-depth: 0` answers that
   objection directly; the comment was rewritten rather than deleted, so the reversal is
   readable.

## What was done

- **Waivers, with reasons.** `grandfatheredCommits` went from 2 entries to 7, and gained a
  `reason` field. A waiver without a stated reason is indistinguishable from a rule that was
  never enforced; a test now requires every entry to carry one.
- **`commitcheck -message-file PATH`.** A second mode that checks a message file instead of
  history: comment and blank lines are stripped the way git strips them, `Merge `/`Revert `
  prefixes are exempt, and an unreadable file exits 2 rather than 0 — a check that could not run
  has not approved anything.
- **`.githooks/commit-msg`.** Invokes that mode. Exits 2 when Go is absent instead of passing
  silently. `go run` collapses every nonzero exit to 1 and appends its own `exit status 1` line
  to stderr; the hook drops that line so the author reads the verdict, not an apparent hook
  malfunction.
- **`make install-hooks`.** Sets `core.hooksPath` to `.githooks`. The hook is opt-in per clone
  because git will not run a tracked hook otherwise.
- **CI backstop.** A `commit-subjects` job with `fetch-depth: 0`, and the `(CI)` label on the
  Makefile target so `tools/cilabels` keeps the two in parity. CI is the backstop, not the gate:
  by the time it runs, the subject is already written.

## Completion Criteria

- [x] Every waiver names the commit by exact SHA and subject, and states why it could not be repaired | verify: `go test ./tools/commitcheck -run TestEveryWaiverRecordsWhyItCouldNotBeRepaired -count=1`
- [x] Waivers are pinned to the exact historical objects, so a rewritten history invalidates them rather than silently widening the exemption | verify: `go test ./tools/commitcheck -run TestGrandfatheredCommitsAreTheExactHistoricalObjects -count=1`
- [x] Message-file mode rejects each violation class the history sweep catches, and accepts a conforming subject | verify: `go test ./tools/commitcheck -run TestMessageFileModeRejectsWhatTheHistorySweepWouldHaveCaught -count=1`
- [x] A message file that cannot be read fails loudly instead of approving the commit | verify: `go test ./tools/commitcheck -run TestMessageFileModeFailsLoudlyWhenTheFileIsMissing -count=1`
- [x] The hook exists, is executable, and runs the message-file mode | verify: `test -x .githooks/commit-msg && /usr/bin/grep -q 'message-file' .githooks/commit-msg`
- [x] `make install-hooks` points git at the tracked hook directory | verify: `/usr/bin/grep -q 'core.hooksPath .githooks' Makefile`
- [x] The CI job clones deep enough for the history sweep to be meaningful | verify: `/usr/bin/grep -q 'fetch-depth: 0' .github/workflows/ci.yml`
- [x] `commit-check` appears in both `ci.yml` and the Makefile's `(CI)` labels, and `tools/cilabels` agrees | verify: `make doc-check`
- [x] The sweep passes on the current history with the waivers in place | verify: `make commit-check`
- [x] Repository gates pass | verify: `make lint && make test && make test-integration && make doc-check && make commit-check`

## Evidence

Hook exercised against three message files before installation:

```
docs: repair drift across core documents              → [format] scope is required by the SSOT   exit 1
docs(tasks): xxxxx… (91 chars)                        → [length] subject is 91 chars, limit 72    exit 1
chore(commitcheck): gate commit subjects before…      → (silent)                                  exit 0
```

`make commit-check` reports `7 exact historical exception(s) skipped; all other commits remain
enforced` followed by `OK -- every non-exempt subject since the baseline matches the format
SSOT`.

## Non-goals

- No rewrite of published history. The five subjects were already on master; the waiver table is
  the repair, and the hook is what makes it the last one.
- No change to the format SSOT itself. `maxSubject` stays at 72 with an SSOT target of 50; the
  gap between them is a separate question.
- The hook is not installed automatically. A repository cannot set `core.hooksPath` on a clone it
  does not own; `make install-hooks` is the deliberate step.
