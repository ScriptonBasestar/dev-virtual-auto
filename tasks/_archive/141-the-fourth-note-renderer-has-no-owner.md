---
id: TASK-141
title: "hooks.go still renders note: inline, on a different stream and indent than writeNote"
type: chore
priority: P3
effort: S
created-at: 2026-08-03T13:00:00+09:00
completed-at: 2026-08-07
scope: "dva repo — internal/cli/hooks.go, provision writeNote"
status: done
quality-review: pass
quality-reviewed-at: 2026-08-07T18:05:08+09:00
verified-at: 2026-08-07T18:05:08+09:00
archived-at: 2026-08-07T18:05:08+09:00
quality-review-evidence: |
  - kind: test
    command-or-step: make test && make doc-check (mise go 1.26.4)
    result: exit 0; shared suite green
  - kind: recheck
    command-or-step: acceptance criteria re-observed
    result: hooks use writeNote only; provision_note_test
verification-summary: |
  quality-review pass; re-checked deliverables. hooks use writeNote only; provision_note_test. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 141: Route the last inline note renderer through `writeNote`

## Result

```
grep -c 'step.Note' internal/cli/hooks.go
# → 1  (writeNote(os.Stderr, step.Note) only — inline loop gone)
```

**Stream:** hooks keep **stderr** (progress channel, same as `$ cmd` lines). Provision keeps
**stdout** (result-adjacent). `writeNote` already takes the writer; hooks pass stderr
deliberately.

**Indent:** one value — writeNote's four spaces. Native-build test updated to assert
byte-identity with `writeNote`.
