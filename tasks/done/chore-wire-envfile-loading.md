---
id: TASK-002
title: "env_file 로딩 파이프라인 연결"
type: chore
priority: P1
effort: XS
parent: PLAN-001
created-at: 2026-03-24
---

## Summary
이미 구현된 `envfile.LoadEnvFile()`을 `config.Load()` 체인에 연결하여 dva.yml의 `env_file` 필드가 실제로 동작하도록 함.

## Rationale
- `internal/config/envfile.go` (90줄) 완전 구현
- `config.Load()`에서 호출하지 않아 기능 비활성 상태
- README line 100에 ".env 파일 로딩 지원" 명시

## Completion Criteria
- [ ] `config.Load()` 내에서 `LoadEnvFile()` 호출
- [ ] 로드된 변수가 `Environment`에 올바른 우선순위로 병합
- [ ] 우선순위: env_file < config environment < OS env
- [ ] 기존 테스트 전체 통과
- [ ] env_file 로딩 테스트 추가
