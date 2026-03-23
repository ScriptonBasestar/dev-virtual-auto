---
id: TASK-001
title: "Shell completion (bash/zsh/fish) 커맨드 구현"
type: feature
priority: P1
effort: S
parent: PLAN-001
created-at: 2026-03-24
---

## Summary
README에 명시된 `dva completion bash/zsh/fish` 커맨드 구현. Cobra 내장 completion 기능 활용하되, 동적 interaction 커맨드도 자동완성 대상에 포함.

## Rationale
- README line 81에 나열되었으나 구현 없음
- `reserved.go`에 예약어 등록 완료
- Cobra가 이미 completion 인프라 제공 → 연결만 필요
- CLI DX 대폭 향상

## Completion Criteria
- [ ] `dva completion bash` 출력 → bash completion 스크립트
- [ ] `dva completion zsh` 출력 → zsh completion 스크립트
- [ ] `dva completion fish` 출력 → fish completion 스크립트
- [ ] 동적 interaction 커맨드가 ValidArgsFunction으로 완성됨
- [ ] rootCmd에 completion 서브커맨드 등록
