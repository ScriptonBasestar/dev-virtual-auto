---
id: TASK-208
title: "Five comments size the flag-fallthrough class at 12 call sites; the real count is 6"
type: docs
priority: P3
effort: S
created-at: 2026-08-20T14:08:00+09:00
source: "found by the TASK-198 corpus sweep, which was sized from these comments and expected a backlog of 5 unguarded call sites that does not exist"
scope: "Comments only, five sites: internal/cli/compose.go:846-847, internal/cli/flagtoken.go:131-132, internal/cli/build_flag_leak_test.go:5, internal/cli/flagtoken_test.go:214. No behaviour change."
status: todo
---

# Task 208: Five comments size the flag-fallthrough class at 12 call sites; the real count is 6

## Summary

Five comments in `internal/cli/` state that `parseDvaFlags` has **12** call
sites, and three of them split that total into "7 that have their own
unknown-flag rejection to name" and "the other 5" that do not. The real
denominator is **6**, and it has been 6 since 2026-08-06.

Measured at `5649d70` (2026-08-26 worktree baseline):

```
grep -rn 'parseDvaFlags(' internal/cli/*.go \
  | grep -v '_test.go' | grep -v 'func parseDvaFlags' | grep -v '//'
```

→ 6 (`compose.go:161,286,397,466,546,719`)

The figure was **correct when written** and went stale by refactor, not by
error:

```
f4c83d7  2026-08-03  fix(cli): reject a malformed bool in parseDvaFlags   -> 12 call sites
6710766  2026-08-06  refactor(cli): move to intent-centric plan model     ->  6 call sites
```

`6710766` deleted the `stack`/`app` command families along with
`applications:`, which halved the class. The comments were not updated with it.

The five sites:

```
internal/cli/compose.go:846-847     "7 of the 12 call sites" / "all 12 callers"
internal/cli/flagtoken.go:131-132   "That held for 7 of its 12 call sites. The other 5 have no…"
internal/cli/build_flag_leak_test.go:5   "which 7 of its 12 call sites have"
internal/cli/flagtoken_test.go:214  "which 5 of the 12 call sites do not have"
```

## Why this is worth a card

A stale denominator in a comment is not inert — it is the number the next person
sizes the work from. TASK-198's sweep was commissioned expecting a backlog of
**5 unguarded call sites** on the strength of `flagtoken.go:87`. The measured
answer was 1 (`restart`, now fixed), and 0 remain. Someone reading these
comments without re-measuring would open four cards that have no subject.

This is the same failure mode the repo has already recorded twice — a count that
must come from the tool that owns the definition, re-derived at the moment of
use rather than carried forward.

## Do not just divide by two

The total is a measurement; the **7/5 split is a classification**, and the
refactor did not necessarily halve it proportionally. On the current 6 sites the
"has its own rejection to name" property distributes differently: `up`,
`teardownCommon` and (as of TASK-198) `restart` name their own rejection, while
the second `down` and `stop` parses inherit theirs from `teardownCommon` five
lines above and `build` is a deliberate passthrough into docker's argv. Whoever
fixes this must re-derive the split, state the extraction command beside the
number, and not arithmetic 7 and 5 down to 3 and 3 by assumption.

Prefer a form that cannot go stale silently: name the extraction command in the
comment, or drop the count and describe the property instead.

## Completion Criteria

- [ ] No comment in `internal/` states a 12-call-site figure for `parseDvaFlags` | verify: `grep -rc '12 call sites\|All 12 callers\|of the 12 call' internal/ | grep -v ':0' | wc -l` returns 0 (today: 4, the five sites live in four files)
- [ ] Any surviving count names the command that produces it, so the next reader can re-measure | verify: human — read the five sites and confirm each number is either removed or accompanied by its extraction
- [ ] The split is re-derived rather than halved | verify: human — the disposition states the new classification and the command used, not just the new totals
- [ ] No behaviour change: the two test files still pass unchanged in intent | verify: `go test ./internal/cli/ -count=1`
- [ ] `make test` passes | verify: `make test`
- [ ] `make doc-check` passes | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && make doc-check`

## References

- `internal/cli/flagtoken.go:131-132` — the site TASK-198's sweep was sized from
- `internal/cli/compose.go:846-847`
- `internal/cli/build_flag_leak_test.go:5`, `internal/cli/flagtoken_test.go:214`
- `f4c83d7` — the commit that authored the figure, where 12 was correct
- `6710766` — the refactor that halved it without touching the comments

## Technical Notes

The current sites above are at `5649d70`; they moved when the restart guard and
later explanatory comments landed. The historical `684/691` and `87` locations
are retained in the creation record only by the measured claim that those were
the sites when TASK-198's sweep was written. Re-run the extraction rather than
trusting line numbers after integration — which is the same discipline this card
is asking for.
