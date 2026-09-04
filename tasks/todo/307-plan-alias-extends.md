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

## Dogfood evidence (2026-09-05 실행)

- overlay 엔트리 재사용 부재: base+overlay 엔트리를 plan에서 조합하면 TASK-288 경고(validate_warnings.go:893)로 거부되어
  overlay마다 base 서비스/태그를 재선언해야 함 (primeno1 observability/tracing). 후보: 엔트리 `extends:` 또는 PlanEntry `overlays:`.
- `plans.<name>.composes`(TASK-260)는 composition plan이 자체 entries를 못 가져 "infra + 추가 서비스"에 못 씀 (dns-bridge: postgres/redis가 8개 plan에 반복).
- plan 경로에서 `--env` 거부 → "같은 plan, 다른 env" 표현 수단 없음 (matdosa).

## Design record (2026-09-05)

- 문서: `docs/55-plan-alias-extends-design.md`. 설계 문서 작성 완료, 승인 대기. 권고: alias만 구현하고 YAML anchor/merge key를 부분 복제 해법으로 문서화, extends는 보류. §5의 결정 항목 4개에 답하면 구현 착수.
