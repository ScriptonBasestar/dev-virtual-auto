# AGENTS.md — DVA (Dev Virtual Auto)

## Overview

DVA는 개발 환경 오케스트레이터입니다. `dva.yml`의 `stack:` 섹션에 정의된 플러그인들을 `order` 순서대로 실행하며, `interaction:` 섹션의 커맨드를 간단하게 실행합니다.

## Architecture

```text
cmd/dva/main.go                → Entry point
internal/cli/                  → Cobra commands
  root.go                      → Dynamic routing (interaction → run)
  stack.go                     → dva stack up/stop/down/status/log
  app.go                       → dva app ls/up/stop/down/build/restart/log
  compose.go                   → upCmd/downCmd/stopCmd etc. (stack + app 통합 실행)
  run.go, ls.go, show.go       → Core commands
  validate.go                  → dva config validate (schema + semantic warnings)
internal/config/               → dva.yml loading, env interpolation, schema validation
  config.go                    → Config struct (Stack, Interaction, Modes, Applications, etc.)
  lifecycle.go                 → LifecycleEntry, plugin config types (Compose, Kubectl, Helm, ...)
  lifecycle_helpers.go         → SortedStack(), PrimaryComposeEntry(), ComposeEntries(), etc.
  merge.go                     → Field-level deep merge (modules/override 적용)
  validate_warnings.go         → 13 semantic warning checks (non-fatal)
  reserved.go                  → Reserved/restricted field definitions
internal/lifecycle/            → Stack + App orchestration
  orchestrator.go              → Stack Up/Down/Stop with tag/mode/env filtering
  app_manager.go               → App lifecycle (topo-sort, concurrent start, health, PID tracking)
  process.go                   → Process execution and signal handling
internal/runner/               → Interaction execution (DockerCompose, Kubectl, Local)
internal/exec/                 → Process execution (syscall.Exec, subprocess)
```

## Key Flows

### Stack Lifecycle (dva stack up/down)

1. `cli/stack.go`: Parses `--mode`, `--tags`, `--exclude-tags`, `--force`, `--no-wait` flags
2. `lifecycle/orchestrator.go`: `NewOrchestrator()` → `cfg.SortedStack()` (order 정렬)
3. 각 `LifecycleEntry`의 `DetectPlugin()`으로 플러그인 타입 결정
4. 플러그인별 Up/Down 실행 (compose → `docker compose up`, kubectl → `kubectl apply` 등)
5. Down은 역순 teardown (LIFO)

### App Lifecycle (dva app up/down)

1. `cli/app.go`: Parses `--dev`, `--docker` flags, resolves mode strategy
2. `lifecycle/app_manager.go`: `StartApps()` → `topoSortWaves()` (의존성 기반 웨이브)
3. 각 웨이브 내 앱들을 동시 시작 (goroutine)
4. Strategy 결정: mode 설정 → 글로벌 전략 → 앱 기본값
5. PID 파일로 프로세스 추적, `.sb/dva/logs/` 에 앱별 로그 저장

### Command Execution (dva run/shell/...)

1. `cli/root.go`: Dynamic routing — if arg is not a built-in command but exists in `interaction`, prepend `run`
2. `cli/run.go`: Uses `InteractionTree.Find()` to resolve command from config
3. `runner/runner.go`: Factory creates `DockerComposeRunner` (service:), `KubectlRunner` (pod:), or `LocalRunner`
4. `exec/exec.go`: `ExecReplace` (syscall.Exec) replaces process; `ExecSubprocess` spawns child

### Config Loading

1. Walk up from CWD to find `dva.yml` (or use `$DVA_FILE`)
2. Merge `.sb/dva/*.yml` modules
3. Merge `dva.override.yml`
4. Validate against embedded `schema.json`
5. Resolve plugin types from entry names (auto-inference)

### Stack Plugin Resolution (3가지 방식)

```yaml
# 1. Nested (legacy): 플러그인 서브키
compose:
  compose:
    files: [docker-compose.yml]

# 2. Flat + explicit plugin:
my-compose:
  plugin: compose
  files: [docker-compose.yml]

# 3. Flat + auto-inference (엔트리 이름 = 플러그인명)
compose:
  files: [docker-compose.yml]
```

### Config Merge (modules/override 적용)

1. `config/merge.go`: `mergeFrom()` — base config에 overlay 적용
2. Map 섹션 (stack, interaction, modes 등): key별 deep merge
3. List/Scalar: replace (나중 레이어 우선)
4. 구조적 필드 (`plugin`, `runner`): override 시 hard error
5. 상세 규칙: `docs/30-config-merge-semantics.md`

## File Map

| Task | Files |
|------|-------|
| Add new CLI command | `internal/cli/` + register in `root.go` init() |
| Add new stack plugin | `internal/config/lifecycle.go` (config type) + `internal/lifecycle/` (executor) |
| Add new runner | `internal/runner/` + update `NewRunner()` factory |
| Modify config schema | `internal/config/schema.json` + `config.go` structs |
| Fix env interpolation | `internal/config/environment.go` |
| Modify compose behavior | `internal/runner/docker_compose.go` |
| Stack orchestration | `internal/lifecycle/orchestrator.go` |
| App lifecycle | `internal/lifecycle/app_manager.go` |
| Config merge logic | `internal/config/merge.go` |
| Validation warnings | `internal/config/validate_warnings.go` |
| Stack CLI commands | `internal/cli/stack.go` |
| App CLI commands | `internal/cli/app.go` |

## Build & Test

```bash
make build   # → ./bin/dva
make test    # go test -race -cover ./...
```
