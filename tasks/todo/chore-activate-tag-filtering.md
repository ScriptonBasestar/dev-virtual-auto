---
id: TASK-003
title: "Tag 필터링 시스템 활성화"
type: chore
priority: P1
effort: XS
parent: PLAN-001
created-at: 2026-03-24
---

## Summary
이미 구현된 태그 필터링 함수들을 `run.go`와 `compose.go`에서 실제 호출하여 서브프로젝트의 `exclude_tags` 기능 활성화.

## Rationale
- `subproject.go:48-117` — HasTag, FilterInteractions, GetComposeServicesExcluding 완전 구현
- 어떤 CLI 커맨드에서도 호출하지 않음
- 서브프로젝트 기능의 핵심 요소

## Completion Criteria
- [ ] `run.go`에서 서브프로젝트 실행 시 `FilterInteractions()` 적용
- [ ] `compose.go` up/down에서 `GetComposeServicesExcluding()` 적용
- [ ] 태그 미설정 시 기존 동작 변경 없음
- [ ] 태그 필터링 동작 테스트 추가
