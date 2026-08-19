---
id: TASK-191
title: "A comment explaining the code is what stops the step from running"
type: bug
priority: P1
effort: S
created-at: 2026-08-18T17:46:43+09:00
completed-at: 2026-08-18T17:46:43+09:00
quality-review: pass
quality-reviewed-at: 2026-08-19T14:04:00+09:00
verified-at: 2026-08-19T14:20:08+09:00
archived-at: 2026-08-19T14:20:08+09:00
quality-review-evidence: |
  - kind: automated
    command-or-step: "go run ./tools/flowcheck (AC1)"
    result: exit 0 over a non-zero denominator — 10 flow files, 103 shell fields scanned, 0 findings
  - kind: automated
    command-or-step: "go test ./tools/flowcheck/ -run TestCommentSubstitution (AC2)"
    result: exit 0 — 9 named subtests including the negative cases (span outside a comment, hash inside quotes, empty comment)
  - kind: automated
    command-or-step: "independent mutation check — stub commentSubstitutions to return nil, re-run the suite, then restore"
    result: 6 of 9 subtests FAIL when stubbed; restored file is byte-identical (git diff empty) and the suite is green again. The card claimed 5 of 8; the suite has since grown one case, so the discrimination claim holds and is stronger than recorded
  - kind: automated
    command-or-step: "am validate agent-mesh-flows/dva-improve.yaml && am validate agent-mesh-flows/dva-improve-guided/10-verify.yaml (AC4)"
    result: exit 0 on both
  - kind: manual
    command-or-step: "AC3 — three fields still blocked after the fix"
    result: correctly attributed, not hidden. They were blocked at 4cfd365 too, on a different span, and are filed as TASK-192 rather than closed silently
source: "4cfd365 — found while probing am for TASK-184"
scope: "dva repo — agent-mesh-flows/ comments, tools/flowcheck"
status: done
verification-summary: |
  quality-review pass, re-observed at disposition. All four AC bindings exit 0; TestComment-
  Substitution still reports 9 passing subtests, and the rule is emitted from
  tools/flowcheck/shell.go:36, not from a comment. The three fields this card left blocked
  were routed to TASK-192 rather than closed silently, and TASK-192 is itself reviewed pass and
  dispositioned in this same run — the follow-up chain is closed, not dangling.
---

# Task 191: A comment explaining the code is what stops the step from running

## Summary

`/bin/sh` ignores everything after `#`. am does not. Its shell policy analyzer drops a
comment's plain words but still pulls backtick and `$(...)` spans out of it and runs them
past the allowlist, so the sentence explaining a defect blocks the step that fixes it.

Four shipped steps — six fields — were blocked this way, all of them on the decision path:

| field | file | blocked on |
|---|---|---|
| `scan_project.context.compose_services` | `dva-improve-guided/00-analyze.yaml` | `` `backend` `` |
| `scan_project.context.package_names` | `dva-improve-guided/00-analyze.yaml` | `` `subprojects` `` |
| `check_dva_needed.action` | `dva-improve-guided/10-verify.yaml` | `` `[1][2]{...}` `` |
| `detect_track.context.track` | `dva-improve-guided/30-configure.yaml` | `` `[1][2]{...}` `` |
| `scan_compose.context.root_compose` | `dva-improve.yaml` | `` `backend` `` |
| `scan_compose.context.infra_compose` | `dva-improve.yaml` | `` `backend` `` |

The corpus held 23 such spans across 8 fields in 4 files; all 23 are gone. Three of the six
fields run now. The other three turned out to be blocked twice over and hit the second
defect underneath, filed as TASK-192.

## Why it hid

Two reasons, both measured against am cb8b4ce.

**A blocked step does not stop an interactive run.** am prompts `Continue anyway?
[Y/n/f(force)]`, defaults to continue, every reader of the blocked step's keys receives
the literal text `{{step.key}}`, and the pipeline ends `Done`. Only `-y` batch mode exits
non-zero. So the guided pipeline reported success while its verify gate never ran.

**Whether a span blocks at all is a coin flip decided elsewhere in the field.** am's quote
tracking crosses lines and `#` does not end a quote, so an apostrophe anywhere in the field
opens a string that swallows every span until the next one. That is why `save_report` and
`backup_paths.src` held six spans between them — four of which block on their own, measured
in isolation — and still ran: an apostrophe earlier in each field had hidden them.
`backup_paths.src`'s was `am's`, in the comment explaining the quoting rule two lines above.
Deleting that word would have armed the block.

This is why the rule reports every span rather than the ones that block today. The
apostrophe parity of a comment is not a property anyone can maintain.

## Resolution

- Removed the backticks from all 23 spans. The prose is unchanged otherwise; the corpus
  already used `--` dashes for the same emphasis.
- Added the `comment-substitution` rule to `tools/flowcheck`. It reads the raw field
  *before* `blankComments`, which every other rule depends on, since the whole point is
  that am does not blank what /bin/sh does.
- Split `TestBareWordArg` into `shell_test.go` alongside the new
  `TestCommentSubstitution`, mirroring the `rules.go` / `shell.go` / `gate.go` split.

Writing the negative cases turned up a defect in the rule itself: `skipQuoted(text, i, q)`
takes the index *after* the opening quote — its three existing call sites all pass `i+1` —
and the new one passed `i`, so the function returned immediately and quoted regions were
never skipped. The corpus never showed it because the bug can only produce false positives
and no corpus comment sits inside quotes. A negative case that only passes when the rule is
exactly right is worth more than one that passes when the rule does nothing.

## Evidence

Each affected field was extracted verbatim from the YAML and run as a one-step flow, at
`HEAD` and again after the fix. `am run <probe> < /dev/null`, no `-y`, so a block is
reported rather than aborting the run.

| field | at 4cfd365 | after |
|---|---|---|
| `scan_project.context.compose_services` | blocked `backend` | blocked `yaml_block_keys` → TASK-192 |
| `scan_project.context.package_names` | blocked `subprojects` | ran |
| `save_report.action` | ran (spans hidden by `am's`) | ran |
| `check_dva_needed.action` | blocked `[1][2]{...}` | ran |
| `detect_track.context.track` | blocked `[1][2]{...}` | ran |
| `backup_paths.context.src` | ran (spans hidden) | ran |
| `scan_compose.context.root_compose` | blocked `backend` | blocked `yaml_block_keys` → TASK-192 |
| `scan_compose.context.infra_compose` | blocked `backend` | blocked `yaml_block_keys` → TASK-192 |

The three still-blocked fields are blocked by a second, unrelated defect the fix uncovered
rather than caused: they were blocked at `HEAD` too, on the span that came first. Filed as
TASK-192.

Rule discrimination: with `commentSubstitutions` stubbed to return nil, five of the eight
`TestCommentSubstitution` cases fail. With it restored, `go test ./tools/flowcheck/` is
green and `go run ./tools/flowcheck` reports 0 findings over 101 shell fields.

## Completion Criteria

- [x] No backtick or `$(...)` span survives in a flow shell comment | verify: `go run ./tools/flowcheck`
- [x] The rule fires on a comment span and stays silent without one | verify: `go test ./tools/flowcheck/ -run TestCommentSubstitution`
- [x] No field is blocked by a comment span any more | verify: human — see the Evidence table; each of the eight fields was extracted and run through `am` at 4cfd365 and after
- [x] Flows still validate | verify: `am validate agent-mesh-flows/dva-improve.yaml && am validate agent-mesh-flows/dva-improve-guided/10-verify.yaml`

## Technical Notes

- am's model, as measured: comment words are dropped for the command scan, but backtick and
  `$(...)` spans are extracted from the raw text, where quotes count and `#` does not.
- `printf yes` in a comment blocks on `yes`, not `printf` — the analyzer reads the span's
  arguments too, so the `bare-word-arg` rules apply inside a comment span as well.
- Measured allowlisted, so harmless in a span on their own: `jq`, `true`, `false`. Measured
  blocking: `backend`, `subprojects`, `smart`, `json_mode:`, `dva-improve`, `depends_on`,
  `.dva_needed`, `[1][2]{...}`, `dva.yml`, `yes`. Sorting comment prose by that list is the
  thing the rule refuses to do.
- The allowlist can be widened in `~/.config/agent-mesh/sandbox_override.yaml`, which is
  user-local config. A shipped flow cannot depend on it.
