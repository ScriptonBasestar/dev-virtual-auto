---
id: TASK-316
title: "compose drift detection: include, compose-*.yaml, subdirectories, asymmetry"
type: bug
priority: P2
effort: M
exec-tier: standard
created-at: 2026-09-05T10:30:00+09:00
source: "docs/dogfood/{gizzahub,sigdock-pass,sigdock-idp,primeno1,flow-knowchain}.md"
status: todo
---

# Task 316: compose drift 감지 결함

## Findings

1. `include:`로 분산된 서비스를 따라가지 않음 — gizzahub 루트 compose.yaml에 services 블록 없음에도 drift warn 0 (false negative 의심).
2. `compose.yml` + `compose-*.yaml` 명명은 감지 목록에 없어 미등록 overlay 10개여도 경고 없음. 반대로 등록하면 "선언 파일이 감지 목록에 없음" 경고 (sigdock-pass, 편집 전/후 validate로 재현).
3. 서브디렉터리 compose(env/docker-compose/*.verify.yml)를 보지 않음 (primeno1).

ignore 선언 수단은 TASK-309 범위.

## Completion Criteria

- [ ] 감지 패턴 확장 + include 추적 + 등록 파일 비대칭 경고 제거, 픽스처 테스트 | verify: `make test`
