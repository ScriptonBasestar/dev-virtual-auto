---
id: TASK-305
title: "validate: continue diagnostics after hard errors"
type: chore
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-05T09:00:00+09:00
source: "reports/{familybook,flow-agent-mesh,sadawiki,scripton-signalhub,primeno1}.md"
status: todo
---

# Task 305: validate 에러 후에도 진단을 계속 출력

## Summary

interaction.clean replace 훅 같은 hard error가 발생하면 validate가 거기서 멈춰,
legacy 설정의 전체 문제 목록을 한 번에 볼 수 없다. 5개 devbox 프로젝트에서
마이그레이션 전체 그림 파악을 막은 실증 사례.

## Direction

- 에러를 수집(collect)하고 가능한 진단을 끝까지 수행한 뒤 일괄 출력, exit code는 유지.
- 파싱 자체가 불가능한 경우만 조기 종료.

## Completion Criteria

- [ ] 복수 에러 픽스처에서 전체 에러 목록 출력 테스트 | verify: `make test`
- [ ] familybook/primeno1 dva 설정에 대해 한 번의 validate로 전체 legacy 목록 출력 확인 | verify: human — 실행 출력 첨부
