---
id: TASK-314
title: "logs/build <plan>: scope to plan services and buildable services"
type: bug
priority: P2
effort: S
exec-tier: standard
created-at: 2026-09-05T10:30:00+09:00
source: "docs/dogfood/{flow-knowchain,scripton-db-orchestrator}.md"
status: done
---

# Task 314: `dva logs/build <plan>` 서비스 범위 지정

## Repro

- `cd ~/mydevbox/flow-knowchain-devbox && dva --dry-run --debug logs docker-full` → `compose logs -f` (서비스 미지정). profile 뒤 서비스는 로그 미출력.
- `cd ~/mydevbox/scripton-db-orchestrator-devbox && dva --dry-run build hybrid` → `compose build postgres redis kafka prometheus …` (build 컨텍스트 없는 서비스 포함).

## Completion Criteria

- [x] logs가 plan entry services로 범위를 좁힘, build는 build 컨텍스트 있는 서비스만 전달(또는 인자 생략) | verify: `make test`

## Resolution (2026-09-05)

- 원인: `planComposeLogArgs`/`planComposeBuildArgs`가 passthrough가 하나라도 있으면 plan
  services 서브셋을 버렸다. `dva logs docker-full -f`의 `-f`가 서비스 목록을 지워 profile 뒤
  서비스가 빠졌다.
- 수정 (internal/cli/logs.go, build.go): `passthroughNamesServices`가 플래그(값을 먹는 플래그의
  값 포함)를 걸러 **서비스 이름이 있을 때만** 서브셋을 대체한다.
- build: `composeBuildableServices`가 엔트리 compose 파일(include 포함, compose_inspection.go
  `extractComposeBuildable`)에서 `build:` 유무를 읽어 이미지 전용 서비스를 인자에서 제외. 파일에
  없는 서비스는 그대로 통과(미선언 ≠ 이미지 전용), 파일을 못 읽으면 서브셋 그대로. 전부 이미지
  전용이면 `nothing to build`를 출력하고 compose build를 실행하지 않는다.
- USAGE.md plan 예시 아래에 범위 규칙 문단 추가.
- 테스트: logs/build arg 표 갱신(플래그 유지·값 플래그), `plan_build_scope_test.go`
  (buildable 필터, all-image, 파일 없음, include 파싱). 수정을 되돌리면 5개 서브테스트 실패 확인.
