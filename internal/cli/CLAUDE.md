# internal/cli — DVA CLI Commands

Cobra 기반 CLI 명령어 구현. 전역 설정 로드 및 서브커맨드 등록.

## Global State

```go
var cfg     *config.Config       // dva.yml 파싱 결과
var env     *config.Environment  // 활성 환경 프로필
var debug   bool
var dryRun  bool
var jsonOutput bool
```

`PersistentPreRun`에서 로거 초기화, config 로드는 각 명령어에서 개별 수행.

## Command Groups

| Group ID | 설명 | 주요 파일 |
|---|---|---|
| `core` | ls, run, version | list.go, run.go, version.go |
| `project` | show, status, config, doctor | show.go, status.go, config_dump.go, doctor.go |
| `lifecycle` | up, down, stop, restart, build, logs | plan_lifecycle.go, compose.go, build.go, logs.go |
| `integration` | compose, ktl, ssh | compose.go, kubectl.go, ssh.go |
| `advanced` | manifest, console, provision, validate | manifest.go, console.go, provision.go, validate.go |

lifecycle 동사는 전부 plan(`<name>`) 기준 단일 세대입니다. `stack`/`app`/`infra`/`clean`은
제거됐고 (docs/43) `stack.go`·`app.go`·`infra.go`도 함께 사라졌습니다. 엔트리 부분 실행은
plan 선언으로 표현합니다.

## Dynamic Commands

`dva.yml`의 `interaction:` 섹션 키가 동적 서브커맨드로 등록됨.
`isTopLevelCommand()` → `config.IsReservedCommand()`로 충돌 방지.

## Embedded Files

`//go:embed`로 포함된 파일:
- `dva_guide_template.txt` — `ai-docs` 명령어용 (`ai_docs.go`)

`library_reference.txt`는 `make generate`가 생성하지만 **embed하지 않음** — am flow가
런타임에 `read_file`로 읽고, `internal/config/removed_keys_test.go`가 corpus로 사용.
Go 팩트(reserved commands, section order)는 `tools/libgen`이 `shared-guardrails.md`에 주입.

## Naming Convention

파일명 = 커맨드명 (예: `compose.go` → `dva compose`, `logs.go` → `dva logs`).
plan 경로를 공유하는 lifecycle 동사는 `plan_lifecycle.go`에 모입니다.
테스트는 `*_test.go`, 복잡한 플래그 테스트는 `*_extra_test.go`.

## Output Formatting

`--json` 플래그 → `output.go`의 JSON 포맷터 사용 (LLM 파이프라인용).
