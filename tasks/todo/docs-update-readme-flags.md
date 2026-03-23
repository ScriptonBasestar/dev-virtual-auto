---
id: TASK-004
title: "README 미문서화 플래그 업데이트"
type: docs
priority: P1
effort: XS
parent: PLAN-001
created-at: 2026-03-24
---

## Summary
구현되었으나 README에 누락된 11개 플래그와 2개 커맨드를 문서화.

## Rationale
- gap-analysis에서 발견된 문서/구현 불일치
- 사용자 발견성 직접 영향

## Completion Criteria
- [ ] init: --ai, --ai-docs, --no-ai-docs, -v, -t 추가
- [ ] validate: --fix 추가
- [ ] clean: -f (--force) 추가
- [ ] up: -f (--foreground), --force, --no-wait 추가
- [ ] status 커맨드 테이블에 추가
- [ ] config dump 커맨드 테이블에 추가
