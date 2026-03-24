---
id: BACKLOG-001
title: "devcontainer 지원 구현"
type: feature
priority: P3
effort: XL
category: integration
tags: [vscode, devcontainer]
source: "config.go:24 — 필드 정의만 존재"
verified-at: "2026-03-24T16:17:17+09:00"
archived-at: "2026-03-24T16:17:17+09:00"
verification-summary: "devcontainer 명령어 및 테스트 구현 완료, config 스키마 반영 및 make test 정상 통과 확인."
---

## Description
dva.yml 기반으로 VS Code devcontainer.json 자동 생성/관리 기능. config struct에 devcontainer 필드가 이미 정의되어 있으나 구현 전무.

## Expected Value
VS Code Remote Container 워크플로우와 DVA 통합. 개발환경 일관성 향상.
