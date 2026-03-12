# DVA (Docker Virtual Auto)

Docker Compose / Kubernetes CLI wrapper — `dva.yml` 설정 파일로 복잡한 명령어를 간단하게.

> Hip CLI의 Go 재작성 버전 (v10+).

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

프로젝트 루트에 `dva.yml` 생성:

```yaml
version: "0.1.0"

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
| `dva ls [-f json, yaml] [-d]` | List available commands |
| `dva init [-p, --prompt]` | Scaffold dva.yml or generate LLM prompt |
| `dva compose ARGS` | Pass-through to docker compose |
| `dva up [SERVICE]` | Start services (default: -d --wait) |
| `dva down` | Stop and remove containers |
| `dva stop [SERVICE]` | Stop services |
| `dva build [SERVICE]` | Build images |
| `dva clean [-v] [-i]` | Full cleanup |
| `dva provision [PROFILE]` | Execute provision scripts |
| `dva validate` | Validate config schema |
| `dva manifest [-f json, yaml]` | Output command manifest |
| `dva ktl ARGS` | Pass-through to kubectl |
| `dva ssh up/down/status` | Manage SSH agent container |
| `dva infra up/down/update SVC` | Manage infra services |
| `dva console start/inject` | Shell integration |
| `dva migrate` | Generate migration guide |
| `dva version` | Show version |
| `dva completion bash/zsh/fish` | Generate shell completions |

## LLM Integration (Agent Skills & Plugins)

DVA is designed to work seamlessly with LLM environments (Cursor, Claude Desktop, Antigravity) to act as a **tool provider**:

### Prompt-based Agent Skills & Plugins
When working in agents like **Claude**, **Cursor**, or **Antigravity**, you can use our pre-built instruction skills:
- **`claude-plugin/`**: We packaged the DVA skill specifically for Anthropic's Claude Code CLI. Developers can mount this via `claude --plugin-dir ./claude-plugin` to let Claude automatically discover the `skills/dva.md` definition.
- **Cursor**: Copy `.cursor/rules/dva.mdc` to enforce DVA usage.
- **Antigravity**: Copy `skills/dva/SKILL.md` to your workspace skills directory.
## Configuration

### Features

- **Module system**: `.dva/*.yml` 파일로 설정 분리
- **Override**: `dva.override.yml`로 로컬 설정 오버라이드
- **Environment interpolation**: `$VAR` / `${VAR}` 지원
- **Special variables**: `DVA_OS`, `DVA_WORK_DIR_REL_PATH`, `DVA_CURRENT_USER`
- **env_file**: `.env` 파일 로딩 지원

## AI & LLM Integration (DVA Auto-Config)

다른 외부 프로젝트에서 DVA를 강력하게 활용하고 싶다면, LLM이나 자체 에이전트(Cursor, Claude 등)에게 프로젝트 구조를 분석하게 한 뒤 `dva.yml`을 자동으로 생성하게 할 수 있습니다.

`dva init --prompt` (또는 `-p`) 명령어를 사용하면 현재 디렉토리 구조(Dockerfile, Makefile, package.json 유무 등)를 사전 탐색하여 **LLM에게 전달하기 가장 최적화된 프롬프트**를 클립보드로 복사하거나 터미널에 출력합니다. 

```bash
# 터미널에서 실행하여 AI에게 전달할 프롬프트 텍스트 확보
dva init --prompt
```

출력된 프롬프트를 복사하여 AI 에이전트에게 전달하면, 해당 프로젝트에 완벽하게 호환되는 `dva.yml` 템플릿을 자동으로 작성해 줍니다. 스스로 `dva validate` 까지 거치도록 프롬프트 구조가 설계되어 있어 문법 오류를 최소화합니다.

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
