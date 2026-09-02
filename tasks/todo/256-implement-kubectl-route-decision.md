---
id: TASK-256
title: "Implement the approved kubectl route decision"
type: feature
priority: P1
effort: M
exec-tier: standard
created-at: 2026-09-02T10:08:00+09:00
source: "PLAN-003 approved TASK-255 contract"
scope: "kubectl command registration, compatibility reservation, parity tests, manifest, completion, and documentation"
status: todo
depends-on: [TASK-255, TASK-272]
---

# Task 256: implement kubectl route decision

## Summary

Implement exactly the route and compatibility contract approved in TASK-255. If the decision retains
`ktl`, do not manufacture a `kubectl` alias; close only the documentation or validation gaps identified by
the approved decision.

## Completion Criteria

- [ ] Command registration, canonical naming, visibility, aliases, and reserved-name handling match the approved TASK-255 record; every coexisting public name preserves the approved load-warning/validate-error/bare-built-in/explicit-`run`/reserved-prefix collision matrix | verify: `/usr/bin/grep -Eq '^func TestKubectlCompatibilityRouteParity\(' internal/cli/ktl_flag_passthrough_test.go && go test ./internal/cli ./internal/config -count=1`
- [ ] Canonical and compatibility invocations have identical root-flag handling, entry selection, kubectl argv, stdin/stdout/stderr, debug mode, exit status, signal forwarding, and process-replacement behavior except for explicitly approved warnings | verify: `go test ./internal/cli -count=1`
- [ ] Root help, direct help, shell completion, user documentation, canonical skills, and generated projections identify canonical and compatibility status consistently; manifest uses the approved existing representation or waits for the bounded route-identity child rather than changing schema inside this task | verify: `make generate && make check-generate && make doc-check`
- [ ] No compatibility name is removed and no deprecation warning is added outside the exact release/channel contract approved in TASK-255 | verify: human — diff and tests must be checked against the signed decision record
- [ ] Repository gates pass | verify: `make lint && make test && make test-integration && make commit-check`

## Non-goals

- No kubectl feature or runner refactor unrelated to the route.
- No unapproved compatibility removal.
- No broader command registry refactor.
