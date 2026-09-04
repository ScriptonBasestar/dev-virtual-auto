---
id: TASK-307
title: "plans: alias / extends to remove duplicate plan declarations"
type: feature
priority: P2
effort: M
exec-tier: strong
created-at: 2026-09-05T09:00:00+09:00
source: "reports/{cwrapper,dripter,scripton-nd-stack,gizzahub,matdosa}.md"
status: todo
needs-human: true
---

# Task 307: plan alias/extends 도입

## Summary

완전 중복 plan 쌍(infra≡local-infra, hybrid≡local-dev 등)이 5개 프로젝트에서 반복.
nd-stack에서는 서비스 목록이 4중 복제됨. 선언 중복을 제거할 alias 또는 extends가 필요.

## Decision required

alias(단순 별칭)만으로 충분한지, extends(부분 상속)까지 갈지 설계 결정 필요.
SOUL.md의 선언 단순성 원칙과 대조해 스펙 문서(docs/)부터 작성 후 구현.

## Completion Criteria

- [ ] 설계 문서 작성 및 승인 | verify: human
- [ ] 구현 + 순환 참조/미정의 참조 에러 테스트 | verify: `make test`
- [ ] nd-stack 설정을 alias로 재작성한 예시가 validate 통과 | verify: human — 출력 첨부
