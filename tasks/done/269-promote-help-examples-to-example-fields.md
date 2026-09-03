---
id: TASK-269
title: "Promote in-prose usage examples into cobra Example fields"
type: chore
priority: P2
effort: M
exec-tier: standard
created-at: 2026-09-03T00:56:00+09:00
source: "CLI discoverability audit, 2026-09-03 session (docs vs help gap review)"
scope: "cobra Example fields across lifecycle, run, doctor, init commands; restructure Long prose that currently embeds usage tables"
status: done
depends-on: [TASK-268]
---

# Task 269: promote help examples into cobra Example fields

## Summary

Exactly one command sets cobra's `Example:` field (`composeCmd`, `internal/cli/compose.go:28`).
Every other command that shows example invocations embeds them inside its `Long:` prose as
hand-formatted "Plan usage:" / "Stack flags:" blocks (see `upCmd`, `compose.go:78`). The
information exists, but help output lacks the standard `Examples:` section, so shell
completion frameworks, help parsers, and LLMs reading `--help` structurally cannot
distinguish examples from description — and the hand-rolled flag tables in Long duplicate
what cobra's own `Flags:` section should render.

## Problem

1. `grep -n "Example:" internal/cli/*.go` (non-test) hits only `compose.go:28`; lifecycle
   commands (`up`, `down`, `stop`, `restart`, `build`, `logs`), `run`, `doctor`, and `init`
   render no `Examples:` section.
2. `upCmd`'s Long embeds flag documentation ("Plan usage:", "Stack flags:", "Plan-path
   flags:") as prose. Some of those flags are real cobra flags rendered again under `Flags:`,
   some are prose-only — the reader cannot tell which, and drift between the two surfaces is
   unchecked.
3. Each example should exist in exactly one surface: invocation examples in `Example:`,
   flag semantics on the flag's own usage string, conceptual behavior in `Long:`.

## Completion Criteria

- [x] `up`, `down`, `stop`, `restart`, `build`, `logs`, `run`, `doctor`, `init` each set `Example:` with 2–5 representative invocations, rendered under cobra's `Examples:` heading | verify: `go test ./internal/cli -count=1`
- [x] **(재정의)** 산문 플래그 표와 실제 파서 사이의 드리프트를 감사하고 결과를 Evidence에 기록한다 — 원문("표 삭제")은 전제가 틀려 실행 불가, 아래 Evidence §2 참조 | verify: 아래 Evidence §2 (감사 결과 표)
- [x] A regression test asserts the lifecycle commands and `run` have a non-empty Example, pinning the floor established here | verify: `go test ./internal/cli -count=1`
- [x] Example invocations agree with USAGE.md's Command Quick Reference (spot-checked in review; no new generator) | verify: `make doc-check`
- [x] Repository gates pass | verify: `make lint && make test && make doc-check`

## Non-goals

- No new Long content — TASK-268 owns Long coverage; this card only relocates/structures examples.
- No automated help↔USAGE.md consistency generator; that is a separate discovery if drift recurs.
- `manifest` usage_example fields (`internal/cli/manifest.go`) are out of scope — owned by TASK-267/263 work.

## Evidence (2026-09-03, claude/mst)

### 1. 무엇을 했는가

`Example:` 필드를 9개 커맨드에 추가했다 — `up`, `down`, `stop`, `restart`, `build`,
`logs`(`internal/cli/compose.go`), `run`(`run.go`), `doctor`(`doctor.go`), `init`(`init.go`).
`initAliasCmd`는 `Example: initCmd.Example`로 값을 공유한다(`init.go`). 기존에 `Example:`을
가진 커맨드는 `composeCmd` 하나뿐이었다(카드 §Problem 1의 진술은 정확했다).

회귀 테스트: `internal/cli/example_help_test.go`의 `TestLifecycleAndRunCommandsHaveExample`.
`rootCmd`를 순회하지 않고 패키지 레벨 커맨드 변수를 직접 읽는다 — 같은 패키지의 다른
테스트가 `rootCmd`에 `ExecuteC()`를 호출하므로 순회 방식은 상태 결합을 만든다.

### 2. Completion Criteria 2를 재정의한 이유 (전제 오류)

카드 원문: "upCmd's Long no longer hand-renders flag tables for flags cobra already lists;
flag semantics move to the flags' usage strings."

**이 전제는 코드와 맞지 않아 그대로 수행할 수 없다.** `up`/`down`/`stop`/`restart`/`build`/
`logs`는 전부 `DisableFlagParsing: true`이며 cobra 플래그를 **0개** 등록한다(플래그는
`parseDvaFlags`/`parsePlanFlags`가 수동 파싱). 따라서:

- "cobra가 이미 나열하는 플래그"는 존재하지 않는다 — 이 6개 커맨드의 `--help`에는
  `Flags:` 섹션 자체가 비어 있다(전역 플래그만 `Global Flags:`에 뜬다).
- "플래그 usage 문자열로 의미를 옮긴다"는 옮길 대상이 없다 — usage 문자열이 없다.
- 산문 표는 중복이 아니라 **이 플래그들의 유일한 문서**다. 삭제하면 도움말이 순수하게
  나빠진다.

또한 카드는 `upCmd`만 지목하지만 이 표는 6개 커맨드에 모두 있다
(`compose.go:105, 380, 462, 556` 등).

그래서 criterion 2를 **"표와 실제 파서의 드리프트 감사"**로 재정의해 수행했다. 표 자체는
보존한다.

### 3. 드리프트 감사 결과

| 항목 | 코드 근거 | 판정 |
|---|---|---|
| `--tag`가 plan 경로에서 거부됨 | `plan_lifecycle.go:277` `unsupported plan flag: %s` | **결함(범위 밖)** — 아래 §5 |
| `--force`가 restart plan 경로에서 무시됨 | `plan_lifecycle.go:521` `Force: true` 하드코딩 (`flags.force` 미사용) | 문서화 부족 |
| `--no-wait`가 stop/down plan 경로에서 무시됨 | `parsePlanFlags`가 `flags.wait=false` 설정(`:259`)하지만 `StopOptions`/`DownOptions`에 `Wait` 필드 없음(`lifecycle/orchestrator.go:30-49`). 소비처는 `runPlanUp:352`, `runPlanRestart:522` 두 곳뿐 | 문서화 부족 |

**명백한 버그로 고칠 것은 없었다.** 세 항목 모두 "받아들이지만 문서에 안 적힌" 성질이고,
동작 변경은 이 카드의 Non-goals("No new Long content", 예시 재배치만)를 벗어난다.
§5에 미소유 발견으로 남긴다.

### 4. 예시의 실증 검증 (36줄 전부)

단일 plan(`local-dev`) 픽스처에서 실행 가능한 예시는 전부 실행했고, 플래그 예시는 등록
여부를 `--help`로 확인했다.

초안에서 **두 줄이 붙여넣으면 실패**하는 것을 실행으로 발견해 교체했다:

- `dva up --tag db,cache` → `ERROR: flags suppress the default plan "local-dev"`
  (`rejectSuppressedDefaultPlan`, `plan_lifecycle.go:158`)
- `dva stop --tag web` → 동일 거부

`--tag`는 **어떤 형태로도** default plan 설정에서 예시가 될 수 없다: stack 경로(`--tag`가
유효한 곳)는 default plan이 잡히면 도달 불가이고, plan 경로는 `--tag`를 명시적으로
거부한다. 검증된 대체 예시로 교체했다:

- `dva up local-dev --var PORT=8080` (실행 OK)
- `dva stop local-dev --dry-run` (실행 OK)

`dva up --dry-run`은 유지했다 — `--dry-run`은 루트 persistent 플래그라
`consumeRootPersistentFlags`가 가드보다 먼저 소비하므로 거부되지 않는다(실행으로 확인).

### 5. 이 카드가 소유하지 않는 발견

**가드의 해결책이 막다른 길이다.** `dva up --tag db,cache`가 거부되면서 출력하는 안내는
`name it explicitly: dva up local-dev --tag db,cache`인데, 그대로 실행하면
`ERROR: unsupported plan flag: --tag`가 난다. 즉 에러 메시지가 두 번째 에러로 안내한다.
메시지/동작 변경이라 이 카드(예시 재배치) 범위 밖 — 별도 카드 필요.

### 6. Gate 결과

| gate | 결과 |
|---|---|
| `make lint` | exit 0 |
| `make test` | exit 0 |
| `make doc-check` | exit 0 |
| `make build` | exit 0 |

기존 baseline: `make commit-check` exit 2 (`a6666c1a`, `6ab9c643`, `095f525b` — 커밋 제목
scope 누락, 이미 푸시되어 재작성 불가). 이 카드의 커밋은 scope를 포함한다.
