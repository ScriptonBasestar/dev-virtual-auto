---
id: TASK-268
title: "Add Long help to concept-bearing commands that only have Short"
type: chore
priority: P2
effort: M
exec-tier: standard
created-at: 2026-09-03T00:56:00+09:00
source: "CLI discoverability audit, 2026-09-03 session (docs vs help gap review); scope corrected 2026-09-03 after a full rootCmd tree walk"
scope: "cobra Long fields on the 21 Short-only commands in the rootCmd tree (see Problem inventory) plus a regression test; no behavior change"
status: done
depends-on: []
---

# Task 268: add Long help to concept-bearing commands

## Summary

`dva --help` and the flagship lifecycle commands carry excellent Long help (`up` explains
default_plan resolution and even leading-`--` semantics), but 21 of the commands in the tree
ship only a `Short:` line. The worst gap is `dva run --help`: it prints one sentence plus flags, while the
concepts a first-time user needs — what an *interaction* is, why the `run` prefix can be
omitted, how subcommands and `--project` targeting resolve — live only in USAGE.md. The stated
product goal is that a user (human or LLM) can learn the tool from the CLI alone; these
commands break that.

## Problem

A recursive walk of `rootCmd` (in-package test that prints every command whose `Long` is
empty) finds **21** Short-only commands, not the five a `grep -n "Long:" internal/cli/*.go`
sweep suggests — grep tells you which *file* mentions `Long:`, not which command in it carries
one. `internal/cli/skill.go` and `internal/cli/ssh.go` define eleven commands between them and
none has a Long; `internal/cli/validate.go` defines `validateCmd` with no Long at all.

Concept-bearing (the reason this task exists — each hides a `dva.yml` section or a resolution
rule that only USAGE.md explains):

1. `runCmd` (`run.go`) — the most concept-heavy command has the thinnest help: what an
   *interaction* is, why the `run` prefix is optional, subcommand/`default_args` resolution,
   and `--project` targeting are all absent.
2. `lsCmd` (`list.go`) — does not say where the listed commands come from (`interaction:` in
   dva.yml, imported subprojects) or how it relates to `manifest`.
3. `statusCmd` (`status.go`) — does not say what is inspected or how plans scope it.
4. `provisionCmd` (`provision.go`) — "Execute the provisioning steps" with no pointer to the
   `provision:` section semantics.
5. `validateCmd` (`validate.go:129`) — reached as both `dva validate` and `dva config
   validate`; neither explains what is checked or what `--strict` / `--fix` change.
   Note `validate_alias.go:9` copies `validateCmd.Long` **by value inside `init()`**, so the
   Long must be set in the struct literal — assigning it from a later `init()` would leave the
   top-level alias empty while `dva config validate` looks fixed.
6. `manifestCmd` (`manifest.go`) — the LLM-facing surface; needs to say what the manifest
   contains and that it is the machine-readable twin of `ls`.
7. `skillCmd` and `sshCmd` — group parents whose `--help` currently lists subcommands with no
   statement of what the group is for (which AI runtimes; which agent container).

Mechanical (no concept to explain, but the regression test below covers them, which is what
turns this from an S into an M):

8. `versionCmd`, `configCmd`, `consoleCmd`, `console start`.
9. `skill install`, `skill status`, `skill uninstall`, `skill backup`, `skill backup list`.
10. `ssh up`, `ssh down`, `ssh status`.

Full missing list, as printed by the tree walk: `config`, `config validate`, `console`,
`console start`, `ls`, `manifest`, `provision`, `run`, `skill`, `skill backup`,
`skill backup list`, `skill install`, `skill status`, `skill uninstall`, `ssh`, `ssh down`,
`ssh status`, `ssh up`, `status`, `validate`, `version`.

## Completion Criteria

- [x] `dva run --help` explains: interaction commands come from `dva.yml`'s `interaction:` section, the `run` prefix is optional for non-reserved names, subcommand/default_args resolution in one sentence, and `--project` targeting; content agrees with USAGE.md rather than duplicating its detail | verify: `go test ./internal/cli -count=1`
- [x] `lsCmd`, `statusCmd`, `provisionCmd`, `manifestCmd`, `validateCmd` each carry a Long that states what the command reads, what it prints, and where the authoritative reference lives (USAGE.md section name); `validateCmd`'s Long is set in its struct literal so `dva validate` and `dva config validate` both show it | verify: `go test ./internal/cli -count=1`
- [x] `skillCmd` and `sshCmd` carry a Long stating what the group manages, and every one of their subcommands plus `configCmd`, `consoleCmd`, `console start`, `versionCmd` has at least a one-line Long | verify: `go test ./internal/cli -count=1`
- [x] A regression test walks the rootCmd tree recursively (not just direct children) and asserts every command has a non-empty Long, so future commands cannot regress to Short-only; cobra adds `help`/`completion` at Execute time so they do not appear in the walk and need no special-casing — if the test is written to run after `Execute`, exclude them explicitly | verify: `go test ./internal/cli -count=1`
- [x] Repository gates pass | verify: `make lint && make test && make doc-check`

## Non-goals

- No `Example:` field work — TASK-269 owns promoting examples into cobra Example fields.
- No USAGE.md restructuring; Long text links to it, it stays canonical.
- No flag additions or behavior changes on any command.

## Evidence

카드 인벤토리를 먼저 실측으로 검증했다. `rootCmd`를 재귀 순회해 `Long`이 빈 커맨드를
출력하는 임시 테스트를 돌린 결과 정확히 21개가 나왔고, 카드가 나열한 목록과 한 글자도
다르지 않았다. 카드에서 틀린 것으로 밝혀진 항목은 없다.

한 가지 카드의 전제가 어긋났다: 이 저장소의 기존 `Long` 필드는 **영어**다(`upCmd`,
`configShowCmd`, `consoleInjectCmd` 확인). 한국어가 관례라는 가정 대신 실제 표면에 맞춰
영어로 작성했다 — 도움말 표면 내부의 일관성이 우선이다.

### 사실 정확성 검증

Long 본문의 주장은 전부 코드/USAGE.md에 대조했다. 서술만 그럴듯하고 코드와 어긋나는
도움말은 없는 도움말보다 나쁘기 때문이다.

- `run` — `default_args` 상속 서술("자식이 `command`/`script`/`script_file`/`steps`를
  선언하면 부모 `default_args`는 비워진다")이 USAGE.md:1178 표와 정확히 일치.
  `project:cmd` 네임스페이스 형태는 `run.go:50`의 `strings.SplitN(cmdName, ":", 2)`로 실재.
- `ls` — `(unreachable: ...)` 표기(`list.go:88`)와 plans 별도 섹션 출력(`list.go:114-117`) 확인.
- `provision` — 프로필 해석 순서 서술이 `resolveProvisionProfile`의 주석
  "Priority: exact match → default_profile alias → single-profile auto → error"(`:432`)와 일치.
- `validate` — `--strict`가 승격하는 세 종류(drift/semantic/collision)가 `validate.go:190`의
  실제 조건과 일치. `--fix`의 devcontainer.json 생성은 `:211-217`로 확인.
- `version` — Version/Commit/BuildDate가 Makefile:35의 `-ldflags -X`로 주입됨을 확인.
- `console start` — `dva_inject`/`dva_reload`/`dva_clear`, `DVA_SHELL`/`DVA_PROMPT_TEXT`
  모두 `console.go`에 실재.
- `config` — 나열한 5개 하위 커맨드(show/validate/init/docs/migrate)가 실제 `AddCommand` 집합과 일치.
- `skill` — 나열한 런타임 6종이 `install.go:35-40`의 상수 집합과 정확히 일치.
- `ssh` — 기본 이미지 `whilp/ssh-agent`(`ssh.go:20`), `ssh.agent_image` 오버라이드(`:45-47`),
  `status`의 `docker inspect --format {{.State.Status}}`(`:93`) 확인.
- USAGE.md 교차 참조 8건 전부 실재하는 제목을 가리킴("run" 236, "ls" 251, "manifest" 260,
  "named execution entry" 315, "ssh up" 582, "provision" 601, "config validate" 652,
  "interaction (예약어와 훅)" 1039, "interaction.subcommands (`default_args` 상속)" 1178).
  각 Long은 USAGE.md 내용을 복제하지 않고 권위 있는 절 이름만 지목한다.

### validate_alias 함정

`validateCmd.Long`은 구조체 리터럴에 직접 설정하고 그 이유를 주석으로 남겼다.
`validate_alias.go:9`가 `init()` 안에서 Long을 **값으로 복사**하므로, 나중 `init()`에서
대입하면 `dva config validate`만 고쳐진 것처럼 보이고 최상위 `dva validate` 별칭은 빈 채로
남는다. 빌드된 바이너리로 두 표면의 `--help` 출력을 직접 비교해 byte-identical임을 확인했다.

### 회귀 테스트

`TestAllCommandsHaveLongHelp`(`internal/cli/long_help_test.go`). `Execute()`를 호출하지 않고
순회하므로 cobra가 Execute 시점에 붙이는 `help`/`completion`은 애초에 나타나지 않지만,
이름으로도 명시적으로 제외한다 — `rootCmd`는 테스트 바이너리 전체가 공유하는 패키지 수준
변수이고 같은 패키지의 다른 테스트가 `rootCmd.ExecuteC()`를 호출해 두 커맨드를 프로세스가
끝날 때까지 영구히 붙여 놓기 때문이다. 테스트 실행 순서에 결과가 의존하지 않게 하는 방어다.

리뷰에서 한 가지를 보강했다: 순회가 `rootCmd`의 **자식**부터 시작하므로 `rootCmd` 자신의
Long은 검사되지 않았다. "모든 커맨드"에서 사용자가 가장 먼저 보는 커맨드가 빠지는 구멍이라
직접 단언을 추가했다.

## Gate results

워크트리 `claude__mst__chore__task268-long-help`에서 실행:

| gate | 결과 |
| --- | --- |
| `make lint` | exit 0 (gofmt -s 340 files, go vet clean, 0 issues) |
| `make test` | exit 0 (internal/cli 78.4%) |
| `make doc-check` | exit 0 (doc-check / cilabels / flowcheck 모두 OK) |
| `make build` | exit 0, `make generate` 부수 변경 없음 |

알려진 비차단 사항: `internal/cli/validate.go`는 이 변경 **이전에** 이미 저장소 파일 크기 훅
임계치(500)를 넘어 있었고(829줄), 이번 변경은 15줄만 더했다. 파일 분할은 이 카드의 범위
밖("no behavior changes")이라 손대지 않았다.
