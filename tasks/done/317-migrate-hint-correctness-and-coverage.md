---
id: TASK-317
title: "config migrate: wrong hints and unreported legacy fields"
type: bug
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-05T10:30:00+09:00
source: "docs/dogfood/{sadawiki,sigdock-idp,scripton-signalhub,familybook,primeno1,scripton-gitrump,scripton-db-orchestrator}.md"
status: done
---

# Task 317: migrate 힌트 오류 및 누락 항목

TASK-306(스캐폴드 출력)과 별개로, 현재 힌트 자체가 틀리거나 legacy 필드를 침묵으로 통과시킨다.

## Wrong hints

- `compose_profiles: [dev-tools]` → "`--profile full`", `[mq]` → "`--profile infra-mq`": profile 값 대신 mode 이름 출력 (sadawiki, sigdock-idp).
- "environment → environments.hybrid.vars" 안내인데 유효 키는 `environments.<name>.environment` (signalhub).
- `tags`를 엔트리와 runners.compose 양쪽에 복제 (migrate.go:161) — drift 씨앗 (gitrump).
- `applications.*.port`를 버림 — endpoints 후보로 제안해야 손실 방지 (db-orchestrator).
- `applications.*.dev`를 "별도 엔트리 선언"으로만 안내, 동일 명령의 기존 interaction 중복 미언급 (gitrump).

## Unreported legacy fields

`environments.*.compose_files`, env_file map형 잉여 키(priority/interpolate), 최상위 health_checks의 start/start_hint,
최상위 `environment:`, `modes.*.provision`(대체물 없음 — 명시 안내 필요), interaction/script 속 `dva up -M` 잔재.

## Completion Criteria

- [x] 위 힌트 각각에 대한 픽스처 테스트로 정정 | verify: `make test`
- [x] 누락 필드 각각이 migrate 출력에 등장 | verify: `make test`

## Resolution (2026-09-05)

- 힌트 정정 (internal/config/migrate_report.go): `compose_profiles`는 mode 이름 대신 실제 값으로
  `--profile dev-tools --profile debug`를 렌더링. `environment`는 `environments.<name>.environment`
  로. `provision`은 `dva provision <profile>`을 `dva up <plan>` 앞에 실행하거나 interaction steps로
  묶으라는 명시 안내.
- `tags` 복제는 의도된 동작(엔트리 tags = `--tags` 필터, runners.compose.tags = 서비스 필터 기본값,
  `Config.ComposeTags()`)이라 유지하되, `noteDuplicatedTags`가 Converted 목록에 이유를 적는다.
- applications (migrate_applications.go): `port`는 `endpoints.<name>: {url: "http://localhost:<port>"}`
  후보를 제안. `dev`는 같은 명령을 `command:`로 가진 interaction이 있으면 그 이름을 지목.
- 누락 legacy 필드 (신규 migrate_legacy_fields.go, `ReportLegacyFields` → Left for you):
  `environments.*.compose_files`, `env_file.priority/interpolate`, 최상위 `health_checks.*.start/start_hint`,
  interaction command/script/steps/subcommands 및 provision 항목 속 `dva … -M|--mode` 잔재.
- 최상위 `environment:`는 보고하지 않음 — `newConfigEnvironmentAt`(cli/root.go)과 `loadEnv`가 plan
  경로에서도 읽는 live 키라 legacy가 아님. 카드의 열거는 dogfood 시점 오해였다.
- validate 쪽 동일 오류(validate_warnings.go "use 'vars' only")도 `environment:`로 정정.
- 테스트: internal/config/migrate_hints_test.go — 힌트별 픽스처 8건, 누락 필드 11항목 전부 출력 확인.
  `make test`, `make lint`, `make doc-check`, `make generate`(diff 없음) 통과.
