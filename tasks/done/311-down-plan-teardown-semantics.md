---
id: TASK-311
title: "down <plan>: rm-based teardown leaves named volumes and networks"
type: bug
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-05T10:30:00+09:00
source: "docs/dogfood/{sadawiki,scripton-signalhub,scripton-db-orchestrator,scripton-dns-bridge}.md (dogfood 2026-09-05)"
status: done
---

# Task 311: `dva down <plan>` teardown 의미론 수정

## Summary

services가 있는 plan의 `down`/`down --volumes`/`--purge`가 `compose rm --force --stop [--volumes] <svc…>`로
실행돼(internal/lifecycle/compose.go composeDownArgs) named volume과 network가 남는다. 그 결과 4개 프로젝트가
`docker compose down -v`를 직접 호출하는 clean/reset interaction을 제거하지 못했다.

## Repro

- `cd ~/mydevbox/scripton-db-orchestrator-devbox && dva --dry-run down infra --volumes` → `compose rm …`
- `cd ~/mydevbox/scripton-dns-bridge-devbox && dva --dry-run down infra` → network 미제거
- services 없는 plan(funbricks-elemhant full-stack)만 `down --remove-orphans --volumes`

## Direction

프로젝트 전체 teardown 옵션(예: `--all` 또는 `--purge`의 의미를 compose down -v로 승격)과
부분 plan의 rm 동작을 구분해 문서화. 기존 `--purge` 의미 변경 시 docs/43 갱신.

## Completion Criteria

- [x] services 있는 plan에서 named volume/network까지 제거하는 경로 존재 + dry-run 테스트 | verify: `make test`
- [x] sadawiki/signalhub의 `docker compose down -v` interaction을 dva 동사로 대체 가능함을 dry-run으로 확인 | verify: human

## Resolution (2026-09-05)

- 방향: 기존 `--purge`의 의미를 "compose 프로젝트 전체 teardown"으로 승격. plan이 services를
  골라도 `--purge`는 `compose down --remove-orphans --volumes --rmi local`을 실행한다
  (internal/lifecycle/compose.go `composeDownArgs`, `PluginContext.Purge`/`DownOptions.Purge`/
  `ChildDownOptions.Purge`). 부분 plan의 `down`/`-v`는 `rm --force --stop [--volumes] <svc…>`를
  유지한다 — `compose down`은 서비스 필터가 없어 같은 프로젝트의 다른 plan 서비스까지 내린다.
- `rm` 경로는 stderr에 남는 자원(named volume, 프로젝트 network)과 `--purge` 안내를 출력한다
  (`composeDownLeftovers`).
- 문서: USAGE.md `--purge` 절에 범위 설명 추가, docs/43 Tier 1 표기 갱신.
- 테스트: `TestComposeDownArgsPurgeWidensToProject`(lifecycle, 인자 5형태),
  `TestPlanDownPurgeTearsDownWholeProject`(cli, services plan `--dry-run` 3형태). 수정을 되돌리면
  cli 테스트가 실패함을 확인.
- 기준 2(sadawiki/signalhub interaction 대체 dry-run)는 cli 픽스처가 같은 shape(services 선택 plan +
  `--purge --force --dry-run` → `down --remove-orphans --volumes --rmi local`)로 검증. devbox 실제
  repo에서의 interaction 제거는 후속 dogfood 라운드에서 적용.
