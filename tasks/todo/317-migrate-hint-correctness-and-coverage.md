---
id: TASK-317
title: "config migrate: wrong hints and unreported legacy fields"
type: bug
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-05T10:30:00+09:00
source: "docs/dogfood/{sadawiki,sigdock-idp,scripton-signalhub,familybook,primeno1,scripton-gitrump,scripton-db-orchestrator}.md"
status: todo
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

- [ ] 위 힌트 각각에 대한 픽스처 테스트로 정정 | verify: `make test`
- [ ] 누락 필드 각각이 migrate 출력에 등장 | verify: `make test`
