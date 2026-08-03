---
id: TASK-134
title: "Naming presets and forbidden ports are hand-maintained in the flow library with no Go source of truth"
type: chore
priority: P3
status: todo
effort: S
created-at: 2026-08-03T12:05:00+09:00
source: "TASK-061 finalize verification — the deferred Phase 2 existed only as README prose"
depends-on: [TASK-061]
scope: "dva repo — agent-mesh-flows/shared/library/, tools/libgen, internal/config"
---

# Task 134: Give the last two hand-authored flow-library facts a Go source

## Problem

TASK-061 removed the hand-copying of Go facts into the agent-mesh flow library for
two rules, and left two more explicitly out of scope:

- **Rule 23** — naming presets
- **Rule 7** — forbidden ports

Both are still typed by hand into `agent-mesh-flows/shared/library/`, with no Go
source of truth and no `AUTOGEN` block. They can drift from the validator exactly
the way rules 9 and 14 did before TASK-061, and nothing would catch it — the
`make check-generate` gate only diffs what the generator writes.

The deferral is recorded, but only as narrative prose:
`agent-mesh-flows/shared/library/README.md:29` — "Facts still authored here (Phase
2 migration candidates)". A finalize sweep of TASK-061 on 2026-08-03 searched
`tasks/todo`, `tasks/plan`, `tasks/blocked` and the whole repo for any ROADMAP or
BACKLOG entry and found none. This file exists so the work is discoverable rather
than living in a paragraph.

## Two acceptable outcomes

This is a decision task first, work second.

**A — migrate.** Give each rule a Go source in `internal/config` (as `reserved.go`
and `validate_warnings.go` are for rules 9 and 14), teach `tools/libgen` to emit
its `AUTOGEN` block, and add both to the `check-generate` diff set.

**B — drop the framing.** If neither rule is worth a Go source — e.g. they are
guidance rather than anything the validator enforces — remove the "Phase 2
migration candidates" heading from the README so it stops advertising work nobody
intends to do, and say plainly that these two are authored here on purpose.

Do not leave the third state, which is what exists today: a stated intention with
no owner and no tracking.

## Acceptance criteria

- [ ] Outcome A or B is chosen and recorded in this file's frontmatter as `decision:`.
- [ ] If A: rule 23 and rule 7 each have a Go source of truth under `internal/config`.
- [ ] If A: `tools/libgen` emits both blocks, and `make generate` twice in a row
      leaves the tree clean (`make check-generate` exits 0).
- [ ] If A: `agent-mesh-flows/shared/library/README.md` moves both rules from the
      "still authored here" list to the generated list.
- [ ] If B: the "Phase 2 migration candidates" section is gone from README.md:29
      and replaced by an explicit statement that these facts are authored by hand
      deliberately.
- [ ] `make test` exits 0.

## Notes

Phase 2-B is already settled and needs nothing here: `dva-schema.md` moved to
`skills/config/references/schema-reference.md` and is symlinked back into the
library (README.md:32-33).
