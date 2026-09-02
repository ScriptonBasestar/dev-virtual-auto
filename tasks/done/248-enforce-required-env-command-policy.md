---
id: TASK-248
title: "Enforce the decided required env policy without breaking diagnostics"
type: feature
priority: P0
effort: L
exec-tier: standard
created-at: 2026-09-01T19:24:00+09:00
source: "PLAN-002 and TASK-247 current-loader safety decision"
scope: "loadEnv result model, all CLI callers, doctor hints, text/JSON fixtures, child-start guards"
status: done
depends-on: [TASK-247, TASK-264, TASK-265]
---

# Task 248: enforce required env behavior by command

## Summary

Implement TASK-247's caller matrix before adding encrypted-source mutation, while preserving optional-file
semantics and complete doctor diagnostics.

## Problem

Every caller must implement TASK-247's public behavior without duplicating inconsistent policy or
cutting off the command that diagnoses the missing file.

## Completion Criteria

- [x] Refactor environment loading so callers can distinguish required true/false, missing, inaccessible, malformed, and multi-file partial-merge state while successful precedence and caching remain unchanged | verify: `go test ./internal/config ./internal/cli -count=1`
- [x] Implement every TASK-247 matrix row with table-driven text/JSON/exit tests; root, imported canonical, and imported alias routes preserve their owner, and fail-closed rows stop before hooks, health checks, backend runners, or any external child | verify: `go test ./internal/cli -count=1`
- [x] Preserve complete doctor output in default and strict modes; refine existing env-file checks and source-aware hints rather than adding a duplicate check | verify: `go test ./internal/cli -count=1`
- [x] Keep stdout to one JSON document, use the existing root error envelope where the decision calls for failure, and keep human diagnostics off JSON stdout | verify: `go test ./internal/cli -count=1`
- [x] Optional missing files remain skipped, optional existing unreadable/malformed files remain explicit errors, no execution continues on accidental partial merge, and no command invents an unseal hint without recognized source metadata from a later approved bridge | verify: `go test ./internal/config ./internal/cli -count=1`
- [x] Usage and migration documentation name the behavior of observation, execution, teardown, and doctor commands | verify: `make doc-check`
- [x] Repository gates pass | verify: `make lint && make test && make test-integration && make commit-check`

## Completion Summary

`loadEnv`가 `(*config.Environment, *config.EnvInputReport)`를 반환하고, `internal/config/envinput.go`가
inspect-then-apply 2단계 모델을 소유합니다. 실패 시 어떤 선언에서 온 값도 적용되지 않으며(atomicity),
이는 `MergeVars` 이후에는 되돌릴 수 없기 때문에 적용 전에 판정을 끝내는 구조로만 표현 가능합니다.

Route별 구현: 실행/정리는 첫 자식 이전에 `report.Err()`, 관찰은 partial document 후 exit 1,
doctor는 `Environment input loads: <path>` row와 compose probe skip row, validate는 env I/O 없음.
소유권은 `planRuntime`/`commandRuntime`이 각자 report를 들고 다니며, root report는 `detectPlanRoute`
*이전에* 만들되 root 경로에서만 검사해 imported 경로를 막지 않습니다. Hook wrapper는 정책을 복제하지
않고 incomplete일 때 built-in에 위임합니다 — before-hook은 안 돌고, route 정책은 한 곳에만 있습니다.

검증: `internal/config/envinput_test.go`(상태 table, atomicity, 경로 표기, 성공 precedence),
`internal/cli/env_input_policy_test.go`(marker 기반 fail-closed, hook 미실행, status/logs partial
text·JSON·exit, doctor default 0 / strict 1, owner isolation 양방향, validate env I/O 없음,
secret sentinel 부재). Gates: `make lint` 0 issues, `make test`, `make test-integration`,
`make doc-check` broken_links 0, `make commit-check` 통과.

### 계약과 다르게 구현한 두 지점

1. **`logs` partial 문구의 첫 글자**: TASK-247 §5는 `Logs not queried for ...`로 고정했지만, 이
   문자열은 error 값으로 전달되어야 JSON envelope의 `message`가 되고(계약이 요구하는 바), Go의
   error 문자열은 대문자로 시작할 수 없습니다 — `make lint`가 ST1005로 실제 차단했습니다. 소문자
   `logs not queried for ...`로 구현했으며, 렌더링 결과는 `ERROR: logs not queried for ...`입니다.
   Lint 규칙을 끄는 대신 한 글자를 양보했습니다.
2. **TASK-247 criterion 1의 verify binding**: `loadEnv(` 호출 표현식 18개를 세는 명령은 이제 10을
   반환합니다. 8개 site가 report를 함께 쓰기 위해 `rootEnvLoad(`/`newOwnedConfigEnvironment(`로
   바뀌었기 때문입니다. 세 이름을 합치면 21개(정의부와 `loadEnv` 내부 위임 제외)입니다. 닫힌 카드의
   binding은 동결 시점 inventory의 기록이라 수정하지 않았고, 후속 invariant는 이 21개입니다.

### 문서

`USAGE.md`에 [환경 입력이 불완전할 때](../../USAGE.md) 절(route 표, partial JSON, 소유자 규칙,
마이그레이션 주의)을 추가하고 doctor 빌트인 체크 목록을 갱신했습니다. `CHANGELOG.md`에 동작 변경과
마이그레이션 영향을 기록했습니다. `docs/42-migration-and-compatibility.md`는 10226/10240 byte로
doc-check 상한 직전이라 한 줄도 추가할 수 없어 사용하지 않았습니다.

## Non-goals

- No interaction or subcommand `env_file` support, deprecation warning or schema change. TASK-265 decided
  versioned rejection and [TASK-266](266-deprecate-and-reject-interaction-env-file.md) owns it; this task
  only keeps the field inert exactly as TASK-247 §4 froze it.
- No implicit unseal.
- No config env command, encrypted-source schema, or sops invocation.
- No change to optional env-file absence.
- No promotion of doctor default mode into a release gate.
