---
id: TASK-002
title: "env_file 로딩 파이프라인 연결"
type: chore
priority: P1
effort: XS
parent: PLAN-001
created-at: 2026-03-24
completed-at: 2026-03-24
archived-at: 2026-03-24
verified-at: 2026-03-24
verification-summary: "LoadEnvFile called via root.go loadEnv() on all commands. Added 7 envfile_test.go tests covering string/slice/map configs, optional vs required missing files, OS priority, and quoted values. Fixed LoadEnvFile to use MergeVars (env_file < OS env priority)."
---

## Summary
이미 구현된 `envfile.LoadEnvFile()`을 `config.Load()` 체인에 연결하여 dva.yml의 `env_file` 필드가 실제로 동작하도록 함.

## Rationale
- `internal/config/envfile.go` (90줄) 완전 구현
- `config.Load()`에서 호출하지 않아 기능 비활성 상태
- README line 100에 ".env 파일 로딩 지원" 명시

## Completion Criteria
- [x] `config.Load()` 내에서 `LoadEnvFile()` 호출
- [x] 로드된 변수가 `Environment`에 올바른 우선순위로 병합
- [x] 우선순위: env_file < config environment < OS env
- [x] 기존 테스트 전체 통과
- [x] env_file 로딩 테스트 추가

## Resolution
`LoadEnvFile()`은 `root.go` `loadEnv()` 함수에서 모든 커맨드 공통으로 호출됨.
`LoadEnvFile` 내부에서 직접 `env.Vars[k]=v` 하던 것을 `env.MergeVars(vars)`로 수정하여
OS 환경변수 우선순위(env_file < OS env)가 올바르게 적용됨.
7개 테스트 추가: `internal/config/envfile_test.go`
