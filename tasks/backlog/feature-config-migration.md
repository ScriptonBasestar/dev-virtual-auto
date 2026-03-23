---
id: BACKLOG-004
title: "Config 스키마 마이그레이션 도구"
type: feature
priority: P3
effort: L
category: dx
tags: [config, migration]
source: "config.go:15 version 필드 존재, 버전 간 변환 로직 없음"
---

## Description
dva.yml 스키마 버전 변경 시 자동 변환 도구. `dva migrate` 커맨드와 연계하여 config 업그레이드 경로 제공.

## Expected Value
Breaking change 시 사용자 마이그레이션 부담 감소.
