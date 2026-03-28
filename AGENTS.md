# AGENTS.md — DVA (Dev Virtual Auto)

## Overview

DVA는 개발 환경 오케스트레이터입니다. `dva.yml`의 `stack:` 섹션에 정의된 플러그인들을 `order` 순서대로 실행하며, `interaction:` 섹션의 커맨드를 간단하게 실행합니다.

## Architecture

```text
cmd/dva/main.go                → Entry point
internal/cli/                  → Cobra commands (root, run, compose, ls, etc.)
internal/config/               → dva.yml loading, env interpolation, schema validation
  config.go                    → Config struct (Stack, Interaction, Modes, etc.)
  lifecycle.go                 → LifecycleEntry, plugin config types (Compose, Kubectl, Helm, ...)
  lifecycle_helpers.go         → SortedStack(), PrimaryComposeEntry(), etc.
internal/lifecycle/            → Stack orchestrator (plugin execution by order)
  orchestrator.go              → Up/Down/Stop with tag/mode filtering
internal/runner/               → Interaction execution (DockerCompose, Kubectl, Local)
internal/exec/                 → Process execution (syscall.Exec, subprocess)
```

## Key Flows

### Stack Lifecycle (dva up/down)

1. `cli/up.go`: Parses `--mode`, `--tags`, `--exclude-tags` flags
2. `lifecycle/orchestrator.go`: `NewOrchestrator()` → `cfg.SortedStack()` (order 정렬)
3. 각 `LifecycleEntry`의 `DetectPlugin()`으로 플러그인 타입 결정
4. 플러그인별 Up/Down 실행 (compose → `docker compose up`, kubectl → `kubectl apply` 등)

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

## File Map

| Task | Files |
|------|-------|
| Add new CLI command | `internal/cli/` + register in `root.go` init() |
| Add new stack plugin | `internal/config/lifecycle.go` (config type) + `internal/lifecycle/` (executor) |
| Add new runner | `internal/runner/` + update `NewRunner()` factory |
| Modify config schema | `internal/config/schema.json` + `config.go` structs |
| Fix env interpolation | `internal/config/environment.go` |
| Modify compose behavior | `internal/runner/docker_compose.go` |
| Lifecycle orchestration | `internal/lifecycle/orchestrator.go` |

## Build & Test

```bash
make build   # → ./bin/dva
make test    # go test -race -cover ./...
go build -o dva ./cmd/dva  # direct build
```
