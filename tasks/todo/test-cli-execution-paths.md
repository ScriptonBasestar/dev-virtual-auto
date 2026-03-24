---
id: BACKLOG-008
title: "cli 커맨드 실행 경로 테스트 확장"
type: tech-debt
priority: P3
effort: L
category: quality
tags: [test, coverage]
source: "BACKLOG-002 후속"
status: todo
created: 2026-03-25
---

## Description
`BACKLOG-002` (50% 커버리지 목표)는 달성되었으나 (54.6%), `run.go`의 `runSubprojectCommand` 실행 블록, `provision.go`의 `executeProvisionStep` 및 `runProvisionCompose` 실제 subprocess 호출 블록, `compose.go`의 `execComposeSubprocess` 등 명령어 실행(execution passthrough) 경로의 테스트 커버리지가 매우 낮습니다.
이를 보완하기 위한 Mock, Stub 혹은 추가적인 단위 테스트 작성이 필요합니다.

## Expected Value
실제 subprocess / syscall.Exec 등에 대한 파라미터 전달과 오류 반환 여부를 검증하여 명령어 실행 모듈의 신뢰성을 확보합니다.
