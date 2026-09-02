---
id: TASK-258
title: "Implement the approved validate-route decision"
type: feature
priority: P1
effort: M
exec-tier: standard
created-at: 2026-09-02T10:10:00+09:00
source: "PLAN-003 approved TASK-257 contract"
scope: "validate route registration, parity tests, help, manifest, completion, documentation, and generated skills"
status: todo
depends-on: [TASK-257, TASK-272]
---

# Task 258: implement validate route decision

## Summary

Implement exactly the route, visibility, compatibility, and documentation decision approved in TASK-257.
If both current routes remain coequal, do not introduce artificial warnings or aliases; close only verified
parity and documentation gaps.

## Completion Criteria

- [ ] Route registration, canonical naming, help visibility, aliases, and reserved-name behavior match the approved TASK-257 record | verify: `/usr/bin/grep -Eq '^func TestValidateRouteCompatibilityContract\(' internal/cli/root_command_registration_test.go && go test ./internal/cli ./internal/config -count=1`
- [ ] Every supported route has approved parity for config discovery, `--strict`, `--fix`, root persistent flags including `--json`, text/JSON output, diagnostics, stdout/stderr, and exit codes | verify: `go test ./internal/cli -count=1`
- [ ] Root help, direct help, shell completion, README/USAGE, canonical skills, and generated projections present canonical and compatibility status consistently; manifest uses the approved existing representation or waits for the bounded route-identity child rather than changing schema inside this task | verify: `make generate && make check-generate && make doc-check`
- [ ] No route is hidden, warned, or removed outside the approved release and rollback contract | verify: human — compare the final diff and behavior fixtures with the signed TASK-257 record
- [ ] Repository gates pass | verify: `make lint && make test && make test-integration && make commit-check`

## Non-goals

- No new validation rule or output schema unrelated to route identity.
- No broader `config` reorganization.
- No unapproved compatibility removal.
