---
id: TASK-325
title: "Refresh the stale flowcheck example output in docs/51"
type: docs
priority: P3
effort: S
exec-tier: standard
created-at: 2026-09-05T15:10:00+09:00
source: "TASK-244 done-disposition observation, 2026-09-05"
parent: PLAN-007
status: todo
---

# Task 325: docs/51 flowcheck 예시가 낡은 개수를 찍는다

## Summary

`docs/51-flowcheck-rules.md` "판정 옆의 개수" 절의 예시 출력이 `23 built-in command(s)`를 보여주지만
현재 `make doc-check`의 flowcheck는 `26 built-in command(s)`를 찍는다(agent-deny·kubectl 추가 이후).
그 절의 논지가 "개수가 검사 범위 손실을 드러낸다"인데 예시 자체가 낡아 있으면 논지를 스스로 약화한다.
예시를 실제 출력으로 갱신하고, 가능하면 개수를 손으로 적지 않는 서술로 바꾼다.

## Completion Criteria

- [ ] docs/51의 예시 블록이 현재 flowcheck 출력과 일치한다 | verify: `make doc-check 2>&1 | /usr/bin/grep -o '[0-9]* built-in command(s)' | head -1 | xargs -I{} /usr/bin/grep -q '{}' docs/51-flowcheck-rules.md`
- [ ] 예시 앞뒤 서술이 "이 숫자는 예시 시점 값"임을 밝히거나 숫자 없이 규칙만 설명한다 | verify: human — 절을 읽고 낡은 숫자가 다시 논지를 깨지 않는 구조인지 확인
- [ ] 문서 게이트 통과 | verify: `make doc-check`

## Non-goals

- flowcheck 출력 형식 변경 없음.
