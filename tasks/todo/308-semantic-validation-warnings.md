---
id: TASK-308
title: "validate: plan→stack reference check and semantic warnings"
type: feature
priority: P2
effort: M
exec-tier: strong
created-at: 2026-09-05T09:00:00+09:00
source: "reports/{gizzahub,matdosa,funbricks-postkit,funbricks-notifire,flow-knowchain,gorisa,sigdock-pass}.md"
status: todo
---

# Task 308: 참조 검증 및 semantic warning 추가

## Summary

validate가 침묵하는 실증 사례들:

- plan이 stack services 맵에 없는 서비스를 참조해도 침묵 (gizzahub: temporal/kafka/prometheus)
- 미참조 environments/sites dead 선언 (matdosa)
- no-op entry_overrides (flow-taskchain, funbricks-postkit)
- default_plan 미설정 (multi-plan인데 기본 없음 — 3개 프로젝트)
- 빈 command: "" (sigdock-pass)
- note/주석 문자열 속 제거된 CLI(`dva dev`, `-M`, `dva clean`, `dva app up`) 참조 (3개 프로젝트)
- 최상위 고아 health_checks (flow-knowchain, flow-pipechain)

## Completion Criteria

- [ ] 위 각 케이스에 대한 warning 규칙 + 픽스처 테스트 (docs/51-flowcheck-rules.md 갱신 포함) | verify: `make test`
- [ ] gizzahub/matdosa 설정에서 해당 warning이 실제 출력됨 | verify: human — 출력 첨부
