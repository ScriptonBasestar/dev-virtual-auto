---
id: TASK-239
title: "Doctor treats optional env files as required readiness failures"
type: bug
priority: P1
effort: S
created-at: 2026-08-29T22:29:37+09:00
source: "DVA wave follow-up reproduced missing optional env files as false doctor failures"
scope: "env_file required metadata, doctor built-in results, tests, usage documentation, and changelog"
status: todo
---

# Task 239: make doctor honor optional env file semantics

## Problem

The environment loader skips a missing `required: false` file, but doctor flattens declarations to
paths and reports every missing file as a failed built-in check. `dva doctor --strict` therefore
rejects configurations that the runtime intentionally accepts.

## Completion criteria

- [ ] Normalized env-file access preserves each path's required flag while path-only callers keep their existing contract | verify: `go test ./internal/config -run TestAllEnvFileConfigsPreservesRequiredMetadata`
- [ ] Doctor omits missing optional files, fails missing required files, and reports existing optional and required files as passing | verify: `go test ./internal/cli -run 'TestDoctorEnvFiles|TestDoctorFailRow'`
- [ ] User documentation and the changelog describe required-only missing-file diagnostics | verify: `make doc-check`
- [ ] Repository gates pass before integration | verify: `make lint && make test && make test-integration && make commit-check`

## Decision

Keep missing optional files out of the result set rather than rendering a misleading passing
"exists" assertion. Preserve the existing stable result name for files that exist and for missing
required files. Project configurations must not create placeholder secrets or change optional files
to required merely to silence doctor.
