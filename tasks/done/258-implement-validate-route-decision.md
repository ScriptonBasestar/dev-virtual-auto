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
status: done
depends-on: [TASK-257, TASK-272]
---

# Task 258: implement validate route decision

## Summary

Implement exactly the route, visibility, compatibility, and documentation decision approved in TASK-257.
If both current routes remain coequal, do not introduce artificial warnings or aliases; close only verified
parity and documentation gaps.

## Completion Criteria

- [x] Route registration, canonical naming, help visibility, aliases, and reserved-name behavior match the approved TASK-257 record | verify: `/usr/bin/grep -Eq '^func TestValidateRouteCompatibilityContract\(' internal/cli/root_command_registration_test.go && go test ./internal/cli ./internal/config -count=1`
- [x] Every supported route has approved parity for config discovery, `--strict`, `--fix`, root persistent flags including `--json`, text/JSON output, diagnostics, stdout/stderr, and exit codes | verify: `go test ./internal/cli -count=1`
- [x] Root help, direct help, shell completion, README/USAGE, canonical skills, and generated projections present canonical and compatibility status consistently; manifest uses the approved existing representation or waits for the bounded route-identity child rather than changing schema inside this task | verify: `make generate && make check-generate && make doc-check`
- [x] No route is hidden, warned, or removed outside the approved release and rollback contract | verify: human — compare the final diff and behavior fixtures with the signed TASK-257 record
- [x] Repository gates pass | verify: `make lint && make test && make test-integration && make commit-check`

## Non-goals

- No new validation rule or output schema unrelated to route identity.
- No broader `config` reorganization.
- No unapproved compatibility removal.

## Completion Evidence (2026-09-04)

All five criteria independently re-verified in the task worktree (not taken on the implementer's
report alone), on top of commit `d13fb63` "feat(cli): implement the approved validate route
decision (TASK-258)":

1. `grep TestValidateRouteCompatibilityContract` matched; `go test ./internal/cli ./internal/config -count=1` → both `ok`.
2. `go test ./internal/cli -count=1` → `ok` (includes existing `TestRootValidateMatchesConfigValidate`/parity coverage).
3. `make generate && make check-generate && make doc-check` → clean regen (0 diff), `doc-check: OK`, `cilabels: OK`, `flowcheck: OK`.
4. Diff reviewed directly (`git show d13fb63`): adds `ManifestCmd.CanonicalName` (TASK-272 Option B), bumps `schema_version` 1.4→1.5 (ownership assigned to whichever of TASK-256/TASK-258 starts first, per TASK-272's Decision Record — TASK-258 started first), marks `static_commands.validate.canonical_name = "config validate"`, and adds the previously-missing `config` subcommand coverage (docs/env/init/migrate/show/validate). No route hidden, warned, aliased, or removed.
5. `make lint && make test && make test-integration && make commit-check` → all exit 0.

Schema-ownership note for TASK-256: `ManifestCmd.CanonicalName` and `schema_version = "1.5"` already
exist — TASK-256 populates the field for the kubectl/ktl pair, it does not add it.
