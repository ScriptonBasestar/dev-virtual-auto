---
status: todo
---
# Interaction Inheritance Warnings Implementation

## 배경

`tasks/_archive/interaction-inheritance-semantics.md` 설계에 따라, interaction 명령어의 override와 중첩 깊이에 대한 Semantic Warning을 구현해야 합니다.

## 범위

- `warnChildOverridesParentCritical()` 함수 구현
- `warnDeepSubcommandNesting()` 함수 구현
- `warnUnreachableCommands()` 함수 구현 
- `ValidateWarnings()` 연동
- 관련 단위 테스트 추가

## 참조
- `tasks/_archive/interaction-inheritance-semantics.md`
