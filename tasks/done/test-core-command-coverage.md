---
id: BACKLOG-002
title: "핵심 커맨드 테스트 커버리지 확대 (15%→50%)"
type: tech-debt
priority: P3
effort: XL
category: quality
tags: [test, coverage]
source: "테스트 없는 파일: run.go, compose.go, status.go, manifest.go, provision.go 등"
---

## Description
현재 테스트 커버리지 ~15%. run, up/down, validate 등 핵심 커맨드에 단위 테스트 추가하여 50% 목표.

## Expected Value
리팩토링 안전망 확보. 회귀 버그 방지.
