---
id: TASK-006
title: "Provision dry-run 플래그 추가"
type: feature
priority: P2
effort: S
parent: PLAN-001
created-at: 2026-03-24
completed-at: 2026-03-24
archived-at: 2026-03-24
verified-at: 2026-03-24
verification-summary: "Verified --dry-run flag added and plan printing via code review and git log."
---

## Summary
provision 커맨드에 `--dry-run` 플래그를 추가하여 실행 계획을 미리 확인 가능하도록 함.

## Rationale
- `run --explain/-e`로 dry-run 패턴 이미 존재
- provision은 여러 step을 순차 실행하므로 미리보기 중요
- 프로비저닝 실수 방지

## Completion Criteria
- [x] `dva provision PROFILE --dry-run` 실행 시 step 목록 출력 (실행 없음)
- [x] 각 step의 타입과 커맨드 표시
- [x] 기존 `--explain` 패턴과 일관된 출력 형식

## Resolution
`--dry-run`은 root.go:54의 global persistent flag로 구현됨.
provision.go에서 각 step 실행 전 `if dryRun` 체크로 plan만 출력.
