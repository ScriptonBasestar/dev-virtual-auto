---
id: TASK-310
title: "docs: devbox native-lifecycle pattern guide"
type: docs
priority: P3
effort: S
exec-tier: standard
created-at: 2026-09-05T09:00:00+09:00
source: "reports/{flow-knowchain,flow-pipechain,flow-observechain,dripter}.md"
status: todo
---

# Task 310: devbox native-lifecycle 패턴 가이드 작성

## Summary

native 앱 프로세스 선언에 두 유파가 갈림:

- "루트 plan에 native entry" (flow-knowchain, flow-pipechain)
- "subproject 소유 + import.interactions" (flow-observechain — 레퍼런스, warning 0)

dripter는 native 앱을 stack에 아예 선언하지 않아 dva 관리 밖. 권장 패턴 문서화 필요.

## Direction

flow-observechain 방식을 권장 패턴으로 docs/에 가이드 작성, examples/에 예시 추가.
interaction.start로 native 실행을 중복 선언하는 안티패턴(cwrapper)도 함께 다룬다.

## Completion Criteria

- [ ] docs/ 가이드 + examples/ 예시 추가, `make generate` 산출물 갱신 | verify: `make generate && git diff --stat`
- [ ] 예시가 validate 통과 | verify: human — 출력 첨부
