---
id: TODO-001
title: "AI 기반 DVA 자동 설정 워크플로우(setup-dva) 구축"
type: feature
priority: P1
effort: M
category: ai-agent
tags: [workflow, orchestration, setup]
source: "ai-docs/workflow/setup-dva/auto.md"
archived-at: "2026-03-24T17:31:15+09:00"
verified-at: "2026-03-24T17:31:15+09:00"
verification-summary: "Verified 00-40 stages created and flag handlers tested internally."
---

## Description
AI 서브에이전트를 활용하여 DVA 설정 및 구동 파이프라인을 자동화하는 기반을 구성합니다. 현재 오케스트레이터 역할을 하는 `auto.md`가 추가되었으며, 각 단계를 위임받을 실제 Stage 명세 파일들의 구현과 테스트가 필요합니다. 또한 Makefile에 빌드 플래그(Version, Commit 등)를 보강하여 설치 시 버전 정보를 정확히 표시하도록 조치한 상태입니다.

### 진행해야 할 세부 작업 (Sub-tasks)
1. **Subagent 스테이지 파일 작성**
   - [x] `stages/00-analyze.md`
   - [x] `stages/10-verify.md` (사용자 승인 분기 포함)
   - [x] `stages/20-transform.md`
   - [x] `stages/30-configure-full.md`
   - [x] `stages/30-configure-adopt.md`
   - [x] `stages/40-execute.md`
2. **파이프라인 상태/플래그 동작 검증**
   - [x] 캐시 기반 `--resume` 및 `--dry-run` 동작 확인
   - [x] DVA CLI 부재 시 `docker compose up -d` Fallback 작동 확인

## Expected Value
기존에 컨테이너 환경을 수동 구성하던 사용자들도 AI 분석을 통해 손쉽고 정확하게 DVA 연동 및 구조 마이그레이션을 자동 수행할 수 있어 DVA 도입 허들을 크게 낮춥니다.
