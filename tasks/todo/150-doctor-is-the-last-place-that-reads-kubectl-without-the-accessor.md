---
id: TASK-150
title: "checkStackFiles reads entry.Kubectl directly, so a runners-form kubeconfig is never checked"
type: bug
priority: P3
status: todo
effort: S
created-at: 2026-08-03T13:40:00+09:00
source: "TASK-102 finalize verification — residual of the same mechanism, outside 102's blast radius table"
depends-on: [TASK-102]
scope: "dva repo — internal/cli/doctor.go"
---

# Task 150: Route the last raw `.Kubectl` read through `KubectlConfig()`

## Problem

TASK-102 introduced `entry.KubectlConfig()` so both declaration forms — the legacy typed
field and the `runners:` map — resolve to the same config. Eighteen call sites use it.
One does not.

`internal/cli/doctor.go:488-489` (`checkStackFiles`) iterates `c.Stack` and reads the typed
field directly:

```go
if entry.Kubectl != nil {
    files = []string{entry.Kubectl.Kubeconfig}
}
```

Measured 2026-08-03: `grep -rn '\.Kubectl\.' --include='*.go' internal/` over non-test code
returns exactly **one** line — this one — against **18** uses of `KubectlConfig()`. It is
the last direct typed-field read on a raw-map path.

The consequence: an entry declaring its kubeconfig under `runners.kubectl` is skipped
entirely by the file check. On a fixture whose two entries differ only in declaration form,
both with missing kubeconfig files:

```
[FAIL] Stack file exists: legacyk8s (missing-legacy.yaml)
                                        # ...and no row at all for modernk8s
```

The user who wrote the modern form gets silence where the legacy form gets a diagnosis.

## Severity

Lower than the defect TASK-102 fixed — this is a missing diagnostic, not a wrong action.
But it is a diagnostic tool reporting on a config it cannot fully see, which is the failure
mode `doctor` exists to prevent.

Untracked: nothing in `tasks/todo/`, `tasks/blocked/`, `tasks/decision/` or `tasks/plan/`
references TASK-102 or `checkStackFiles`. TASK-139 also touches `internal/cli/doctor.go`,
but it is about how a failing row is *worded*, not what the check *inspects*.

## Acceptance criteria

- [ ] `checkStackFiles` uses `entry.KubectlConfig()`. Print
      `grep -rn '\.Kubectl\.' --include='*.go' internal/ | grep -v _test.go` — it must
      return 0 lines, and say so with the count.
- [ ] Both declaration forms produce a row on the fixture above; show both rows.
- [ ] A test covers the `runners.kubectl` form through `doctor`, not only through the
      accessor's own unit test — the accessor was already correct; the caller was not.
- [ ] Check the other `doctor.go` checks for the same shape while here. This one was found
      by exercising a fixture, not by enumeration; state how many raw-map iterations were
      reviewed.
- [ ] `make test` exits 0.

## Notes

Coordinate with TASK-139, which rewords `DoctorResult` rows in the same file. Landing them
in either order is fine; landing them blind to each other will conflict.
