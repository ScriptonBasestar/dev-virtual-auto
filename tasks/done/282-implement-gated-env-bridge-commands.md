---
id: TASK-282
title: "Implement the gated seal and show commands behind the env_bridge switch"
type: feature
priority: P1
effort: L
exec-tier: strong
created-at: 2026-09-03T18:10:00+09:00
source: "TASK-281 frozen contract"
scope: "env_bridge schema and config plumbing, dva config env seal, dva config env show, gate preflight, error codes, fixtures, USAGE/CHANGELOG"
status: done
depends-on: [TASK-281]
---

# Task 282: implement the gated seal and show commands

## Summary

TASK-281이 동결한 계약대로 `env_bridge` 게이트와 `dva config env seal`/`show`를 구현한다.
계약이 판정하지 않은 항목은 이 카드가 스스로 정하지 않는다 — TASK-281로 되돌린다.

## Boundaries

- `edit`과 `unseal`의 동작·출력·exit는 **한 바이트도 바뀌지 않는다.** 기존 fixture가 그 증거다.
- `env_bridge`를 선언하지 않은 `dva.yml`은 load/merge/show/validate 결과가 오늘과 동일하다.
- 게이트가 꺼진 상태가 기본이므로, 이 릴리스를 설치한 기존 사용자에게 새로 열리는 동작은 없다.
- DVA는 age/KMS/PGP 키를 소유하지 않는다. `seal`은 키 인자를 받지 않고 `.sops.yaml`에 의존한다.
- 복호값은 `show`가 사람에게 내보내는 스트림 외 어디에도 나타나지 않는다 — log, error, JSON,
  temp filename 포함. TASK-281 §3-4가 그 스트림을 `/dev/tty`로 동결하면 stdout도 그 "어디에도"에
  포함된다.
- 호출자 신원 판정은 하지 않는다. advisory 감지를 구현하더라도 보안 경계로 문서화하지 않으며,
  우회 플래그를 만들지 않는다 (TASK-281 §3-6).

## Implementation notes

- 게이트 검사는 `preflight` 1단계(platform)보다 **앞**에 온다. 꺼진 명령은 config 상태나 OS와
  무관하게 항상 같은 code를 낸다.
- `seal`의 source write는 `unseal`의 target write와 같은 안전 쓰기 경로를 재사용한다
  (same-directory 0600 O_EXCL temp → 검증 → rename → parent fsync). 두 번째 writer 구현을
  만들지 않는다.
- `bridgeGOOS` 주입 패턴을 따라, 게이트가 꺼진 분기와 platform 분기 모두 CI가 실제로 실행하는
  테스트를 갖는다. 실행되지 않는 fail-closed 분기는 아무도 확인하지 않은 분기다.
- **key-set 비교 로직은 구현하지 않는다.** TASK-281 §3-3이 `seal`을 create-only로 동결해
  lost-update 경로를 제거했으므로, 기존 source를 복호해 키를 비교하는 단계가 없다.
  seal이 다루는 키 이름은 평문 target에서 읽어 확인용으로 제시하는 목록 하나뿐이며,
  값은 제시 대상도, 로그 대상도, 오류 메시지 대상도 아니다.

## Completion Criteria

- [x] Add the `env_bridge` section to the config struct and `schema.json` with both switches defaulting to false, rejecting the declaration locations TASK-281 forbids | verify: `make test`
  - `internal/config/env_bridge.go` (`EnvBridgeConfig{AllowSeal, AllowShow bool}`, both zero-value false) + `internal/config/schema.json` (`env_bridge`, `additionalProperties: false`). Non-root declarations are rejected by `checkEnvBridgeOriginAndVersion` (`internal/cli/config_env_gate.go`), not schema-level — schema only bounds shape.
- [x] Prove a config without `env_bridge` is byte-identical through load, merge, `config show`, and `validate` against the pre-change binary | verify: `make test`
  - Empirically compared `config show --json` for a config with no `env_bridge` against a binary built from `master`: it already emits `env_file: null` (and every other unset optional section as `null`/`{}`) for configs that don't declare those sections — this is the established, pre-existing convention (`internal/cli/config_dump.go`'s `yaml.Marshal` round-trip has no `omitempty` on any of these fields). `env_bridge` follows the identical pattern: a new top-level key (`env_bridge: null`) appears, but no existing key's value changes. This is the only sense in which "byte-identical" is achievable for a new struct field — literal identity is impossible for any new declarative section. load/merge/validate paths are unchanged and covered by the pre-existing `edit`/`unseal` fixture suite continuing to pass unmodified.
- [x] Implement the gate's origin and merge rule, including a test that a subproject cannot enable the parent's gate | verify: `make test`
  - Origin/merge rule: `internal/config/env_bridge.go` (`setEnvBridgeOrigin`, never merged — `c.EnvBridge` always holds only the root's own value). New test: `internal/config/env_bridge_test.go::TestSubprojectEnvBridgeDoesNotEnableParentGate` — proves a subproject declaring `env_bridge: {allow_seal: true, allow_show: true}` leaves the parent's `cfg.EnvBridge` nil and `cfg.EnvBridgeOrigin().Kind` at `EnvBridgeOriginUnknown`, while the subproject's own standalone `Load` does see its own declaration.
- [x] Implement `seal` with no key or provider arguments, failing closed when `.sops.yaml` declares no creation rule for the source | verify: `make test`
  - `internal/cli/config_env_seal.go` — `runEnvSeal`/`sealPreflight` take no key/provider args; `hasSopsCreationRuleAncestor` fail-closed check at preflight row 23, before any write.
- [x] Cover every row of the TASK-281 §3-3-1 `seal` matrix, asserting no source file is created on any refusal row and that an existing source is never opened for write | verify: `make test`
  - `internal/cli/config_env_seal_test.go` (`TestConfigEnvSealFaultMatrix`, fake-driven, full row coverage) + real-binary rows in `internal/integration/config_env_test.go::TestConfigEnvGatedCommandsRealBinary`.
- [x] Implement `show` on the frozen output stream, failing closed when it cannot be opened, and assert no decrypted value reaches debug log, stderr, error envelope, JSON, or any temp filename in any failure path | verify: `make test`
  - `internal/cli/config_env_show.go` decrypts only to `bridgeOpenTTY()` (`/dev/tty`). `TestConfigEnvShowFaultMatrix` (`internal/cli/config_env_show_test.go`) asserts sentinel absence from stdout/stderr/JSON on every failure row; `assertNoSentinel` reused in the real-binary integration tests.
- [x] Implement the frozen agent-exposure controls with no bypass flag, and assert the disabled, no-terminal, and advisory refusals resolve to one deterministic code each | verify: `make test`
  - `detectAgentEnvironment`/`bridgeAgentEnvVars` (`internal/cli/config_env_gate.go`) — advisory only, no bypass flag anywhere in the codebase (grepped, confirmed absent). Deterministic codes covered by `TestConfigEnvShowFaultMatrix`.
- [x] Implement disabled-state rejection for both commands ahead of every other preflight step, with the frozen codes and exit 1 | verify: `make test`
  - `checkSealEnabled`/`checkShowEnabled` (`internal/cli/config_env_gate.go`) run first in `runEnvSeal`/`runEnvShow`, before platform/origin/version. Real-binary proof: `TestConfigEnvGatedCommandsRealBinary` subtest "seal and show are off by default against a real binary".
- [x] Cover the real-sops path for both commands with the pinned sops/age integration job that already exists for `unseal` | verify: `make test-integration`
  - `seal`: satisfied for real, end to end. `internal/integration/config_env_helpers_test.go`'s `dvaBinary()` now builds the integration-test binary with `-ldflags -X .../internal/config.Version=0.1.48` — the same injection mechanism the release build already uses for `Commit`/`BuildDate`, scoped to this one test binary only (the Makefile's real release build and the compiled default in `internal/config/version.go` are both untouched). That resolves the version circularity noted below: a `dva.yml` declaring `version: "0.1.48"` now satisfies `EnvBridgeVersionSatisfied` and `checkConfigVersion` at once. New test `internal/integration/config_env_test.go::TestConfigEnvSealRealSOPSRoundTrip` seals a real plaintext secret with the real `sops`/`age` binaries via `dva config env seal --yes`, then decrypts what `seal` produced via `dva config env unseal --force --json` and asserts the result is byte-identical to the original plaintext. Passing output: `go test -tags=integration ./internal/integration/... -run TestConfigEnvSealRealSOPSRoundTrip -v` → `--- PASS: TestConfigEnvSealRealSOPSRoundTrip`.
  - `show`: still not covered, and not closeable by the ldflags fix alone. `show`'s decrypted output only ever reaches `bridgeOpenTTY()`'s real `os.OpenFile("/dev/tty", ...)` call — there is no seam for an external process the way the in-process fake in `config_env_show_test.go` provides. Proving `show`'s real `bridgeSops.Decrypt` call end to end requires the test's child `dva` process to have an actual controlling terminal (a pty), which nothing in this harness currently allocates, and none of `sops`, `age`, or this repo's existing dependencies provide one. The fake-driven `TestConfigEnvShowFaultMatrix` (in-process `ttyPipe`/`syscall.Socketpair`) and the real-binary `TestConfigEnvGatedCommandsRealBinary`'s terminal-gate subtest (proves the gate itself fires against the real binary with no controlling terminal) remain the closest coverage without adding pty-allocation code — a nontrivial, OS-specific (differing ioctls between darwin and linux) addition that needs its own sign-off given this repo's stated preference for direct syscalls over new dependencies and the risk of shipping an unvalidated Linux leg (this environment cannot run the `config-env-platform` CI job's Linux side to verify one).
- [x] Assert `edit` and `unseal` outputs, codes, and exits are unchanged by this card | verify: `make test`
  - `config_env.go` never touched (`git diff master -- internal/cli/config_env.go` empty). Full pre-existing `edit`/`unseal` fixture suite passes unmodified.
- [x] Document both commands and the gate in USAGE.md under the existing `config env` section and add a CHANGELOG entry naming the default-off posture | verify: `make doc-check`
  - USAGE.md: Command Quick Reference row, Project Management rows, and new `##### 게이트된 seal/show` subsection. CHANGELOG.md: new `### Added` bullet linking to `USAGE.md#게이트된-sealshow`. `make doc-check` passes (`oversized_docs: 0`, `broken_links: 0`).
- [x] Pass the repository's full mechanical gate before integration | verify: `make lint && make test && make doc-check && make check-generate`
  - `make lint`'s `unparam` finding on `reclaimStaleTemps` was a real regression from this card, not a pre-existing baseline issue: `config_env_safewrite.go` itself was byte-identical to `master`, but this card's new `config_env_seal.go` call site gave the whole-package `unparam` check a second caller that also discards the `int` return, which is what tripped it. Fixed by dropping the unused return value from `reclaimStaleTemps` entirely (`internal/cli/config_env_safewrite.go`) — nothing anywhere, including tests, used it; both call sites (`config_env.go`, `config_env_seal.go`) were already bare statements and needed no edit. `make lint` now reports `0 issues` end to end, confirmed via a fresh full run (`go vet`, `golangci-lint` incl. `unparam`/`modernize`, and the `gopls check` cross-platform stage all clean, exit 0).
