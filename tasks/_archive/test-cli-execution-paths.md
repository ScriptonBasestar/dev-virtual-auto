---
id: BACKLOG-008
title: "cli 커맨드 실행 경로 테스트 확장"
type: tech-debt
priority: P3
effort: L
category: quality
tags: [test, coverage]
source: "BACKLOG-002 후속"
status: done
created: 2026-03-25
completed-at: 2026-03-25T09:58:13+09:00
archived-at: 2026-03-25T10:07:37+09:00
verified-at: 2026-03-25T10:07:37+09:00
verification-summary: "Test coverage metrics verified; internal/cli (58.1%) and internal/exec (96.5%) targets met. Evidence aligns with test execution outputs."
completion-summary: |
  internal/exec 패키지와 internal/cli 실행 경로에 대한 단위 테스트 추가.
  두 개의 새 테스트 파일 작성:
  - internal/exec/exec_test.go (24 tests): SplitCommand, buildCommandLine, ExecSubprocess, ExecSubprocessOutput, ExecReplace
  - internal/cli/execution_paths_test.go (30 tests): executeProvisionStep, executeParallelBatch, execComposeSubprocess, execComposePassthrough, runSubprojectCommand 확장
verification-status: passed
verification-evidence: |
  go test -count=1 ./internal/... 결과 (모두 PASS):
  - internal/cli: 53.4% → 58.1% (전체 패키지 커버리지)
  - internal/exec: 0% → 96.5%

  주요 함수별 커버리지 변화:
  | 함수                     | before | after   |
  |--------------------------|--------|---------|
  | execComposeSubprocess    | 0%     | 100%    |
  | execComposePassthrough   | 0%     | 100%    |
  | executeProvisionStep     | 0%     | 87.2%   |
  | executeParallelBatch     | 0%     | 95.4%   |
  | runSubprojectCommand     | 25%    | 79.2%   |
  | ExecSubprocess           | 0%     | 100%    |
  | ExecSubprocessOutput     | 0%     | 100%    |
  | buildCommandLine         | 0%     | 100%    |
  | SplitCommand             | 0%     | 100%    |
  | ExecReplace              | 0%     | 75%     |

  runProvisionCompose는 실제 docker compose 프로세스가 필요하여 0% 유지
  (인프라 의존성 있는 통합 테스트 범위로 분류)
---

## Description
`BACKLOG-002` (50% 커버리지 목표)는 달성되었으나 (54.6%), `run.go`의 `runSubprojectCommand` 실행 블록, `provision.go`의 `executeProvisionStep` 및 `runProvisionCompose` 실제 subprocess 호출 블록, `compose.go`의 `execComposeSubprocess` 등 명령어 실행(execution passthrough) 경로의 테스트 커버리지가 매우 낮습니다.
이를 보완하기 위한 Mock, Stub 혹은 추가적인 단위 테스트 작성이 필요합니다.

## Expected Value
실제 subprocess / syscall.Exec 등에 대한 파라미터 전달과 오류 반환 여부를 검증하여 명령어 실행 모듈의 신뢰성을 확보합니다.
