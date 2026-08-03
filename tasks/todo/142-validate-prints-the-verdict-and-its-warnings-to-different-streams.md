---
id: TASK-142
title: "dva validate prints ✅ on stdout while the warnings that qualify it go to stderr"
type: bug
priority: P3
status: todo
effort: S
created-at: 2026-08-03T13:00:00+09:00
source: "TASK-088 finalize verification — 088's own Left open, untracked"
depends-on: [TASK-088]
scope: "dva repo — internal/cli/validate.go"
---

# Task 142: Put the verdict and its qualifiers on the same stream

## Problem

`dva validate` splits one answer across two streams. The verdict goes to stdout:

```
✅ dva.yml is valid
```

The warnings that qualify it — deprecated `stack.*.order`, pending migrations, their
`migration_guide` and `affected_entries` — go to stderr. A human or script reading only
stdout sees an unqualified ✅ and learns nothing about work that is due.

Measured 2026-08-03 with `bin/dva` v0.1.44 on a config using `stack.*.order`: plain stdout
21 bytes (`✅ dva.yml is valid`), stderr 837 bytes carrying two warnings. Exit 0 either way.

`--json` does not have this problem: TASK-088 put the warnings inside the single stdout
document as `.warnings`, each with a `fields` object. It is only the human path that is
split.

## Why it has no owner

TASK-088 named it a non-goal and recorded it under "Left open". TASK-093 restated it,
saying the stream question was "not settled, only reduced from four independent decisions
to three", and named TASK-088 as one of the carriers. Both are archived. A sweep of
`tasks/todo/`, `tasks/blocked/` and `tasks/plan/` on 2026-08-03 found nothing tracking it.

## Acceptance criteria

- [ ] Decide and record the rule: warnings that qualify a stdout verdict travel with it, or
      stdout is reserved for machine-consumable results and the ✅ line moves to stderr too.
      One rule, stated once, applied to `validate`.
- [ ] `dva validate` on the `stack.*.order` fixture puts the verdict and both warnings on
      the same stream — print the byte counts for stdout and stderr before and after.
- [ ] `--json` output is unchanged: still exactly one document
      (`dva validate --json 2>/dev/null | jq -s 'length'` → 1) on all four paths
      (clean, warnings, schema error, load failure), plus `--json --strict`.
- [ ] `TestValidateWithoutJSONIsUnchanged` is updated deliberately, not deleted — it exists
      to catch exactly this kind of move, so the new expected bytes are the deliverable.
- [ ] `make test` exits 0.

## Notes

The same stream question is open for the `note:` renderer (TASK-141). Answer it once and
apply it to both, rather than deciding it twice.

The migration URL in TASK-088's prose is stale — the binary now emits
`docs/42-migration-and-compatibility.md#11-migration`, not `docs/40-…`. Narrative drift in
the archived task only; the deliverable is correct.
