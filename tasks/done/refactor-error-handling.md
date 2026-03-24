---
id: BACKLOG-003
title: "에러 처리 패턴 통일"
type: improvement
priority: P3
effort: M
category: quality
tags: [refactor, error-handling]
source: "run.go:67 os.Exit vs compose.go:119 return err 혼재"
---

## Description
CLI 전반에 os.Exit(1) 직접 호출과 return err가 혼재. cobra RunE 패턴으로 통일하고 중앙 에러 핸들링 적용.

## Expected Value
일관된 에러 메시지. 테스트 용이성 향상. 에러 래핑으로 디버깅 개선.
