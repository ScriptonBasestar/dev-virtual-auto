---
id: TASK-306
title: "migrate: emit actual plans scaffold YAML for modes"
type: feature
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-05T09:00:00+09:00
source: "reports/{primeno1,sigdock-idp,sadawiki}.md"
status: todo
---

# Task 306: migrate가 modes → plans 스캐폴드 YAML을 실제 출력

## Summary

현재 migrate는 modes에 대해 "Left for you"만 남긴다(primeno1: modes 6개 전부 수동).
그러나 "stack 선택만 하는 mode"(sigdock-idp)와 "compose_services 목록만 있는
mode"(sadawiki)는 plan으로 기계 변환 가능한 하위 클래스임이 확인됐다.

## Direction

- 기계 변환 가능한 mode 하위 클래스를 식별해 plans 스캐폴드 YAML을 preview로 출력.
- 변환 불가능한 mode는 사유와 함께 명시적으로 남긴다. 파일 자동 수정은 opt-in.
- interaction.clean → down.after 자동 이관도 이 변환기에 포함 검토.

## Completion Criteria

- [ ] mode 하위 클래스별(stack 선택형/compose_services형/변환 불가형) 픽스처 테스트 | verify: `make test`
- [ ] primeno1 설정에서 migrate preview가 6개 mode 중 기계 변환분의 plans YAML을 출력 | verify: human — 출력 첨부
