---
status: todo
---
# Validation Severity Code Refactoring

## 배경

`tasks/_archive/validation-severity-policy.md` 정책에 따라, 기존 validation 코드를 hard error, semantic warning, drift warning, suggestion warning으로 명확히 구분하는 작업을 수행해야 합니다.

## 범위

- 각 warning 함수에 코멘트로 severity 레벨 명시
- Drift Warning과 Semantic Warning 접두사 구분 강화 (`[warn] config drift:`, `[warn] config suggestion:`)
- `ValidateComposeProjectNames` 코드를 `doctor` 로직과 공유할 수 있도록 정리

## 참조
- `tasks/_archive/validation-severity-policy.md`
