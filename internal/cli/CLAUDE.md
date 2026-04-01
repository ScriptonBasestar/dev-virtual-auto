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
| `core` | ls, run, show, validate | list.go, run.go, show.go |
| `project` | init, version, doctor | init.go, doctor.go |
| `lifecycle` | compose, stack, app | compose.go, stack.go |
| `integration` | kubectl, infra, ssh | kubectl.go, infra.go, ssh.go |

## Dynamic Commands

`dva.yml`의 `interaction:` 섹션 키가 동적 서브커맨드로 등록됨.
`isTopLevelCommand()` → `config.IsReservedCommand()`로 충돌 방지.

## Embedded Files

`make generate`가 생성한 `*.txt` 파일들이 `//go:embed`로 포함됨:
- `library_reference.txt` — `ai-docs` 명령어용
- `improve_*.txt` — `config improve` 워크플로우용

## Naming Convention

파일명 = 커맨드명 (예: `compose.go` → `dva compose`, `stack.go` → `dva stack app`).
테스트는 `*_test.go`, 복잡한 플래그 테스트는 `*_extra_test.go`.

## Output Formatting

`--json` 플래그 → `output.go`의 JSON 포맷터 사용 (LLM 파이프라인용).
