# DVA (Docker Virtual Auto)

Docker Compose / Kubernetes CLI wrapper — `dva.yml` 설정 파일로 복잡한 명령어를 간단하게.

> Hip CLI의 Go 재작성 버전 (v10+). Ruby 원본: [ScriptonBasestar/hip](https://github.com/ScriptonBasestar/hip)

## Install

### Binary (추천)

```bash
# From source
go install github.com/ScriptonBasestar/dva/cmd/dva@latest

# Or build locally
make build
./bin/dva version
```

### From Release

```bash
curl -sL https://github.com/ScriptonBasestar/dva/releases/latest/download/dva_linux_amd64.tar.gz | tar xz
sudo mv dva /usr/local/bin/
```

## Quick Start

프로젝트 루트에 `dva.yml` (또는 기존 `hip.yml`) 생성:

```yaml
version: "10.0.0"

compose:
  files:
    - docker-compose.yml

interaction:
  shell:
    description: "Open shell in app container"
    service: app
    command: /bin/bash
  test:
    description: "Run tests"
    service: app
    command: bundle exec rspec
```

```bash
dva ls          # 사용 가능한 커맨드 목록
dva shell       # = dva run shell → docker compose run app /bin/bash
dva test        # = dva run test → docker compose run app bundle exec rspec
dva up          # docker compose up -d --wait
dva down        # docker compose down --remove-orphans
dva validate    # dva.yml 스키마 검증
dva manifest    # LLM용 전체 커맨드 매니페스트 출력
```

## Commands

| Command | Description |
|---------|-------------|
| `dva run CMD [ARGS]` | Run configured interaction command |
| `dva ls [-f json\|yaml] [-d]` | List available commands |
| `dva compose ARGS` | Pass-through to docker compose |
| `dva up [SERVICE]` | Start services (default: -d --wait) |
| `dva down` | Stop and remove containers |
| `dva stop [SERVICE]` | Stop services |
| `dva build [SERVICE]` | Build images |
| `dva clean [-v] [-i]` | Full cleanup |
| `dva provision [PROFILE]` | Execute provision scripts |
| `dva validate` | Validate config schema |
| `dva manifest [-f json\|yaml]` | Output command manifest |
| `dva ktl ARGS` | Pass-through to kubectl |
| `dva ssh up\|down\|status` | Manage SSH agent container |
| `dva infra up\|down\|update SVC` | Manage infra services |
| `dva console start\|inject` | Shell integration |
| `dva migrate` | Generate migration guide |
| `dva version` | Show version |
| `dva completion bash\|zsh\|fish` | Generate shell completions |

### Hip Backward Compatibility

- **Config files**: `hip.yml` is still supported (dva.yml takes priority)
- **Env vars**: `HIP_FILE` is supported (DVA_FILE takes priority)
- **Module dir**: `.hip/` is supported (`.dva/` takes priority)

## Configuration

### Features

- **Module system**: `.dva/*.yml` (or `.hip/*.yml`) 파일로 설정 분리
- **Override**: `dva.override.yml` (or `hip.override.yml`)로 로컬 설정 오버라이드
- **Environment interpolation**: `$VAR` / `${VAR}` 지원
- **Special variables**: `DVA_OS`, `DVA_WORK_DIR_REL_PATH`, `DVA_CURRENT_USER`
- **env_file**: `.env` 파일 로딩 지원

## Development

```bash
make build      # Build → ./bin/dva
make test       # Run tests
make lint       # Run linters
make fmt        # Format code
make clean      # Clean build artifacts
```

## License

MIT
