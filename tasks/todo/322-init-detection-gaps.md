---
id: TASK-322
title: "init: detection gaps observed on 6 non-adopted devbox projects"
type: chore
priority: P2
effort: M
exec-tier: strong
created-at: 2026-09-05T10:30:00+09:00
source: "docs/dogfood/{scripton-dashboard,gzh-cli,flow-station,mansero,lottomaster,scripton-code}.md — TASK-249 evidence"
status: todo
---

# Task 322: init 탐지 결함 (TASK-249 후속 증거)

1. compose 없는 6개 프로젝트 모두 `dva init --dry-run` → "no Docker Compose file detected … no recognized language manifest" exit 1.
2. `--recursive`가 루트 compose 부재에서 즉시 종료 — 하위 go.mod 12개(gzh-cli), dashboard-webui/package.json 미탐색. help 문구와 불일치.
3. 루트 `go.work`를 language manifest로 인식하지 않음.
4. `PORT_MAPPINGS.yaml`(소문자 변형 포함), `.gz-git.yaml` workspaces, Makefile `dev-*`/`build`/`test` 타깃을 읽지 않음 —
   scripton-dashboard는 이 세 소스만으로 native 엔트리 2개 + plan + endpoints 기계 유도 가능 (리포트에 골격 있음).
5. `--dry-run`이 탐지 실패만 출력하고 "생성됐을 내용" preview 없음.

TASK-249의 capability-driven 생성기 설계에 위 fixture를 반영한다.

## Completion Criteria

- [ ] scripton-dashboard 루트 파일을 fixture로 한 native-only 탐지 테스트 | verify: `make test`
- [ ] --recursive가 루트 compose 없이도 하위 탐색 | verify: `make test`
