---
id: TASK-151
title: "The manifest never mentions --json — the flag its audience needs most"
type: bug
priority: P3
status: done
effort: S
completed-at: 2026-08-07
scope: "dva repo — internal/cli/manifest.go"
---

# Task 151

## Result

`global_flags` on the manifest root, derived from `rootCmd.PersistentFlags()`:

```json
[
  {"name":"debug","type":"bool","description":"Enable debug logging"},
  {"name":"dry-run","type":"bool","description":"Show execution plan without running"},
  {"name":"json","type":"bool","description":"Output in JSON format (LLM-optimized)"}
]
```

Per-command options still exclude them (`TestManifestPublishesEveryPersistentFlag`).
