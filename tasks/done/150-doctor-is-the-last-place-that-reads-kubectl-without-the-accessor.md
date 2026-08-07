---
id: TASK-150
title: "checkStackFiles reads entry.Kubectl directly, so a runners-form kubeconfig is never checked"
type: bug
priority: P3
status: done
effort: S
created-at: 2026-08-03T13:40:00+09:00
completed-at: 2026-08-07
scope: "dva repo — internal/cli/doctor.go"
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
