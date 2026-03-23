---
id: TASK-005
title: "dva migrate 커맨드 구현"
type: feature
priority: P2
effort: M
parent: PLAN-001
created-at: 2026-03-24
---

## Summary
Hip CLI 설정 또는 구버전 dva.yml에서 현재 포맷으로의 마이그레이션 가이드 생성 커맨드 구현.

## Rationale
- README line 79에 "Generate migration guide" 명시
- `reserved.go`에 예약어 등록 완료
- Hip CLI(Go 재작성 전 버전)에서 마이그레이션하는 사용자 지원

## Completion Criteria
- [ ] `dva migrate` 실행 시 구 포맷 감지
- [ ] 차이점 리포트 출력
- [ ] 변환 가이드 또는 자동 변환 옵션 제공
- [ ] rootCmd에 migrate 서브커맨드 등록
