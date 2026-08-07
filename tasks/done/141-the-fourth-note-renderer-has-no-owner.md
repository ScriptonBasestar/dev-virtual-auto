---
id: TASK-141
title: "hooks.go still renders note: inline, on a different stream and indent than writeNote"
type: chore
priority: P3
status: done
effort: S
created-at: 2026-08-03T13:00:00+09:00
completed-at: 2026-08-07
scope: "dva repo — internal/cli/hooks.go, provision writeNote"
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
