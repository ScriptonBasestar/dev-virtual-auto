---
id: TASK-327
title: "Record the TASK-284 temp-name supersession in PLAN-002"
type: docs
priority: P3
effort: S
exec-tier: standard
created-at: 2026-09-05T15:10:00+09:00
source: "TASK-284 done-disposition observation, 2026-09-05"
parent: PLAN-007
status: todo
---

# Task 327: PLAN-002에 TASK-284 supersession 노트가 없다

## Summary

TASK-245 §7-4/§8-5는 env bridge 임시 파일 이름 형태를 정했고, TASK-284(bbe3db1)는 target 디렉토리 고정을
위해 `<leaf>.dva-env-<pid>-<token>.tmp` 형태로 바꿨다. `USAGE.md`와 코드 주석은 새 형태를 따르지만
PLAN-002는 TASK-281 supersession은 두 곳(82행·299행 부근)에 기록하면서 284는 제목 한 줄만 둔다.
PLAN-002를 읽는 사람은 245 계약이 아직 유효하다고 오해한다.

## Completion Criteria

- [ ] PLAN-002에 245 §7-4/§8-5 → 284 supersession 노트가 TASK-281 노트와 같은 형식으로 있다 | verify: `/usr/bin/grep -Eq 'TASK-284.*(supersed|대체|우선)' tasks/plan/002-command-surface-delivery.md`
- [ ] 노트가 가리키는 코드 위치(`internal/cli/config_env_safewrite.go` tempName 주석)와 형태가 일치한다 | verify: human — 주석의 이름 형태와 노트 문구 대조
- [ ] 문서 게이트 | verify: `make doc-check`

## Non-goals

- 245 아카이브 카드 본문은 고치지 않는다. 아카이브는 불변이다.
