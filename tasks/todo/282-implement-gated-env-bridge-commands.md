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
status: todo
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
- 복호값은 `show`의 stdout 외 어디에도 나타나지 않는다 — log, error, JSON, temp filename 포함.

## Implementation notes

- 게이트 검사는 `preflight` 1단계(platform)보다 **앞**에 온다. 꺼진 명령은 config 상태나 OS와
  무관하게 항상 같은 code를 낸다.
- `seal`의 source write는 `unseal`의 target write와 같은 안전 쓰기 경로를 재사용한다
  (same-directory 0600 O_EXCL temp → 검증 → rename → parent fsync). 두 번째 writer 구현을
  만들지 않는다.
- `bridgeGOOS` 주입 패턴을 따라, 게이트가 꺼진 분기와 platform 분기 모두 CI가 실제로 실행하는
  테스트를 갖는다. 실행되지 않는 fail-closed 분기는 아무도 확인하지 않은 분기다.
- key-set 비교는 이름 집합만 다룬다. 값은 비교 대상도, 로그 대상도, 오류 메시지 대상도 아니다.

## Completion Criteria

- [ ] Add the `env_bridge` section to the config struct and `schema.json` with both switches defaulting to false, rejecting the declaration locations TASK-281 forbids | verify: `make test`
- [ ] Prove a config without `env_bridge` is byte-identical through load, merge, `config show`, and `validate` against the pre-change binary | verify: `make test`
- [ ] Implement the gate's origin and merge rule, including a test that a subproject cannot enable the parent's gate | verify: `make test`
- [ ] Implement `seal` with no key or provider arguments, failing closed when `.sops.yaml` declares no creation rule for the source | verify: `make test`
- [ ] Implement the frozen lost-update defense and cover every row of the TASK-281 `seal` matrix, asserting the existing source is byte-identical after each failure | verify: `make test`
- [ ] Implement `show` and assert no decrypted value reaches debug log, stderr, error envelope, JSON, or any temp filename in any failure path | verify: `make test`
- [ ] Implement disabled-state rejection for both commands ahead of every other preflight step, with the frozen codes and exit 1 | verify: `make test`
- [ ] Cover the real-sops path for both commands with the pinned sops/age integration job that already exists for `unseal` | verify: `make test-integration`
- [ ] Assert `edit` and `unseal` outputs, codes, and exits are unchanged by this card | verify: `make test`
- [ ] Document both commands and the gate in USAGE.md under the existing `config env` section and add a CHANGELOG entry naming the default-off posture | verify: `make doc-check`
- [ ] Pass the repository's full mechanical gate before integration | verify: `make lint && make test && make doc-check && make check-generate`
