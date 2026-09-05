---
id: PLAN-007
title: "Retire the unreviewed done backlog and move quality review to card close"
type: plan
scope: "tasks/done/ 58장의 일회성 트리아지, done-disposition 루프에서 발견된 소유자 없는 결함 3건, 닫기 시점 독립 리뷰 전환의 외부 의존"
progress: 0
total-tasks: 3
completed-tasks: 0
children: [TASK-325, TASK-326, TASK-327]
target-date: "2026-09-30"
created: 2026-09-05
---

## Goal

`tasks/done/`가 리뷰 대기열로 쌓이지 않게 한다. 지금은 카드를 닫는 세션이 `done-review`를 돌리지 않아
done 카드 58장 전부에 `quality-review`가 없고, 아카이브 직전의 done-disposition 루프 하나가 리뷰를 전담한다.
리뷰가 변경 시점에서 분리돼 직렬 병목이 됐고, 유입(여러 세션의 병렬 닫기)이 배출(한 프롬프트에 한 장)보다 빠르다.

완료 상태:

1. 58장이 아래 티어대로 처리되어 `tasks/done/`가 비어 있다.
2. 리뷰 없이 아카이브한 카드는 `quality-review` 값으로 그 사실이 정직하게 남아 있다.
3. 루프가 관찰로만 남겼던 결함 3건이 자식 카드로 처리됐다.
4. 이후 done 진입 조건에 독립 리뷰가 요구된다 (외부 의존, §External).

## Starting state (2026-09-05)

- done 58장, `quality-review` 보유 0장. `_archive/done` 245장.
- done-disposition 4사이클(TASK-244·245·246·284, 2026-09-05) 수확: 실질 결함 0건, 문서·메타 정리 4건.
  TASK-284의 major finding은 루프가 아니라 독립 리뷰가 찾은 것을 루프가 기록했을 뿐이다.
  기록: `tmp/task-management/direct/done-disposition/` (gitignored).
- 엔진(`ce-workbook/task_management`)은 이미 done-review → `quality-review` → done-disposition → archive를
  분리해 두고 있고, `ce task archive`는 `quality-review: pass|conditional` 없는 카드를 거부한다.
  빠진 것은 설계가 아니라 **호출 시점**이다.
- PLAN-006이 다른 세션에서 활성이며 TASK-303~317을 소유한다. 그 카드들은 이 계획이 건드리지 않는다.

## Triage

### Tier A — 실질 done-review (13장)

보안·비밀·데이터 손실·롤백 표면이거나 P0 결정. 지금 방식(binding 재실행 + diff 대조)으로 한 장씩.
리뷰어는 구현 세션과 달라야 한다.

| Task | 이유 |
|---|---|
| TASK-248 | P0. required env 정책이 진단을 깨지 않게 강제 |
| TASK-252 | P0 결정. top-level env promotion 안전성 |
| TASK-259 | P0 결정. qualified project addressing |
| TASK-260 | P0 결정. cross-project 조합 계약 |
| TASK-261 | P0 결정. vNext 어휘·마이그레이션 약속 |
| TASK-263 | P0 결정. addressing 노출·충돌 |
| TASK-265 | P0 결정. interaction env_file 호환 |
| TASK-277 | env_file 보간 순서 비결정성 — TASK-246 리뷰가 실재 확인 |
| TASK-281 | env bridge seal/show 계약 동결 |
| TASK-282 | env bridge seal/show 구현 — 비밀 취급 표면 |
| TASK-286 | agent-runtime deny rule 투영 — 파괴적 명령 차단 |
| TASK-291 | composition 런타임·LIFO 롤백 |
| TASK-296 | readiness 실패 시 자식 방치·롤백 누락 |

### Tier B — 기계적 아카이브, `waived` 표기 (36장)

docs/chore/P2·P3, 라우트 결정·cobra 도움말·문서 수정·릴리스 절차·PLAN-005 조합 구현의 나머지.
`ce task validate` 통과 + 부모 링크 재지정 + `quality-review: waived`(엔진이 값을 받으면) 또는
`conditional` + evidence 한 줄 `waived: PLAN-007 backlog triage, no independent review`. 한 커밋으로.

250, 254, 255, 256, 257, 258, 266, 267, 268, 269, 270, 271, 272, 273, 274, 275, 276, 278, 279, 280,
283, 285, 287, 288, 289, 290, 292, 293, 294, 295, 297, 298, 299, 300, 301, 302

### Tier C — 보류, 활성 소유자 있음 (9장)

PLAN-006 자식. 그 계획을 진행하는 세션이 닫는 시점에 리뷰하도록 맡긴다. 이 계획은 건드리지 않는다.

303, 304, 305, 306, 308, 310, 312, 313, 317

## Order

| # | 작업 | 조건 |
|---|---|---|
| 1 | TASK-325, 326, 327 처리 | 독립. 지금 가능 |
| 2 | §External의 엔진 정책 변경 | ce 소스 체크아웃 필요. 별도 태스크 |
| 3 | Tier B 일괄 아카이브 | 2가 `waived` 값을 주면 그 값으로, 아니면 `conditional` 대체 표기 |
| 4 | Tier A 13장 done-review | 3과 병행 가능. P0 결정 카드부터 |
| 5 | Tier C | 이 계획 범위 밖. PLAN-006 종료 시 잔여분만 재평가 |

## External

`ce-workbook/task_management` 엔진 소스(생성물 `generated/` 아님)에 두 가지:

- `run-finish` 또는 `move --to done` 전제조건에 `quality-review` 존재 요구. 대기열 재발을 막는 유일한 지점.
- `host-schema.yaml` `quality-review_values`에 `waived` 추가. 리뷰 생략을 `conditional`로 위장하지 않기 위해.

DVA 쪽 문서는 이 계획이 끝난 뒤 `AGENTS.md`에 "카드를 닫으려면 별도 세션의 done-review가 필요하다"
한 줄과 정책 링크만 둔다. 규칙 본문을 복제하지 않는다.

## Rules for this plan

- 동시 개발 중이다. 기존 done 카드를 수정하는 커밋은 통합 직전 origin/master로 rebase하고,
  카드 파일 외 경로를 건드리지 않는다.
- done-disposition 루프 중 소유자 없는 결함을 보면 같은 사이클에서 todo 카드를 만든다. 기록만 남기는
  관찰은 손실이다.
- Tier B 아카이브가 "검토됨"으로 읽히면 안 된다. `waived`/대체 표기 없이는 아카이브하지 않는다.

## Children

- TASK-325 — docs/51 flowcheck 예시 출력 갱신
- TASK-326 — doccheck 앵커 슬러그가 `_`를 버려 GitHub와 어긋남
- TASK-327 — PLAN-002에 TASK-284 temp-name supersession 기록
