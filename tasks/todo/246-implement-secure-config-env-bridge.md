---
id: TASK-246
title: "Implement the decided config env bridge without exposing plaintext"
type: feature
priority: P0
effort: L
exec-tier: standard
created-at: 2026-09-01T19:22:00+09:00
source: "PLAN-002 frozen security boundary and TASK-245 decision"
scope: "env_file model, Cobra commands, sops runner, safe writer, output fixtures, integration tests, user documentation"
status: todo
depends-on: [TASK-245, TASK-248]
---

# Task 246: implement the secure config env bridge

## Summary

Implement the contract selected by TASK-245 with fail-closed selection, cross-platform write safety,
and negative secret-leak tests.

## Problem

The bridge must materialize an explicitly selected dotenv target without making DVA a secret manager,
without changing legacy env-file shapes, and without leaving a partial or leaked plaintext file.

## Completion Criteria

- [ ] Implement exactly TASK-245's schema and command grammar across schema, runtime normalization, merged config output, and documentation; existing string/list/object shapes preserve their observable load order, required semantics, merge result, and show round-trip | verify: `/usr/bin/grep -Eq '^func TestConfigEnvLegacyShapeRoundTrip\(' internal/config/envfile_test.go && go test ./internal/config -count=1`
- [ ] Encrypted source metadata is accepted only at the decided effective top-level origins with preserved provenance and is rejected rather than ignored in interaction, nested subcommand, ambiguous module/override merge, and subproject shapes outside that contract | verify: `/usr/bin/grep -Eq '^func TestConfigEnvSourceMetadataScope\(' internal/config/envfile_test.go && go test ./internal/config -count=1`
- [ ] Invoke an injectable sops runner by argv without a shell, pin dotenv input/output behavior, and return exit 1 for binary absence, decryption failure, invalid/empty output, or cancellation | verify: `go test ./internal/cli -count=1`
- [ ] Resolve root/module/override/subproject paths exactly as TASK-245 decided, revalidate source and target type/containment at use time, and prevent path-component or symlink swaps between preflight and replace | verify: `/usr/bin/grep -Eq '^func TestConfigEnvRejectsPathSwap\(' internal/cli/config_env_test.go && go test ./internal/cli -count=1`
- [ ] Write through a same-directory 0600/O_EXCL temporary file, validate dotenv before replacement, sync the file and required parent directory, and implement the decided owned stale-temp recovery without claiming SIGKILL or power-loss cleanup that cannot be guaranteed | verify: `/usr/bin/grep -Eq '^func TestConfigEnvAtomicWriteFaultMatrix\(' internal/cli/config_env_test.go && go test ./internal/cli -count=1`
- [ ] Failure preserves an existing target byte-for-byte; success leaves no live temporary artifact; concurrent writers serialize or one fails explicitly without lost update; create/write/sync/close/replace/recovery failures are injected | verify: `/usr/bin/grep -Eq '^func TestConfigEnvConcurrentWriters\(' internal/cli/config_env_test.go && go test ./internal/cli -count=1`
- [ ] A secret sentinel is absent from stdout, stderr, JSON, debug logs, error strings, and temporary names for success and every injected failure | verify: `/usr/bin/grep -Eq '^func TestConfigEnvNeverEmitsSecretSentinel\(' internal/cli/config_env_test.go && go test ./internal/cli -count=1`
- [ ] Fake-sops fault tests and a pinned real-sops integration test pass on every OS TASK-245 declares supported; safe-writer and command tests continuously cover that declared matrix, while every undeclared platform takes an explicitly tested unsupported fail-closed path | verify: `/usr/bin/grep -Eq '^func TestConfigEnvRealSOPS\(' internal/integration/config_env_test.go && /usr/bin/grep -q 'config-env-platform' .github/workflows/ci.yml && make test-integration`
- [ ] User docs explain explicit unseal, no stdout show, no lifecycle auto-unseal, Git/path safeguards, and recovery without recommending a command that does not exist | verify: `make doc-check`
- [ ] Full repository and release snapshot gates pass | verify: `make lint && make test && make test-integration && make check-generate && make release-check && make commit-check`

## Non-goals

- No top-level `env` reservation.
- No `env show`.
- No age provider/key configuration in `dva.yml`.
- No automatic unseal from lifecycle commands.
