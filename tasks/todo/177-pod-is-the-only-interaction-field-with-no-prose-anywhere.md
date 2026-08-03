---
id: TASK-177
title: "`pod:` appears in no document, so the kubectl runner's execution forms are undocumented"
type: chore
priority: P3
status: todo
effort: S
created-at: 2026-08-03T22:45:00+09:00
source: "TASK-175 — a behaviour change to the kubectl runner had nowhere to be written down"
depends-on: [TASK-175]
scope: "dva repo — USAGE.md interaction section"
---

# Task 177: The field that selects the kubectl runner has no prose

## Problem

`grep -rn "pod:" USAGE.md docs/*.md` returns **0**.

The kubectl runner is not entirely unmentioned — USAGE.md:469 lists `kubectl` among the core
plugin types, :304 documents `dva ktl`, and :624 records that the kubectl path forwards no
environment (TASK-129). But `pod:`, the field whose presence selects that runner
(`DetectRunnerType`, `runner.go:65`), is never named, and neither is what a `pod:` interaction can
declare. A reader who wants to run something in a pod has the runner list and nothing else.

This surfaced as a gap while fixing
[TASK-175](175-kubectl-runner-drops-script-and-script-file-and-runs-the-inherited-command.md),
which changed what `pod:` + `script:` does. The change had no document to land in, and writing a
paragraph about `script:`-in-a-pod while `pod:` itself is undocumented would explain a detail of a
feature the reader has never been told exists — so it was filed rather than half-written.

[TASK-149](../done/149-default-args-inheritance-is-documented-only-in-the-schema.md) established
USAGE.md as the home for interaction prose and the reasoning for why, which this can follow rather
than relitigate.

## Acceptance criteria

- [ ] `pod:` is documented in USAGE.md's interaction section: what it does, that it selects the
      kubectl runner, and the `pod:container` form `parsePod` accepts — the qualifier is currently
      inferable only from the code.
- [ ] The four execution forms are stated for the kubectl runner — `command:`, `steps:`, `script:`,
      `script_file:` — including that a script runs *inside the pod* via `sh -c` after TASK-175,
      and that a shebang is not honoured there. Otherwise the surprising half of the behaviour
      stays where it is now, in a comment.
- [ ] Say what the compose runner does differently for `script:` (falls back to the host), since
      the two runners genuinely disagree and a reader moving a config between them will hit it.
- [ ] `make doc-check` exits 0.

## Notes

Scope is `pod:` and the kubectl execution forms, not the whole runner. `runner:`, `service:` and
the compose options are separately documented already; widening this to "document every interaction
field" would make it a different, larger task.
