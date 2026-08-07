---
id: TASK-150
title: "checkStackFiles reads entry.Kubectl directly, so a runners-form kubeconfig is never checked"
type: bug
priority: P3
effort: S
created-at: 2026-08-03T13:40:00+09:00
completed-at: 2026-08-07
scope: "dva repo — internal/cli/doctor.go"
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
    result: doctor KubectlConfig() accessor; CheckStackFiles tests PASS
verification-summary: |
  quality-review pass; re-checked deliverables. doctor KubectlConfig() accessor; CheckStackFiles tests PASS. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 150: Route the last raw `.Kubectl` read through `KubectlConfig()`

## Acceptance criteria

- [x] `checkStackFiles` uses `entry.KubectlConfig()`.
- [x] Both declaration forms produce a row (`TestCheckStackFilesReportsBothKubectlForms`).
- [x] Reviewed other doctor.go stack iterations for raw typed fields: **0** remaining
      `entry.(Compose|Helm|…)` direct reads; compose path already uses `ComposeConfig()`.
- [x] `make test` exits 0.

## Result

```
grep -rn '\.Kubectl\.' --include='*.go' internal/ | grep -v _test.go
# → 0 lines
```

`checkStackFiles` now uses `entry.KubectlConfig()` so `runners.kubectl.kubeconfig` is checked.
