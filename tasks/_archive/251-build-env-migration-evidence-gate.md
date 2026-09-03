---
id: TASK-251
title: "Build a versioned cross-repository env migration evidence gate"
type: feature
priority: P0
effort: L
exec-tier: strong
created-at: 2026-09-01T19:27:00+09:00
source: "PLAN-002 versioned migration evidence contract"
scope: "scanner, manifest schema, pinned corpus validation, nested command and stdout-decrypt detection, freshness checks"
status: superseded
superseded-by: TASK-252
closed-at: 2026-09-03T21:10:00+09:00
archived-at: 2026-09-03T21:10:00+09:00
depends-on: [TASK-246, TASK-248, TASK-252]
---

# Task 251: build the env migration evidence gate

## Summary

Turn cross-repository migration claims into pinned, reproducible, fail-closed evidence that a later
release decision can verify.

This task starts only when TASK-252 records that promotion evidence is worth collecting. If TASK-252
selects permanent `config env`, archive this card with explicit N/A disposition instead of building an
unused scanner or release gate.

## Disposition — N/A (2026-09-03)

[TASK-252](252-decide-top-level-env-promotion.md)가 영구 `config env`를 확정하고 top-level
`env` 승격을 거부했다. 이 카드는 승격 심사에 먹일 증거를 만드는 카드이므로, 그 증거를 소비할
결정이 사라졌다. 스캐너를 짓지 않고 N/A로 닫는다 — TASK-252 완료 기준 1이 명시한 처분이고,
이 카드 Summary가 이미 같은 조건을 걸어두었다("If TASK-252 selects permanent `config env`,
archive this card with explicit N/A disposition instead of building an unused scanner").

P0/L 카드를 코드 한 줄 없이 닫는 것이 낭비처럼 보이지만 반대다. 이 카드의 비용 전체가
승격이라는 하나의 선택지에 종속돼 있었고, 그 선택지가 기각됐다. 짓고 나서 쓰지 않는 것이
낭비다.

**되살리지 않는다.** 승격을 재개하려는 미래 작업은 이 카드를 여는 것이 아니라 새 카드로
시작한다 — 이 카드의 증거 계약은 external repository revision을 pin하고 SHA drift 0을
요구하는데, 그 시점의 외부 상태는 지금과 다르므로 여기 적힌 어떤 pin도 이미 낡았다.
계약 설계 자체는 참고할 가치가 있으니 삭제하지 않고 보존한다.

## Problem

An unversioned “scan 0 / validate 0 / four reports exist” claim is not a release gate. It does not
identify the external revisions, scanner, reserved-set simulation, ownership state, or dynamic-call
blind spots that produced the result.

## Completion Criteria

- [x] If TASK-252 first selects permanent `config env`, record that final decision and mark every evidence-branch criterion below `[~]` with the decision reference before archiving this card as N/A | verify: human — TASK-252 must close first and this card must preserve the explicit N/A rationale

아래 evidence-branch 기준은 전부 `[~]` — TASK-252가 승격을 기각했으므로 전제가 성립하지 않는다. 삭제하지 않고 보존한다:

- [~] Define a machine-readable manifest that pins canonical repository ID, commit SHA, inspected paths, base DVA commit/version, virtual reserved set, scanner digest, timestamp, and migration result; incomplete or unknown fields are errors | verify: `/usr/bin/grep -Eq '^func TestManifestRejectsIncompleteEvidence\(' tools/envgate/manifest_test.go && go test ./tools/envgate -count=1`
- [~] Scan interaction trees recursively, including nested subcommand bodies, and detect literal stdout decrypt behavior rather than only the top-level `env` key | verify: `/usr/bin/grep -Eq '^func TestScannerFindsBareAndNestedDecrypt\(' tools/envgate/scanner_test.go && go test ./tools/envgate -count=1`
- [~] Inventory literal DVA invocations in shell, script, Make, workflow, and documentation paths; report dynamic invocation as an explicit unresolved limitation rather than a green result | verify: `/usr/bin/grep -Eq '^func TestScannerReportsDynamicInvocationAsUnresolved\(' tools/envgate/scanner_test.go && go test ./tools/envgate -count=1`
- [~] Record Make/env.mk, DVA bridge, and direnv `use sops` ownership separately and reject an unverified or multiple-authority migration | verify: `/usr/bin/grep -Eq '^func TestGateRejectsUnverifiedOwnership\(' tools/envgate/gate_test.go && go test ./tools/envgate -count=1`
- [~] Evaluate corpus conflicts through an injected virtual reserved set that reuses the production conflict predicate without modifying `reservedCommands`; bind the base DVA and scanner digests and label this eligibility evidence rather than actual routing-candidate evidence | verify: `/usr/bin/grep -Eq '^func TestVirtualReservationMatchesProductionConflictPredicate\(' tools/envgate/gate_test.go && go test ./tools/envgate -count=1`
- [~] Enforce external SHA drift of zero between evidence and promotion review, and reject missing, stale, duplicate, partial, or ambiguous repository reports | verify: `/usr/bin/grep -Eq '^func TestGateRejectsStalePartialAndDriftedEvidence\(' tools/envgate/gate_test.go && go test ./tools/envgate -count=1`
- [~] Store the secret-free manifest and report bytes at a canonical tracked path; if an immutable external artifact is approved instead, track its URI, content digest, and retention policy. Retain evidence through the promotion decision and the full rollback-support period, and reject artifacts containing env values, decrypted output, credentials, or local absolute paths | verify: human — the reviewed evidence location, byte digest, retention deadline/event, and secret scan result must be recorded
- [~] Provide one documented command that reproduces the gate without executing command text sourced from the portfolio catalog | verify: `go run ./tools/envgate verify --manifest ./tools/envgate/testdata/complete/manifest.json && make doc-check`
- [~] Gate tests, repository tests, and generation checks pass | verify: `make test && make test-integration && make check-generate && make commit-check`

## Non-goals

- This task does not reserve `env`.
- It does not build or claim evidence for an actual routing candidate; a positive TASK-252 decision creates that implementation child.
- It does not mutate external repositories.
- It does not claim complete detection of dynamically assembled shell commands.
