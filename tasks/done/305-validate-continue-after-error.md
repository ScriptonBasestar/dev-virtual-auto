---
id: TASK-305
title: "validate: continue diagnostics after hard errors"
type: chore
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-05T09:00:00+09:00
source: "reports/{familybook,flow-agent-mesh,sadawiki,scripton-signalhub,primeno1}.md"
status: done
---

# Task 305: validate 에러 후에도 진단을 계속 출력

## Summary

interaction.clean replace 훅 같은 hard error가 발생하면 validate가 거기서 멈춰,
legacy 설정의 전체 문제 목록을 한 번에 볼 수 없다. 5개 devbox 프로젝트에서
마이그레이션 전체 그림 파악을 막은 실증 사례.

## Direction

- 에러를 수집(collect)하고 가능한 진단을 끝까지 수행한 뒤 일괄 출력, exit code는 유지.
- 파싱 자체가 불가능한 경우만 조기 종료.

## Completion Criteria

- [x] 복수 에러 픽스처에서 전체 에러 목록 출력 테스트 | verify: `make test`
- [x] familybook/primeno1 dva 설정에 대해 한 번의 validate로 전체 legacy 목록 출력 확인 | verify: human — 실행 출력 첨부

## Completion Evidence (2026-09-05)

**Root cause.** Three separate first-error returns: `config.Load` failed on the first
malformed stack entry (`ResolvePluginFromName` / `validateEntrySource` in
`finalizeLoadedConfig`), `Config.Validate()` returned on its first failing check
(`validateHookPlacement` even sorted the problems and returned only `problems[0]`), and
the validate command returned on the first of load / Validate / env_bridge / unrunnable
compose.

**Fix.**
- `Config.Validate()` runs every check and returns `config.ValidationErrors`
  (`Unwrap() []error`) when more than one fails; a single failure keeps its exact message.
  `validateHookPlacement` returns every dead hook, sorted.
- New load option `config.CollectEntryProblems()`: entry-shape errors are recorded on the
  config (`LoadProblems()`) instead of failing the load. Only `dva validate` uses it, after
  the strict load fails; lifecycle commands still load strictly. Parse failures and
  non-entry load errors still exit immediately.
- `dva validate` collects hard errors from load problems, Validate, env_bridge and the
  unrunnable-compose check, runs every warning printer, then fails once with a numbered
  list (`N errors found in this config:`). `--json` unwraps the list into `errors[]`.

**Tests.** `internal/cli/validate_collect_test.go` (4-error fixture: legacy compose entry,
schema, dead `clean` hook, missing `default_plan`; prose + JSON; single-error message
unchanged), `hook_placement_test.go` asserts both nested hooks are reported.
`make test`, `make lint` pass.

**Human criterion.** The devbox working trees were already migrated by their owners when
this ran (uncommitted), so the committed `HEAD` versions were extracted and validated:

- familybook `HEAD:dva.yaml` — one run, rc 1, `3 errors found`: the legacy
  `stack.compose` entry error, the full schema list (13 keys: `environments.dev.env_file`,
  `modes.*.compose_files`, `env_file.interpolate/priority`, `checks.5/6.address`, …) and
  `interaction.clean` dead hooks. Previously only the entry error was shown.
- primeno1 `HEAD:dva.yml` — one run, rc 1: `modes` deprecation warning, no-plans warning,
  5 compose-file drift warnings, then the `interaction.clean` error. Previously the error
  alone.
