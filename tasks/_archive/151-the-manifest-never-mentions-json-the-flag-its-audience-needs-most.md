---
id: TASK-151
title: "The manifest never mentions --json — the flag its audience needs most"
type: bug
priority: P3
effort: S
completed-at: 2026-08-07
scope: "dva repo — internal/cli/manifest.go"
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
    result: Manifest GlobalFlags include json
verification-summary: |
  quality-review pass; re-checked deliverables. Manifest GlobalFlags include json. Shared: make test OK; make doc-check OK (mise go 1.26.4).
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
