---
id: PLAN-001
title: "DVA Feature Discovery Plan"
type: plan
scope: "Feature discovery, gap analysis, and task emission for DVA v0.1.12"
parent: null
children:
  - TASK-001
  - TASK-002
  - TASK-003
  - TASK-004
  - TASK-005
  - TASK-006
  - TASK-007
  - BACKLOG-001
  - BACKLOG-002
  - BACKLOG-003
  - BACKLOG-004
  - BACKLOG-005
  - BACKLOG-006
  - BACKLOG-007
  - BACKLOG-008
progress: 93
total-tasks: 15
completed-tasks: 14
target-date: null
---

## Goal
DVA v0.1.12 프로젝트의 기능 발굴, 요구사항 갭 분석, 태스크 생성을 통해 다음 개발 방향을 구조화.

## Analysis Sources
- `reports/feature/discovery.md` — 14개 기능 기회 발견
- `reports/feature/ideas.md` — 아이디어 정리 및 스펙 작성
- `reports/feature/gap-analysis.md` — README vs 구현 갭 4건

## Children (Priority Order)

### P1 — 즉시 실행 (이미 구현된 코드 연결 또는 소규모 작업)
- [x] TASK-001 — Shell completion 커맨드 구현 (S)
- [x] TASK-002 — env_file 로딩 파이프라인 연결 (XS)
- [x] TASK-003 — Tag 필터링 시스템 활성화 (XS)
- [x] TASK-004 — README 미문서화 플래그 업데이트 (XS)

### P2 — 다음 마일스톤
- [x] TASK-005 — dva migrate 커맨드 구현 (M)
- [x] TASK-006 — Provision dry-run 플래그 추가 (S)
- [x] TASK-007 — DVA_CURRENT_USER UID→username 수정 (XS)

### P3 — 백로그
- [x] BACKLOG-001 — devcontainer 지원 구현 (XL)
- [x] BACKLOG-002 — 핵심 커맨드 테스트 커버리지 확대 (XL) → 54.6% 달성 (2026-03-25)
- [x] BACKLOG-003 — 에러 처리 패턴 통일 (M)
- [ ] BACKLOG-004 — Config 스키마 마이그레이션 도구 (L)
- [x] BACKLOG-005 — CHANGELOG.md 도입 (S)
- [x] BACKLOG-006 — Provision 병렬 실행 지원 (L)
- [x] BACKLOG-007 — 통합 테스트 프레임워크 구축 (XL)
- [x] BACKLOG-008 — cli 커맨드 실행 경로 테스트 확장 (L)

## Recommended Execution Order
1. TASK-002, TASK-003 (XS, 이미 구현된 코드 연결)
2. TASK-004 (XS, 문서 업데이트)
3. TASK-001 (S, Cobra 내장 기능 활용)
4. TASK-007 (XS, 단순 수정)
5. TASK-006 (S, 기존 패턴 재사용)
6. TASK-005 (M, 새 커맨드 설계 필요)
