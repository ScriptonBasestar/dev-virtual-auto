---
id: TASK-189
title: "The guided path computes its own copy of the config-presence flag"
type: refactor
priority: P3
effort: XS
created-at: 2026-08-18T15:24:47+09:00
completed-at: 2026-08-18T21:10:00+09:00
source: "4ec336b — 30-configure.yaml has no check_config step to borrow from"
scope: "dva repo — agent-mesh-flows/dva-improve-guided/30-configure.yaml"
status: done
---

# Task 189: One producer for one fact

## Summary

`dva-improve.yaml` gates its backup steps on `check_config.has_dva_yml`, the flag that step
already publishes. `dva-improve-guided/30-configure.yaml` has no `check_config` step, so it
carries its own `backup_paths.has_config` computing the same thing with the same shell.

Two copies of one fact, in two files, each feeding a `when:` gate. The gate contract has
five rules and the producer must satisfy three of them; satisfying them twice is twice the
surface for a copy to drift. The copies are byte-identical today — both were run and
observed — which is the best a duplicate ever is.

The fix is not obvious enough to be worth forcing. The guided stages are separate flows
with their own parameters, so sharing means either adding a `check_config`-equivalent stage
to the guided pipeline or accepting the duplicate and pinning it with a test. Judge which
is smaller once TASK-186 lands, since a flowcheck rule that reads producers may make the
duplicate cheap to keep honest.

## Resolution

**Accepted the duplicate and pinned it.** `config-probe-drift`, a new flowcheck rule, now
fails the build when the copies stop agreeing. No flow file changed — `git status` over
`agent-mesh-flows/` is empty for this task.

Sharing was rejected on measurement, not preference. The card guessed at two copies; there
are **four**:

| flow | key | line |
|---|---|---|
| `dva-diagnose.yaml` | `check_prerequisites.has_dva_yml` | 52 |
| `dva-improve.yaml` | `check_config.has_dva_yml` | 115 |
| `dva-improve-guided/00-analyze.yaml` | `analyze_project.has_dva_yml` | 83 |
| `dva-improve-guided/30-configure.yaml` | `backup_paths.has_config` | 162 |

All four byte-identical. Two of them are in flows that have nothing to do with each other —
`dva-diagnose` and `dva-improve` are separate entry points, and no mechanism exists for one
flow to read another's context. That leaves the two guided stages, and they cannot share
either: each stage is independently runnable, which its own `Fix:` lines instruct the user
to do. A flag passed down as a pipeline parameter is absent exactly when a stage is run on
its own — and absent renders as the literal `{{...}}`, never as `'true'`, so the gate closes
and the backup silently skips. Sharing would convert a maintenance cost into a data-loss
path.

So four copies is the honest shape here, and the card's own alternative — "pinning it with
a test" — is the right half. `tools/flowcheck/corpus.go` collects every shell field across
the corpus, selects the config-presence probes, and requires each to be spelled like the
first in sorted path order.

### Two deviations, both forced by measurement

**The probe signature is `-f 'dva.yml'` plus a printed flag — not both filenames.** The
first draft required a copy to test `dva.yml` *and* `dva.yaml`. Running it against a corpus
where `dva-diagnose`'s copy had been edited down to `[ -f 'dva.yml' ]` alone: **exit 0, no
finding.** The narrowed copy stopped matching the signature, left the probe set, and took
the scan green with it — the only trace being the summary count falling 4 → 3. That is a
detector failing open on the drift that costs the most: that flow now misses every project
using the `dva.yaml` spelling, and nothing says so.

**The flag is matched as a printed word, not as a substring.** Relaxing the filename half
and keeping `strings.Contains(text, "true")` pulled in two unrelated `deterministic_check`
fields at `dva-improve.yaml:808,892` — they open on the same `-f 'dva.yml'` test and carry
`|| true` further down as error suppression. Probe count went 4 → 6 and both reported as
drift. The selector now matches `(printf|echo)[ \t]+["']?(true|false)`, which is the flag
being emitted rather than the word appearing.

Comparing against the *first* copy rather than the majority is deliberate: a majority rule
blesses a drift the moment it reaches three copies. The cost is that editing the reference
copy reports the other three — three findings, one edit, each naming the location to
compare against.

## Completion Criteria

- [x] The guided path's config-presence flag has one definition, or the duplicate is pinned by a check that fails on drift | verify: human — read `30-configure.yaml` and the pinning test if one was added
- [x] The gates still hold on both tracks | verify: human — run the stage against a fixture with and without a config; the backup steps run and skip respectively
- [x] Flow still validates | verify: `am validate agent-mesh-flows/dva-improve-guided/30-configure.yaml`
- [x] Corpus stays clean | verify: `go run ./tools/flowcheck`

## Evidence

### The rule fires on real drift

Each row is one edit to a shipped flow, `go run ./tools/flowcheck .`, then revert:

| drift injected into a shipped copy | exit | reported |
|---|---|---|
| `30-configure` swaps `printf` for `echo` | 1 | `30-configure.yaml:162` (also caught by `gate-producer-newline`) |
| `dva-diagnose` drops the `dva.yaml` alternative | 1 | the other three copies, each against `dva-diagnose.yaml:52` |
| `00-analyze` inverts `\|\|` to `&&` | 1 | `00-analyze.yaml:83` |

Row 1 overlaps an existing rule — but only because that copy feeds a gate. Rows 2 and 3 are
invisible to every other rule in the tool: the shell is well-formed, the gate contract is
satisfied, and the flow just answers a different question than its siblings.

### The parts are load-bearing

Eight mutations of `corpus.go`, each run against `TestConfigProbeDrift` and reverted. None
survived:

| mutation | caught by |
|---|---|
| rule stubbed to return nil | `one_copy_drifts_to_echo`, `two_copies_drift`, `a_copy_stops_testing_the_other_filename` |
| flag match drops to a substring | `shell_that_merely_mentions_true_is_not_a_probe` |
| flag half of the signature dropped | `shell_that_merely_mentions_true…`, `the_fields_naming_a_file…` |
| filename half of the signature dropped | `the_sibling_boolean_probes_are_a_different_fact` |
| signature re-requires the `dva.yaml` alternative | `a_copy_stops_testing_the_other_filename` |
| normalize stops dropping comments | `a_comment_on_one_copy_is_not_drift` |
| normalize stops trimming space | `the_same_probe_as_a_plain_scalar…`, `a_comment_on_one_copy…` |
| empty-set guard removed | `no_copies_at_all` (panics) |

The negative cases are copied out of the corpus rather than invented, because the risk in a
cross-file rule is a false positive against a neighbour: `config_path` and the two `CONFIG=`
selectors run the same `-f` test but name a file, `has_compose` and `has_dockerfile` sit in
the same context map printing the same two words about a different question, and
`deterministic_check_1` carries a `|| true`. All must stay out of the set; a rule that
reports three different facts as one gets switched off.

### The gates still hold, and nothing shifted under them

No flow changed, so the gate behaviour is the same bytes TASK-184 verified. Re-measured
against fixtures anyway:

```
with     -> true (bytes: 4)      # dva.yml present
without  -> false (bytes: 5)     # empty directory
gate compare: with == 'true' PASS
```

Four bytes, not five — the `printf` emits no trailing newline, which is what keeps
`{{...}} == 'true'` comparable at all.

```
✅ agent-mesh-flows/dva-improve-guided/30-configure.yaml is valid
flowcheck: 10 flow file(s), 103 shell field(s), 14 when-gate(s), 40 dva invocation(s),
           3 report-reading field(s), 3 skippable reference(s),
           4 config-presence probe(s), 23 built-in command(s)
flowcheck: OK — no decision-path defects
```

The probe count is printed in the summary for the same reason every other count is: a rule
that matches nothing reads exactly like a rule that passed. If a fifth flow starts asking
this question, the count says so.

## Technical Notes

- Both copies read `cd '{{param.target}}' && { [ -f 'dva.yml' ] || [ -f 'dva.yaml' ]; } && printf true || printf false`.
- The quoting is load-bearing, not style — see TASK-186.
- Lowest priority of the set. It is a maintainability cost, not a defect; nothing
  misbehaves today.
- `config-probe-drift` is the first corpus-wide rule in flowcheck. It lives in its own file
  because the unit differs: every other rule reads one field, and each copy of this fact is
  correct on its own — only the set can be wrong.
