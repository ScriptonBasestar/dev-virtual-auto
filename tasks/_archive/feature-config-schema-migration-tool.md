---
id: BACKLOG-004
title: "Config 스키마 마이그레이션 도구 (Auto-fix 지원)"
type: feature
priority: P3
effort: L
parent: PLAN-001
created: 2026-03-25
completed-at: 2026-03-25T11:25:00+09:00
archived-at: 2026-03-25T11:25:00+09:00
verified-at: 2026-03-25T11:25:00+09:00
verification-summary: "Implement --fix flag in dva migrate to rewrite old schema keys using yaml.Node AST. Checked via unit tests."
---

## Summary
원래 TASK-005(dva migrate)에서 가이드만 출력하던 마이그레이션 기능을 확장하여, `dva migrate --fix`를 통해 자동으로 dva.yml을 업데이트 해주는 도구 도입 구축 완료. 추가로 .hip.yml 관련 레거시 감지 로직 및 코드는 모두 제거하여 혼란을 방지함.

## Requirements
- [x] dva migrate --fix 명령어 도입
- [x] yaml.Node를 활용해 주석과 포맷을 유지하며 키 변경
- [x] .hip.yml 등 불필요한 레거시 호환 및 문서 제외 처리
