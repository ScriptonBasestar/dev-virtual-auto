---
id: TASK-315
title: "compose profiles: stack runner option and PlanEntry profiles"
type: feature
priority: P2
effort: M
exec-tier: strong
created-at: 2026-09-05T10:30:00+09:00
source: "reports/{flow-knowchain,scripton-dns-bridge,sigdock-idp,sadawiki}.md"
status: todo
---

# Task 315: compose profile 선택 수단

## Summary

stack compose 러너에 `profiles:` 옵션이 없고(schema.json ~L421은 interaction compose 옵션만) PlanEntry에도 `profiles`가 없어
profile 게이트 서비스는 services에 이름을 나열해야만 활성화된다. dns-bridge는 profile-only mode 4개를 서비스 명시로 우회했다.

## Completion Criteria

- [ ] PlanEntry `profiles:` (또는 stack runner 기본 profiles) 추가 + `--profile` 전달 테스트 | verify: `make test`
- [ ] docs/40 갱신, dns-bridge plan을 profiles로 재표현한 예시 validate 통과 | verify: human
