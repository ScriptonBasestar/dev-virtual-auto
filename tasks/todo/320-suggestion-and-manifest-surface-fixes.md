---
id: TASK-320
title: "Makefile suggestion parser and manifest usage_example fixes"
type: bug
priority: P3
effort: S
exec-tier: standard
created-at: 2026-09-05T10:30:00+09:00
source: "docs/dogfood/{flow-pipechain,dripter,scripton-nd-stack,funbricks-elemhant}.md"
status: todo
---

# Task 320: suggestion/manifest 표면 결함

1. Makefile 파서가 `a b:` 멀티타깃 라인을 하나의 타깃명으로 취급 (flow-pipechain `log-search-bench perf-log-search:`).
2. subprojects import로 들어온 interaction이 Makefile 제안 매칭에 반영되지 않음 (dripter test-e2e).
3. built-in과 동일 동작인 `logs`/`build` replace 훅에 "제거 가능" 경고 없음 (nd-stack) — 자동 탐지 후보.
4. manifest `usage_example`이 interaction `clean`에 대해 여전히 `dva clean` 형태를 생성 — `dva run clean`으로 통일 검토
   (funbricks-elemhant scripts/tests/test-dva-clean-command.sh가 이 값을 단언하므로 변경 시 해당 프로젝트 후속 필요).

## Completion Criteria

- [ ] 각 항목 테스트 + 수정 | verify: `make test`
