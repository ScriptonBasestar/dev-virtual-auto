---
archived-at: 2026-04-01T15:46:00+09:00
verified-at: 2026-04-01T15:46:00+09:00
verification-summary: "Implemented warnUnresolvedEnvVars and warnSuspiciousEnvPatterns in validate_warnings.go and integrated with config validation logic."
---
---
# Environment Interpolation Semantic Warnings

## 배경

`tasks/_archive/environment-interpolation-hardening.md` 설계에 따라, unresolved 변수와 의심스러운 interpolation 패턴에 대한 Semantic Warning을 구현해야 합니다.

## 범위

- [x] `warnUnresolvedEnvVars` 함수 추가
- [x] `warnSuspiciousEnvPatterns` 함수 추가
- [x] `ValidateWarnings()` 연동
- [x] 단위 테스트 추가 및 문서화

## 참조
- `tasks/_archive/environment-interpolation-hardening.md`
