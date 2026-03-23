---
id: BACKLOG-007
title: "통합 테스트 프레임워크 구축"
type: tech-debt
priority: P3
effort: XL
category: quality
tags: [test, integration, e2e]
source: "모든 테스트가 단위 테스트. fixture 기반 / e2e 테스트 없음"
---

## Description
dva.yml fixture 파일과 실제 docker compose 실행 기반의 통합 테스트 구축. CI에서도 실행 가능한 구조.

## Expected Value
실제 동작 검증. 모의 객체/실제 동작 간 불일치 방지.
