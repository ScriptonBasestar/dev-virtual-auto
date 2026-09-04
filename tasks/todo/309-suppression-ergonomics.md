---
id: TASK-309
title: "suppression ergonomics: suggestion_ignore compaction and drift-warning ignore"
type: feature
priority: P3
effort: M
exec-tier: standard
created-at: 2026-09-05T09:00:00+09:00
source: "reports/{dripter,flow-taskchain,flow-knowchain}.md"
status: todo
needs-human: true
---

# Task 309: suggestion_ignore 축약 및 drift warning ignore 수단

## Summary

- suggestion_ignore가 70~120 항목으로 비대해져 Makefile 이중 관리 비용 발생
  (dripter 110+, flow-taskchain 120).
- 의도적인 test/prod override compose 파일이 상주 drift warning을 유발하는데
  이를 ignore할 수단이 없음 (flow-knowchain).

## Decision required

패턴/glob 기반 축약 vs 카테고리 단위 opt-out vs suggestion 소스 자체 개선 중 방향 결정.
ignore 수단은 침묵 남용으로 이어질 수 있어 설계 검토 필요.

## Completion Criteria

- [ ] 설계 결정 기록 | verify: human
- [ ] 구현 + 테스트 | verify: `make test`
- [ ] dripter/flow-taskchain의 ignore 목록이 유의미하게 축약된 예시 | verify: human

## Dogfood evidence (2026-09-05 실행)

- 의도적 미등록 overlay(test/prod/e2e 픽스처)로 상주 drift warning: flow-knowchain(3), sigdock-idp, familybook(5), db-orchestrator(2). `drift_ignore` 류 선언 필요.
- suggestion_ignore 규모: gizzahub 184, familybook 194, db-orchestrator ~85(glob), flow-agent-mesh 8패턴으로 정리 사례. 감지 결함은 TASK-316.

## Design record (2026-09-05)

- 문서: `docs/56-suppression-ergonomics-design.md`. 설계 선택지 문서 작성 완료, 결정 대기. 권고: suggestion은 소스 개선(이미 interaction이 감싸는 타겟 제외)+카테고리 opt-out, drift는 루트 자동탐지 규칙에만 적용되는 `drift_ignore` glob, 억제 건수를 validate 요약에 항상 표시.
