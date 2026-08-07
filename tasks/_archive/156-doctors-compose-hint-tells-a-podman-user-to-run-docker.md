---
id: TASK-156
title: "Doctor runs the configured compose tool and then tells the user to reproduce it with docker"
type: bug
priority: P3
effort: S
created-at: 2026-08-03T14:45:00+09:00
source: "TASK-119 finalize verification — the wrong binary survived in the advice path"
depends-on: [TASK-119]
scope: "dva repo — internal/cli/doctor.go:614, :110"
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
    result: composePrefix in doctor hint; DoctorCompose tests PASS
verification-summary: |
  quality-review pass; re-checked deliverables. composePrefix in doctor hint; DoctorCompose tests PASS. Shared: make test OK; make doc-check OK (mise go 1.26.4).
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

- [x] The hint names `composeCmd`, matching the skip branch two lines up (`doctor.go:614` now
      `fmt.Sprintf("… then run: %s config", composeCmd)`).
- [x] Proven by `TestDoctorComposeConfigReportsWhatTheCommandSaid` (configures `podman-compose`,
      asserts tail `run: podman-compose config`, NOT `docker compose config`) AND
      `TestDoctorComposeConfigDefaultCommandHintDockerCompose` (no command configured, asserts
      `run: docker compose config`). The default-docker case is preserved by reconstructing the
      prefix — see Review C1.
- [x] That test is extended to pin the tail to the configured command (it previously asserted only
      the leading line).
- [x] `doctor.go:110`'s comment corrected to "Runs the configured compose command's `config`".
- [x] Sweep: of `doctor.go`'s hints, only the compose-config hint (614) hardcoded a binary the
      check no longer assumes. `doctor.go:321` ("Add user to docker group") is genuinely
      docker-specific — `checkDockerSocketPermissions` stats `/var/run/docker.sock`, so the advice
      is correct. The other `docker compose` mentions (357/369/434/560/561) are explanatory
      comments, not user-facing hints. **Count: 1 hint + 1 comment fixed; no other hint hardcodes
      a binary the check no longer assumes.**
- [x] `make test` exits 0.

## Review

Combined `core:code-reviewer` pass caught a regression before it shipped:

- **C1 (Critical, fixed)** — the first attempt interpolated `composeCmd`, but `ComposeArgv` returns
  the binary name only ("docker"); the `compose` subcommand lives in `args`. So the default case
  produced `run: docker config` (not a real command) — worse than the original bug. An earlier line
  here claimed the default case was unchanged; that was wrong. Fixed by deriving the prefix from
  `cc.Command` (or the "docker compose" default), which is the user-facing invocation.
- **H1 (test gap, fixed)** — the test only exercised `podman-compose` (a single-token command, where
  the bug was invisible). Added `TestDoctorComposeConfigDefaultCommandHintDockerCompose` covering
  the no-command default, which fails on the C1 regression.

The sweep claim (only this hint hardcodes a binary the check no longer assumes; the docker-socket
and docker-daemon hints are genuinely docker-specific) was confirmed accurate.

## Notes

Distinct from [TASK-150](150-doctor-is-the-last-place-that-reads-kubectl-without-the-accessor.md)
(doctor reading `entry.Kubectl` instead of `entry.KubectlConfig()`) and
[TASK-139](139-doctor-rows-name-the-check-so-a-failure-reads-as-a-pass.md) (rows named after the
assertion, so a failure reads as a pass). Same file, three unrelated surfaces.
