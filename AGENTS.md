# AGENTS.md — DVA (Docker Virtual Auto)

## Overview

DVA는 Docker Compose / Kubernetes CLI 래퍼입니다. `dva.yml`에 정의된 명령어를 간단하게 실행합니다.

## Architecture

```text
cmd/dva/main.go            → Entry point
internal/cli/              → Cobra commands (root, run, compose, ls, etc.)
internal/config/           → dva.yml loading, env interpolation, schema validation
internal/runner/           → Execution strategies (DockerCompose, Kubectl, Local)
internal/exec/             → Process execution (syscall.Exec, subprocess)
```

## Key Flows

### Command Execution

1. `cli/root.go`: Dynamic routing — if arg is not a built-in command but exists in `interaction`, prepend `run`
2. `cli/run.go`: Uses `InteractionTree.Find()` to resolve command from config
3. `runner/runner.go`: Factory creates `DockerComposeRunner` (service:), `KubectlRunner` (pod:), or `LocalRunner`
4. `exec/exec.go`: `ExecReplace` (syscall.Exec) replaces process; `ExecSubprocess` spawns child

### Config Loading

1. Walk up from CWD to find `dva.yml` (or use `$DVA_FILE`)
2. Merge `.dva/*.yml` modules
3. Merge `dva.override.yml`
4. Validate against embedded `schema.json`

## File Map

| Task | Files |
|------|-------|
| Add new CLI command | `internal/cli/` + register in `root.go` init() |
| Add new runner | `internal/runner/` + update `NewRunner()` factory |
| Modify config schema | `internal/config/schema.json` + `config.go` structs |
| Fix env interpolation | `internal/config/environment.go` |
| Modify compose behavior | `internal/runner/docker_compose.go` |

## Build & Test

```bash
make build   # → ./bin/dva
make test    # go test -race -cover ./...
go build -o dva ./cmd/dva  # direct build
```
