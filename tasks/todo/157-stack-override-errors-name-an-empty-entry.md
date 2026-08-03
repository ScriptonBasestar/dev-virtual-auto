---
id: TASK-157
title: "A stack_overrides merge error names an empty entry, because those entries carry no Name"
type: bug
priority: P4
status: todo
effort: S
created-at: 2026-08-03T14:45:00+09:00
source: "TASK-116 finalize verification — its 'Observed but deliberately not fixed' note, untracked"
depends-on: [TASK-116]
scope: "dva repo — internal/config/merge.go, stack_overrides parse"
---

# Task 157: Decide whether stack_overrides entries get a Name

## Problem

When a `stack_overrides` entry fails to merge, the inner error names an empty entry:

```
[warn] stack_override "api": … for stack entry ""
```

The `LifecycleEntry` values under `stack_overrides` carry no `Name`, so the inner half of the
message has nothing to print. The outer half still names the key (`"api"`), so the user can find
the offending override — the message is imprecise, not useless.

TASK-116 recorded this and deliberately did not fix it, on the grounds that the fix means
deciding whether `stack_overrides` entries should have `Name` backfilled at parse time — a
config-semantics question, not the stdout question that task was answering. That reasoning holds.
This task exists so the finding survives TASK-116's archival: it was recorded only inside the
task file, and nothing in `tasks/todo|blocked|decision|plan` mentioned it.

## The actual question

Whether an entry's `Name` is part of its identity or part of its position.

- **A — backfill `Name` from the map key at parse time.** The message becomes precise everywhere,
  and any other code reading `.Name` off a `stack_overrides` entry stops seeing `""`. Cost:
  `Name` now means two different things depending on where the entry came from, and a key/`Name`
  disagreement inside an override becomes possible and has to be resolved.
- **B — leave `Name` empty and fix the message.** The error formatter takes the key it already
  has in scope. Smallest change, no semantics moved. Cost: the next reader of `.Name` on an
  override entry hits the same empty string.

## Acceptance criteria

- [ ] Pick A or B and record why in the Resolution.
- [ ] Reproduce the current message first, against the real binary, and paste it — then the fixed
      one, same fixture.
- [ ] Under A: count every site that reads `.Name` off an entry that may have come from
      `stack_overrides`, and say what each now sees. Under B: confirm no other site prints an
      override entry's `Name`, with the count.
- [ ] A test pins the message, keyed on the override name, so an empty entry name fails.
- [ ] `make test` exits 0.
