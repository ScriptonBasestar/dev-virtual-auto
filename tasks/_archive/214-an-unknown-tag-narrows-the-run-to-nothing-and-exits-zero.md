---
id: TASK-214
title: "An unknown tag narrows the run to nothing and exits zero"
type: bug
priority: P2
effort: M
created-at: 2026-08-20T19:20:00+09:00
source: "carved out of TASK-213, which refused the degenerate tag values (empty, blank, `,`) and measured that the non-degenerate ones fail the same way for a different reason"
scope: "`--tag`/`--tags`/`-T` and `--exclude-tag`/`--exclude-tags` against `filterByTags` in internal/lifecycle/orchestrator.go. Mode and env are out of scope — they already validate against the config and are the model this card asks tags to follow."
status: done
completed-at: 2026-08-26T11:25:51+09:00
completion-summary: "Reject unknown include and exclude tags before lifecycle side effects while preserving declared tag narrowing."
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "dva test"
    result: "passed; internal/cli 75.3% coverage, internal/lifecycle 64.1% coverage"
  - kind: automated
    command-or-step: "dva lint"
    result: "passed; 292 Go files formatted, 0 issues"
  - kind: automated
    command-or-step: "make doc-check"
    result: "passed; 279 Markdown files, 552 links, 1,146 test functions"
quality-review: pass
quality-reviewed-at: 2026-08-26T11:27:27+09:00
quality-review-evidence:
  - "targeted CLI and lifecycle tests passed on the reviewed diff"
  - "full dva test, dva lint, and make doc-check gates passed"
quality-review-receipt: tmp/task-management/direct/queue-run/task-214-review-receipt.json
archived-at: 2026-08-26T11:28:09+09:00
verified-at: 2026-08-26T11:28:09+09:00
verification-summary: "Unknown include and exclude tags are rejected before side effects; declared selector controls and all repository gates passed."
---

# Task 214: An unknown tag narrows the run to nothing and exits zero

## Summary

A tag that matches nothing is not a mistake as far as DVA is concerned. Measured
on a two-entry fixture where `s1` declares `tags: [db]` and `s2` declares
`tags: [api]` — written for this measurement before finding that
`writeRestartTaggedPlanProbeConfig` in `internal/cli/restart_names_test.go`
already provides the same shape with `web`/`db`, which is the one to reuse:

| invocation | error | what ran |
|---|---|---|
| `dva restart --tag=db` | none | `s1_up s1_stop` — the control, and it fires |
| `dva restart --tag=dbb` | **none, rc=0** | **nothing** |
| `dva restart --tag=" db"` | **none, rc=0** | **nothing** |
| `dva restart --exclude-tag=dbb` | **none, rc=0** | the whole stack |
| `dva restart --mode=nosuchmode` | `mode 'nosuchmode' not found…` | nothing |

The last row is the point. One typed character separates rows 2 and 1, and the
result of getting it wrong is a command that reports success having done nothing.
The same typo in `--mode` is refused by name, because something owns mode names
and checks them against the config. Nothing owns tag names: `filterByTags`
(`internal/lifecycle/orchestrator.go`) builds a set from what was asked for and
keeps entries via `hasAnyTag`, so an unknown tag is indistinguishable from a tag
no entry happens to carry.

This is TASK-211's harm — a narrowing flag producing a result the user cannot
tell from success — arrived at through a value that is well-formed. TASK-211 and
TASK-213 closed the ill-formed values (`--tag`, `--tag=`, `--tag=,`,
`--tag=" "`); this card is the well-formed-but-undeclared class.

It is **not** the last one. This sentence read "the class this card names is what
is left after they are gone" until a review found a third: `--exclude-tag
--tag=x` stores the *next flag* as the value, which is neither ill-formed nor
undeclared, and runs the whole stack at rc=0. That is TASK-215. A card is a poor
place to declare a family closed — the two cards that tried both had to be
corrected by the next reviewer who probed instead of reasoned.

`--tag=<U+200B>` (zero-width space) belongs here rather than to TASK-213:
`strings.TrimSpace` rejects only `unicode.IsSpace`, and a zero-width space is
not one, so it arrives as a well-formed tag that nothing declares.

## Why TASK-213 did not close it

TASK-213 refuses values that carry no information *as values*: empty, blank, and
empty-after-splitting. It deliberately does not trim what survives, because
trimming would silently rewrite the user's input — `--tag=" db"` would become
`--tag=db` and the run would look correct rather than be questioned. The right
answer is not to normalise the value but to check it against the config, which
is a different piece of code in a different package and needs its own card.

## What to change

`filterByTags` (or its caller in `internal/cli`) should report tags that match no
declared tag in the loaded config. Two constraints worth settling before writing
code:

- **Where.** `filterByTags` has the config and is the natural place, but it
  currently returns a filtered slice and no error. The alternative is validating
  in `parseDvaFlags`' callers, which have `c *config.Config` in hand — at the
  cost of doing it once per command instead of once.
- **How strict.** "No entry carries this tag" is unambiguous for the include
  side. For `--exclude-tag` the same condition is arguably harmless (excluding
  nothing is what the user asked for if nothing matches) — but it is exactly the
  spelling that ran the whole stack in TASK-213's review, so the card's default
  is to refuse both and let the implementer argue otherwise from a measurement.

Whatever is chosen, the empty-run case needs a distinguishable outcome. Compare
`--mode`: a mode filter that leaves nothing already fails with `no lifecycle
entries matched filters`, which tags never reach because filtering to zero is
only an error on some paths.

## Completion Criteria

- [x] An unknown tag is refused by name rather than silently narrowing | verify: `/usr/bin/grep -rn 'no entry declares' internal/lifecycle/*.go internal/cli/*.go | /usr/bin/grep -v _test` returns at least one line — **today 0, measured**
- [x] A test asserts nothing ran *and* an error was returned for an unknown tag, against a fixture that declares real tags | verify: `/usr/bin/grep -c 'unknown tag: nothing should have run' internal/cli/*_test.go` ≥ 1 — **today 0, measured.** The first draft of this criterion bound on `grep -c 'tags: \[db\]'`, which already returned 1 and so could not fail: `writeRestartTaggedPlanProbeConfig` (`restart_names_test.go`) has declared `web`/`db` tags since TASK-033. **Reuse it — do not write a third fixture**, and note that the default `writeRestartProbeConfig` declares no tags at all, so a test written against *that* one passes for the wrong reason
- [x] The `--exclude-tag` side is settled explicitly, not by default | verify: human — record in this card whether an unknown excluded tag is an error, with the measurement that decided it
- [x] The control still passes: a tag that does match still narrows the run to the matching entries | verify: a test runs `--tag=db` against `writeRestartTaggedPlanProbeConfig` and asserts **`s2` ran and `s1` did not** — measured: `--tag=db` → `[s2_stop s2_up]`, `--tag=web` → `[s1_stop s1_up]`. **Without this row every criterion above is satisfied by a build that refuses every tag.** This said "`s1` ran and `s2` did not" until a review measured it: that mapping is the ad-hoc fixture in the summary table above, where `s1` carries `db`, and the criterion carried it over to the mandated fixture, which is the other way round. An implementer following it literally writes a test that fails on a correct build
- [x] `make test`, `make lint`, `make doc-check` pass | verify: run them and record the denominators, not just OK

## Resolution

Unknown excluded tags are errors, matching unknown included tags. Before this change,
`--exclude-tag=typo` against the tagged restart fixture exited 0 and ran the whole
stack. After the change it returns `no entry declares tag "typo"` and leaves zero
markers. A declared comma list remains valid: `--exclude-tag=web,db` exits 0 and
leaves zero markers because it explicitly excludes both declared entries.

## References

- `internal/lifecycle/orchestrator.go` — `filterByTags` and `hasAnyTag`, where an unknown tag becomes an empty result
- `internal/cli/compose.go` — `takeValue` / `takeList`, which refuse the ill-formed values and deliberately do not trim the rest; `resolveMode`, the validation model this card asks for
- `tasks/_archive/213-an-empty-inline-flag-value-is-accepted-and-widens-the-run-the-way-a-missing-one-used-to.md` — the card whose review measured this, under "What this card does not close"
- `tasks/_archive/211-a-stack-flag-missing-its-value-is-dropped-and-the-command-runs-as-if-unwritten.md` — the original shape: a narrowing flag whose failure is indistinguishable from success

## Technical Notes

The two sides fail in opposite directions from the same cause, which is worth
keeping in mind when picking the message. For `--tag`, matching nothing means
running nothing; for `--exclude-tag`, matching nothing means excluding nothing
and running everything. A single "tag matched no entries" warning would describe
both, but the consequence it should warn about is the opposite in each case.
