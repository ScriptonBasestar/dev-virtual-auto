---
id: TASK-319
title: "native entries: optional dir, post-build steps, explicit primary compose entry"
type: feature
priority: P2
effort: M
exec-tier: strong
created-at: 2026-09-05T10:30:00+09:00
source: "docs/dogfood/{dripter,scripton-db-orchestrator,primeno1,funbricks-postkit}.md"
status: todo
needs-human: true
---

# Task 319: native 엔트리 표현력

1. **optional/dir-exists 조건 없음** — 미체크아웃 subproject dir이면 plan 전체 실패 (dripter). `optional: true` 제안.
2. **post-build step 없음** — `make build-api`는 build/api 복사까지 하지만 `dva build`는 cargo만 실행 (db-orchestrator).
3. **PrimaryComposeEntry 암묵 선택** — order 제거 후 이름순 최소 엔트리(lifecycle_helpers.go:164)가 provision compose_up과
   service 지정 interaction의 compose 파일 세트를 결정. 다중 compose 엔트리에 `primary: true` 같은 명시 수단 필요 (primeno1).
4. native runner `env:` 필드는 존재하나 문서화 부족으로 Makefile 경유가 선택됨 (postkit) — TASK-323 문서 범위와 연계.

## Completion Criteria

- [ ] 각 항목 설계 결정 기록 | verify: human
- [ ] 구현 + 테스트 | verify: `make test`
