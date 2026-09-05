---
id: TASK-326
title: "doccheck anchor slugs drop underscores and diverge from GitHub"
type: bug
priority: P2
effort: S
exec-tier: standard
created-at: 2026-09-05T15:10:00+09:00
source: "TASK-246 done-review: named a follow-up card target, none existed"
parent: PLAN-007
status: todo
---

# Task 326: doccheck가 제목의 `_`를 지워 앵커가 GitHub와 어긋난다

## Summary

`tools/doccheck/anchors.go` `stripHeadingInline`이 강조 마커를 제거할 때 `"_"`를 통째로 지운다.
`sops_source` 같은 식별자가 든 제목은 GitHub에서 `#sops_source` 앵커가 되지만 doccheck는
`#sopssource`로 계산해, 올바른 링크를 "missing target"으로 거부하고 잘못된 링크를 통과시킨다.
TASK-246은 제목에서 `_`를 빼는 우회로 문서를 통과시키고 후속 카드를 예고했으나 카드가 없었다.

## Cause

`_`는 단독으로는 강조가 아니다. GitHub는 `_word_` 쌍만 강조로 보고, 단어 안의 `_`는 그대로 앵커에 남긴다.
현재 코드는 쌍 여부를 보지 않고 문자를 제거한다.

## Completion Criteria

- [ ] 단어 내부 `_`는 앵커에 보존되고, 쌍을 이룬 `_..._` 강조만 제거된다 | verify: `go test ./tools/doccheck -run 'Anchor|Heading' -count=1`
- [ ] `sops_source`류 제목을 가진 fixture가 GitHub 앵커와 같은 슬러그를 산출하는 테스트가 있다 | verify: `/usr/bin/grep -Eq 'sops_source|snake_case' tools/doccheck/*_test.go`
- [ ] TASK-246이 우회한 제목이 있으면 원래 표기로 되돌려도 doc-check가 통과한다 | verify: `make doc-check`
- [ ] 전체 게이트 | verify: `make lint && make test && make doc-check`

## Non-goals

- 다른 GitHub 앵커 규칙(중복 제목 `-1` 접미, 이모지)은 이 카드 범위 밖.
