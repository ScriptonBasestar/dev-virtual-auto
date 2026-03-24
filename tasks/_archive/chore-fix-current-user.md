---
id: TASK-007
title: "DVA_CURRENT_USER UID→username 수정"
type: chore
priority: P2
effort: XS
parent: PLAN-001
created-at: 2026-03-24
completed-at: 2026-03-24
archived-at: 2026-03-24
verified-at: 2026-03-24
verification-summary: "Verified DVA_CURRENT_USER logic and added tests for DVA_CURRENT_UID."
---

## Summary
`DVA_CURRENT_USER` 특수 변수가 UID(숫자) 대신 username(문자열)을 반환하도록 수정.

## Rationale
- `environment.go:41` — `u.Uid` 사용 중 (숫자 ID)
- "CURRENT_USER"라는 이름에 username이 더 직관적
- 기존 UID 의존 설정을 위해 `DVA_CURRENT_UID` 추가 고려

## Completion Criteria
- [x] `DVA_CURRENT_USER` → `u.Username` 반환
- [x] `DVA_CURRENT_UID` 추가 (하위호환)
- [x] environment_test.go 업데이트
