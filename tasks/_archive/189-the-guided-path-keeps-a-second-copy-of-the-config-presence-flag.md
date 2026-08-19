---
id: TASK-189
title: "The guided path computes its own copy of the config-presence flag"
type: refactor
priority: P3
effort: XS
created-at: 2026-08-18T15:24:47+09:00
completed-at: 2026-08-18T21:10:00+09:00
quality-review: pass
quality-reviewed-at: 2026-08-19T14:14:48+09:00
verified-at: 2026-08-19T14:22:49+09:00
archived-at: 2026-08-19T14:22:49+09:00
quality-review-evidence: |
  - kind: automated
    command-or-step: "am validate agent-mesh-flows/dva-improve-guided/30-configure.yaml (AC3) and go run ./tools/flowcheck (AC4)"
    result: exit 0 both — corpus OK, `4 config-presence probe(s)` printed beside the verdict
  - kind: automated
    command-or-step: "AC1 — the four copies read out of the parsed YAML and hashed"
    result: pass — one distinct spelling across all four (sha256 3d9bcb5df0eb…), and the pinning rule is `config-probe-drift` in tools/flowcheck/corpus.go, not a comment. git log -S confirms no flow file changed for this card
  - kind: automated
    command-or-step: "AC2 — probe extracted from 30-configure and run against a fixture with and against one without"
    result: pass — `with -> true (bytes: 4)`, `without -> false (bytes: 5)`, reproducing the card's byte counts; no trailing newline, which is what keeps `== 'true'` comparable. Three gates consume it (30-configure.yaml:184, :193, :215), so backup_marker/backup_config/prune_backups run together and skip together
  - kind: automated
    command-or-step: "'The rule fires on real drift' — all three rows re-injected into the shipped flows, flowcheck run, files restored"
    result: all three reproduce. echo-for-printf in 30-configure -> :162 reported by both config-probe-drift and gate-producer-newline, exit 1; dva-diagnose narrowed to one filename -> the other three copies reported (00-analyze:83, 30-configure:162, dva-improve:115), exit 1; `||` inverted to `&&` in 00-analyze -> :83 alone, exit 1. All three files restored byte-identical, git status clean
  - kind: automated
    command-or-step: "mutation table, two rows sampled"
    result: both match. checkConfigProbe stubbed to nil kills exactly one_copy_drifts_to_echo, two_copies_drift, a_copy_stops_testing_the_other_filename; reBoolEmit relaxed to `(true|false)` kills shell_that_merely_mentions_true_is_not_a_probe and pulls the probe count 4 -> 6 with 2 findings, the deterministic_check pair the card names. corpus.go restored byte-identical
  - kind: manual
    command-or-step: "correction applied to the Resolution table"
    result: the 00-analyze copy is published by `detect_basics`, not `analyze_project` — that id appears nowhere in the corpus and `git log -S` finds no commit that ever added it. File and line (83) were right. Cell corrected in place; the substance of the row is unaffected
source: "4ec336b — 30-configure.yaml has no check_config step to borrow from"
scope: "dva repo — agent-mesh-flows/dva-improve-guided/30-configure.yaml"
status: done
verification-summary: |
  quality-review pass, re-observed at disposition. Both AC bindings exit 0 and the live summary
  still prints `4 config-presence probe(s)`, so the rule's scope is non-empty. The four copies
  re-read out of the flows and hashed: one distinct spelling, sha256 3d9bcb5df0eb6de7…, at
  dva-diagnose.yaml:52, dva-improve.yaml:115, 00-analyze.yaml:83, 30-configure.yaml:162 —
  exactly the file/line pairs the Resolution table records. The in-review correction holds: the
  00-analyze copy is published by `detect_basics`, and `analyze_project` now appears on the card
  only inside the note that records the correction.
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
| `dva-improve-guided/00-analyze.yaml` | `detect_basics.has_dva_yml` | 83 |
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
