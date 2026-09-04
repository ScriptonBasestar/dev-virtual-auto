---
id: TASK-313
title: "interaction.workdir ignored by local runner"
type: bug
priority: P1
effort: S
exec-tier: standard
created-at: 2026-09-05T10:30:00+09:00
source: "reports/{funbricks-notifire,dripter}.md"
status: todo
---

# Task 313: `interaction.workdir`가 local 러너에서 무시됨

## Repro

internal/runner/local.go는 어떤 form에서도 chdir하지 않고 Workdir는 docker_compose.go(--workdir)만 소비한다.
`x: {runner: local, workdir: dripter-engine-ktor, command: pwd}` → `dva run x`가 프로젝트 루트 출력. validate 경고 없음.
프로젝트들이 `cd sub && …` 체인을 유지하는 원인.

## Completion Criteria

- [ ] local 러너가 workdir를 적용(상대 경로는 config 기준), 테스트 추가 | verify: `make test`
- [ ] workdir 미존재 시 명확한 에러 | verify: `make test`
