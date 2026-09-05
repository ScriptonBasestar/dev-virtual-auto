---
id: PLAN-006
title: "Work the devbox dogfood follow-up queue in dependency order"
type: plan
scope: "TASK-311..323 from the 2026-09-05 mydevbox migration, plus the needs-human cards that gate the rest"
progress: 35
total-tasks: 17
completed-tasks: 6
children: [TASK-312, TASK-313, TASK-317, TASK-311, TASK-324, TASK-314, TASK-316, TASK-320, TASK-322, TASK-315, TASK-318, TASK-323, TASK-249, TASK-307, TASK-309, TASK-319, TASK-321]
target-date: "2026-10-31"
created: 2026-09-05
---

## Goal

2026-09-05 mydevbox dogfood(23개 devbox 저장소를 dva v0.1.48 plans 스키마로 이전)에서 나온 결함·
설계 카드를 하나씩 처리한다. `reports/`·`tmp/`는 ignore 대상이라 이 카드가 상태의 정본이다.
카드는 `tasks/todo/`, 완료는 `tasks/done/`, 이 문서는 순서·의존·상태만 소유한다.

## Starting state (2026-09-05)

- 완료·통합: TASK-303, 304, 305, 306, 308, 310 (dva master d7636a3, e3c562d, 9b74de9, b2c2d13).
- devbox 이전 커밋 `chore(dva): migrate dva.yml to plans and clear validate warnings`가 19개
  저장소 trunk에 착지. 미착지 4개는 §Blocked 참조.
- 설계 대기 문서: docs/55(alias/extends), docs/56(suppression). 결정 항목은 각 문서 §5.

## Order (decision-free first)

| # | Task | Why here |
|---|---|---|
| 1 | ~~TASK-312~~ dry-run up이 native health를 기다림 | 완료 2026-09-05. P1 S. 다른 카드의 dry-run 검증을 막았음 |
| 2 | ~~TASK-313~~ local 러너 workdir 무시 | 완료 2026-09-05. P1 S. 독립 |
| 3 | ~~TASK-317~~ migrate 힌트 오류·legacy 누락 | 완료 2026-09-05. P1 M. 306 스캐폴드와 별개 |
| 4 | ~~TASK-311~~ down <plan> volume/network 잔존 | 완료 2026-09-05. P1 M. `--purge`가 프로젝트 전체 down |
| 4a | ~~TASK-324~~ composition plan 중복 경고 오탐 | 완료 2026-09-05. P2 S. `Composes` 비교 추가 |
| 5 | ~~TASK-314~~ logs/build plan 범위 | 완료 2026-09-05. P2 S |
| 6 | TASK-316 drift 감지 결함 | P2 M. docs/56 `drift_ignore`(309)보다 먼저 — 감지 폭이 억제 설계의 입력. 2026-09-05 분석 완료(카드 §Analysis), 코드 미착수 |
| 7 | TASK-320 suggestion 파서·manifest | P3 S. 309 결정 C의 소스 개선과 겹치므로 309 전에 |
| 8 | TASK-322 init 탐지 결함 | P2 M. 249 재설계와 겹치지 않는 탐지 버그만 |
| 9 | TASK-315 compose profiles | P2 M. 둘째 기준 human |
| 10 | TASK-318 섹션 순서 자동 정렬 | P3 S |
| 10a | 실기동 검증 회차 (카드 없음) | 311 완료로 착수 가능. primeno1 native 6종(gate 체인+`exec`), familybook/flow-taskchain composition plan을 `dva up`/`status`/`down --purge` 실제 실행으로 확인하고 각 리포트에 출력 첨부. 결함이 나오면 카드로 승격 |
| 11 | TASK-323 문서 의미 공백 | P3 S. `--env` 항목은 307 결정에 따라 문구가 달라짐 — 마지막 |

## Needs-human (결정 후 착수)

- TASK-307 → docs/55 §5 (4항목). TASK-309 → docs/56 §5 (5항목). 309는 316·320 뒤.
- TASK-319 native entry ergonomics, TASK-321 destructive interaction agent-deny: 설계 결정 기록 필요.
- TASK-249 capability-driven init: PLAN-002 child. 322와 경계 확인 후.

## Devbox integration state (2026-09-05 저녁 갱신)

- 통합 완료: db-orchestrator 6d0e3e07(master), primeno1 768cbb4(native 6 엔트리), familybook 9d559fb(자식 import,
  engine-fiber 3d30df87·client-flutter b792a5b9), flow-taskchain 2134c8c + 자식 4개, dns-bridge 원격 브랜치 삭제 완료.
- 미통합 1건: scripton-dashboard `dev/claude/mst/chore/dva-yml-migrate`(25dd928). 해법 작업 중 브랜치
  `dev/claude/mst/chore/check-prepares-subprojects`(50cf918, 체크포인트): `check`가 order-only 전제로 `prepare`·
  `pnpm install`을 자동 수행. 남은 결함 — dashboard-webui의 `file:` 링크가 install 시점에 복사되므로
  protocol/components `dist`를 dashboard install보다 먼저 빌드해야 함(`make install` 순서와 동일). 워크트리
  `~/worktrees/scripton-dashboard/scripton-dashboard-devbox/claude__mst__chore__check-prepares-subprojects` 유지.
  완료 후 이 브랜치 → develop 통합 → dva-yml-migrate 브랜치 rebase 후 통합 순. 래퍼가 target을 임시 트리에
  체크아웃해 `make check`를 돌리므로 gitignore된 `scripton-mfe-protocol`이 없어 baseline 측정 불가.
  `--allow-skipped-checks`로도 우회 안 됨. 해법은 devbox Makefile `check`가 `prepare`를 선행하거나 사람이 통합.
- 사람 통합 대기: familybook `dev/claude/mst/chore/readiness-dva-yml`(3538cf2) — readiness contract가
  `dva.yml`/`dva.yaml` 둘 다 허용하도록 수정. 계약 파일은 에이전트 통합 불가. 통합 후 `dva.yaml`→`dva.yml` 개명 재시도.
- 세션 종료 2026-09-05 저녁(dva 2cb184e): 이 세션은 TASK-324 완료, PLAN-006/TASK-323 현행화, familybook 자식 통합
  (`.gz-git.yaml` workspace 항목 + `integration: {}`)까지. 다른 세션이 311·313·314·317을 병행 처리했으므로
  다음 착수 전 `git log origin/master`와 `tasks/todo/` 재확인.
- 실기동 검증 미실시: primeno1 native 엔트리(gate 체인 + `exec`)와 familybook/flow-taskchain composition plan은
  dry-run까지만 확인. TASK-311 완료 후 실기동 회차 필요.

## Session handoff (2026-09-05 밤)

- 세션 1회차 종료 시점: 1~5·4a 통합 완료(master 2cb184e). 316은 분석만 카드에 기록, worktree 없음.
- 다음 착수: TASK-316 → 320 → 322 → 315 → 318 → 10a 실기동 → 323. 카드 §Analysis에서 바로 시작.
- 잔여 사람 결정: 위 §Needs-human, §Devbox integration state의 scripton-dashboard·familybook 2건.

## Rules

- 카드 하나 = worktree 하나 = 통합 하나. 완료 시 카드를 `tasks/done/`으로 옮기고 이 문서의
  `completed-tasks`·`progress`를 같은 커밋에서 갱신한다.
- dry-run 검증이 필요한 카드(311, 315)는 1번 완료 전에는 착수하지 않는다.
