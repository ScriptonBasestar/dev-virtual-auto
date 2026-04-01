---
archived-at: 2026-04-01T15:46:00+09:00
verified-at: 2026-04-01T15:46:00+09:00
verification-summary: "Implemented top-level doctor checks for env files, stack files, compose project alignments, and docker socket permissions. Tests pass."
---
---
# Doctor Preflight Implementation

## 배경

`tasks/_archive/doctor-preflight-expansion.md`에서 정리된 preflight 체크 후보들을 단계별로 구현해야 합니다.

## 범위

- [x] Compose project name alignment 체크 `doctor.go` 추가
- [x] Environment file 존재 체크 추가
- [x] Stack entry file 존재 체크 추가
- [x] Docker socket 권한 체크 추가
- [x] 각 항목에 대한 단위 테스트 및 통합 테스트 보강

## 참조
- `tasks/_archive/doctor-preflight-expansion.md`
