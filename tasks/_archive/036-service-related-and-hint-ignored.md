---
id: TASK-036
title: "services.<svc>.related and .hint validate green, are never read, and have a test that proves only that they parse"
type: bug
priority: P3
status: done
effort: S
created-at: 2026-07-17T03:30:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: fresh Phase 1 sweep (silent config no-ops)
source-severity: LOW
moved-at: 2026-07-17T11:49:00+09:00
verified-at: 2026-07-17T11:49:00+09:00
decision: remove related and hint from schema and struct
decision-rationale: |
  Zero non-test readers of Related/Hint. Only coverage was parse-roundtrip
  (false confidence). Surfacing needs UX design with no in-repo precedent.
  Lean remove: drop schema properties, struct fields, and parse-only test;
  replace with schema rejection test + tags control.
verification-summary: |
  Decision: REMOVE related/hint from service_tag_config and ServiceTagConfig.
  Implementation: schema.json, config.go, TestValidateRejectsServiceRelatedAndHint,
  TestServiceTagsFieldParsing (tags control). Docs/templates cleaned.
  go test ./internal/config/ ./internal/lifecycle/ → ok.
---

# Task 036: Two Keys, Zero Reads, And A Test That Manufactures Confidence

## Decision (recorded)

**REMOVE** `services.<svc>.related` and `.hint` from schema and Go struct.

| Option | Chosen | Why |
|--------|--------|-----|
| Remove | **yes** | Never consumed; parse-only test was false confidence |
| Surface | no | No UX precedent for where to show them |

Parse-only `TestServiceRelatedFieldParsing` replaced by tags control + schema rejection test.


## Summary

`stack.<entry>.runners.compose.services.<svc>.related` and `.hint` (`schema.json:52-53`,
struct at `config.go:226-227`) are accepted, validate green, and are never read by any command.
Their only "coverage" is a test asserting they **parse** — which is true of any field that exists.

## Evidence

```
$ grep -rn "\.Related\b" --include="*.go" internal/ | grep -v _test.go   ->  zero
$ grep -rn "\.Hint\b"    --include="*.go" internal/ | grep -v _test.go   ->  zero
```

Control — the same grep shape finds the **sibling** field `Tags` genuinely consumed at
`internal/lifecycle/orchestrator.go:366` (`hasAnyTag(e.Tags, tagSet)`), so the pattern works and
the zeros are real absences, not a broken search.

The only references to either field are the struct definition and `config_test.go:316`, which
asserts the values survive a round-trip through the parser. That test passes today and would
continue to pass if every consumer were deleted — it cannot fail for the reason anyone cares
about. It is false confidence: a green check that certifies nothing about behavior.

A sweep of 11 commands with sentinel values in `related`/`hint` produced **0** occurrences of the
sentinel across 105 lines of output. Control: `grep -c grafana` against the same output → 1, so
the output did contain service-related content and the sweep was looking in a live place.

## Assessment — inert, unlike its siblings

Recorded honestly: this is the mildest of the three silent-config findings and the only one that
is purely inert.

- Nothing is mutated (unlike TASK-032/033).
- No artifact is corrupted (unlike TASK-034's leak).
- No behavior is inverted (unlike TASK-035's `interpolate: false`).

The cost is documentation surface promising a feature that does not exist. `hint` in particular
reads as user-facing help text — a user writing one reasonably expects it surfaced by `dva status`
or `dva ls`. It never appears anywhere.

P3 on its own evidence. Filed because it is a live instance of the run's organizing theme, and
because the parse-only test is a small worked example of *how* this class survives review.

## Scope note — needs a decision

Either **surface them** (where? `status`? `ls`? `doctor`? — that is a UX design question with no
in-repo precedent to settle it, unlike TASK-032/033 where the `Use:` strings decided the
direction) or **remove them from the schema**. Removal is likely right given nothing has ever
consumed them, but that is not the implementer's call.

Whichever is chosen, `config_test.go:316` should stop being cited as coverage: if the fields are
surfaced, the test must assert the surfacing; if removed, the test goes with them.

## Completion Criteria

- [ ] DECISION — surface `related`/`hint`, or remove them from `schema.json`? | verify: `human — decide; if surfacing, name the exact command and output location`
- [ ] No config key remains that validates green and does nothing | verify: `human — either a probe shows the key affecting real output, or the key is gone from schema.json`
- [ ] If surfaced: a test asserts the value reaches user-visible output, not merely that it parses | verify: `human — delete the surfacing code, confirm the new test FAILS; config_test.go:316 must NOT be what covers this`
- [ ] If removed: `config_test.go:316`'s parse-only assertions go too, rather than being left to certify a deleted field | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && go test ./internal/config/`
- [ ] The sibling `Tags` field still works (the control must not regress) | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && go test ./internal/lifecycle/`
- [ ] `make test` and `go vet ./...` pass | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && make test && go vet ./...`

## References

- [034-devcontainer-config-path-ignored-and-leaks.md](./034-devcontainer-config-path-ignored-and-leaks.md) — same class, but actively corrupts an artifact
- [035-env-file-interpolate-and-priority-ignored.md](./035-env-file-interpolate-and-priority-ignored.md) — same class, but inverts requested behavior
- [031-integration-suite-red-and-excluded-from-ci.md](./031-integration-suite-red-and-excluded-from-ci.md) — the other "verification that reports something other than the truth" instance this run
