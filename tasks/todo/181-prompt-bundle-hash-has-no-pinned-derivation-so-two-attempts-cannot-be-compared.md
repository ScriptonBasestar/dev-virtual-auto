---
id: TASK-181
title: "`prompt_bundle_hash` has no pinned derivation, so a stage reads drift where there is none"
type: bug
priority: P3
status: todo
effort: S
created-at: 2026-08-05T17:46:00+09:00
source: "dogfood run 20260805-143543-f82daf stage 10 — PROMPT-002"
scope: "dva repo — workflows/dva-dogfood/ref-artifacts.md, 00-start.md, 10-baseline.md"
---

# Task 181: Pin how `prompt_bundle_hash` is computed, the way the dirty hashes already are

## Problem

`ref-artifacts.md` states the rule for one hash and not the other:

```yaml
# ref-artifacts.md:63
  dva_dirty_hash: null   # same derivation, or the values cannot be compared
# ref-artifacts.md:69
  prompt_bundle_hash: null
```

So two stages of the same run computed it two ways and disagreed. In run
`20260805-143543-f82daf`, stage 00 recorded `375aeb00…` and stage 10 computed `30a736c8…` — while
git reported **0 files changed and 0 dirty** across `workflows/dva-dogfood/` between them. The
prompts had not moved at all.

The cause is what the two derivations covered. A hash over every file in that directory picks up
`workflows/dva-dogfood/.ce/heartbeat/sessions/*.json` and `.ce-metrics/*.json`, which are
untracked and rewritten on every session — so that derivation is **not reproducible from git by
construction**, and its value changes when nothing about the prompts does.

The cost is not the wrong digits. It is that a stage comparing its value against the previous
one's reads a mismatch as prompt drift, investigates, and finds nothing — or worse, records
"prompts changed mid-run" in a report where it is false. The field exists to answer exactly one
question (were the instructions the same?) and currently cannot.

## Acceptance criteria

- [ ] `ref-artifacts.md` states the derivation for `prompt_bundle_hash` next to the field, in the
      same form as the `dva_dirty_hash` comment. It must be reproducible from a clean checkout —
      tracked files only, sorted, with the command written out.
- [ ] The derivation excludes untracked and generated content under `workflows/dva-dogfood/`.
      State it as a property (tracked files only), not as a list of directories to skip, so a new
      telemetry directory does not silently reopen the hole.
- [ ] Every numbered stage that records or compares the field points at that one derivation
      rather than restating it. Restated instructions are how the two versions arose.
      Verify: `human — grep the stage prompts and confirm one definition, N references`
- [ ] A stage that finds a mismatch is told what it means: prompts changed mid-run is a run
      condition to record, not a gate failure, and the stage must say which files differ rather
      than only that the hash did.
- [ ] The same check is applied to `skill_source_hash` / `installed_skill_hash`, or the reason
      they do not need it is recorded — they already use a path-independent content digest, which
      is a third derivation in the same file.
      Verify: `human — the decision and its reasoning are in the Result section`

## References

- `workflows/dva-dogfood/ref-artifacts.md:63` — the rule, stated for the dirty hashes
- `workflows/dva-dogfood/ref-artifacts.md:69` — the field that lacks it
- Run `20260805-143543-f82daf` `state.yaml` — the derivation is pinned inline there as a
  stopgap, with the two conflicting values recorded

## Notes

Found in stage 10 of the dogfood run, by treating a hash mismatch as a question rather than an
answer. Git said the prompt files were unchanged; a hash said otherwise; only one of those can be
right, and the one computed over untracked telemetry is not.

This is the same class the `dva_dirty_hash` comment already guards against — a value that looks
comparable and is not. `da39a3ee…`, the sha1 of empty input, was the earlier instance: identical
for every clean tree anywhere, so it can never detect a change. Both fields fail the same way, and
one of them has the warning.
