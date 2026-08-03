---
id: TASK-156
title: "Doctor runs the configured compose tool and then tells the user to reproduce it with docker"
type: bug
priority: P3
status: todo
effort: S
created-at: 2026-08-03T14:45:00+09:00
source: "TASK-119 finalize verification — the wrong binary survived in the advice path"
depends-on: [TASK-119]
scope: "dva repo — internal/cli/doctor.go:614, :110"
---

# Task 156: Name the configured compose command in the failure hint

## Problem

TASK-119 made `checkComposeConfigResolves` run the tool the config actually names instead of a
hardcoded `docker`. The **hint printed when that check fails** was not converted
(`internal/cli/doctor.go:614`):

```go
hint := "check compose.files in dva.yml and any include: paths, then run: docker compose config"
```

Measured 2026-08-03 against a fixture whose `runners.compose.command` is a failing
`podman-compose` shim:

```
[FAIL] Compose config resolves
       -> include: ./missing.yml not found — check compose.files in dva.yml and any
          include: paths, then run: docker compose config
```

So DVA ran `podman-compose`, and then told the user to reproduce the failure with a binary they
do not have. This is the same wrong-binary defect TASK-119 removed from the execution path,
surviving one line below it in the advice path — and it is the line the user is most likely to
copy.

`composeCmd` is in scope at that point (`doctor.go:571`) and is already interpolated correctly by
the neighbouring skip branch, `doctor.go:596` and `:598`.

## Acceptance criteria

- [ ] The hint names `composeCmd`, matching the skip branch two lines up.
- [ ] Proven with a fixture that configures a non-docker compose command and fails: paste the
      `[FAIL]` block. Then paste the default-docker case, so the common path is shown unchanged.
- [ ] `TestDoctorComposeConfigReportsWhatTheCommandSaid`
      (`internal/cli/doctor_compose_test.go:189`) asserts only the hint's leading line, which is
      why nothing caught this. Extend it, or add a case, so the tail is pinned to the configured
      command.
- [ ] `internal/cli/doctor.go:110`'s call-site comment still says "Runs `docker compose config`,
      which needs no daemon" — untrue since TASK-119. Correct it in the same change.
- [ ] Sweep the rest of `doctor.go` for other hints that hardcode a binary the check no longer
      assumes, and report the count found.
- [ ] `make test` exits 0.

## Notes

Distinct from [TASK-150](150-doctor-is-the-last-place-that-reads-kubectl-without-the-accessor.md)
(doctor reading `entry.Kubectl` instead of `entry.KubectlConfig()`) and
[TASK-139](139-doctor-rows-name-the-check-so-a-failure-reads-as-a-pass.md) (rows named after the
assertion, so a failure reads as a pass). Same file, three unrelated surfaces.
