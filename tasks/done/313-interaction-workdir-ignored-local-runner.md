---
id: TASK-313
title: "interaction.workdir ignored by local runner"
type: bug
priority: P1
effort: S
exec-tier: standard
created-at: 2026-09-05T10:30:00+09:00
source: "docs/dogfood/{funbricks-notifire,dripter}.md"
status: done
---

# Task 313: `interaction.workdir`가 local 러너에서 무시됨

## Repro

internal/runner/local.go는 어떤 form에서도 chdir하지 않고 Workdir는 docker_compose.go(--workdir)만 소비한다.
`x: {runner: local, workdir: dripter-engine-ktor, command: pwd}` → `dva run x`가 프로젝트 루트 출력. validate 경고 없음.
프로젝트들이 `cd sub && …` 체인을 유지하는 원인.

## Completion Criteria

- [x] local 러너가 workdir를 적용(상대 경로는 config 기준), 테스트 추가 | verify: `make test`
- [x] workdir 미존재 시 명확한 에러 | verify: `make test`

## Resolution (2026-09-05)

- 수정: `LocalRunner.runForm`이 모든 form 실행 전에 `enterWorkdir`로 chdir한다
  (internal/runner/local.go). 상대 경로는 `Config.FileDir()` 기준, config가 없으면 cwd 기준.
  `ExecReplace`(formCommand)는 프로세스를 교체하므로 dir 인자를 받을 수 없어 cwd가 5개 form의
  공통 경로다. `script_file` 상대 경로는 chdir 전에 해석해 기존 의미를 유지.
- 미존재/파일: `workdir "nope": directory not found (resolved to /abs/nope)` /
  `not a directory`.
- compose 러너의 `script:`/`script_file:` 호스트 폴백은 Workdir을 비운 사본으로 local을
  호출한다 — 거기서 workdir은 컨테이너 `--workdir` 경로이므로 호스트 chdir 대상이 아니다.
- 테스트: `TestLocalRunnerWorkdir`(config 디렉터리 밖에서 호출해 cwd 기준과 구분),
  `TestLocalRunnerWorkdirMissing`, `TestComposeScriptFallbackDropsWorkdir`
  (internal/runner/local_workdir_test.go). chdir 호출을 제거하면 앞 두 테스트가 실패함을 확인.
- 문서: USAGE.md "`workdir:` — 러너별 의미" 표 추가.
- 검증: `make build`, `make test`, `make lint`, `make generate`(변경 없음) exit 0.
