# DVA (Dev Virtual Auto)

개발 환경 오케스트레이터 — `dva.yml` 하나로 Docker Compose, Kubernetes, Helm, 로컬 프로세스 등을 통합 관리.

**제품 특성**: DVA는 동작을 코드가 아니라 설정으로 정하는 개발환경 *구성* 도구입니다. 실행 방식은 `dva.yml`에서 자유롭게 선언하며, 기본 실행과 개발용(hot-reload) 실행을 옵트인으로 구분합니다 — 무엇이 개발용이고 무엇이 배포용인지는 도구가 아니라 설정이 결정합니다.

## Install

DVA는 Go toolchain에서 직접 설치하거나 로컬 빌드로 사용합니다.

```bash
# From source
go install github.com/ScriptonBasestar/dva/cmd/dva@latest

# Or build locally
make build
./bin/dva version
```

## Quick Start

프로젝트 루트에 `dva.yml` 생성:

```yaml
version: "0.1.44"

stack:
  compose:
    default_runner: compose
    order: 10
    runners:
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
dva ls              # 사용 가능한 커맨드 목록
dva shell           # = dva run shell → docker compose run app /bin/bash
dva test            # = dva run test → docker compose run app bundle exec rspec
dva up              # stack 전체 시작 (compose up -d --wait 등)
dva down            # stack 전체 중지
dva validate        # dva.yml 스키마 검증
dva manifest        # LLM용 전체 커맨드 매니페스트 출력
```

## Commands

```bash
# Lifecycle (named plans are the primary interface)
dva up local-dev           # named plan 시작
dva down local-dev         # named plan teardown
dva status local-dev       # named plan 상태

# Low-level escape hatches
dva stack up compose       # 특정 stack 엔트리 직접 시작
dva compose ps             # raw Docker Compose passthrough

# Applications
dva app ls                 # 앱 목록 (상태, 포트, PID)
dva app up                 # 전체 앱 시작 (의존성 순서)
dva app up api --dev       # dev 모드 (hot-reload)
dva app down               # 전체 앱 중지

# Legacy compatibility (deprecated surfaces may still exist)
dva up                     # plan이 하나면 기본 plan, 없으면 legacy 전체 stack

# Interaction
dva ls                     # 사용 가능한 커맨드 목록
dva shell                  # = dva run shell (run 생략 가능)

# Utilities
dva status                 # 워크스페이스 상태 확인
dva show                   # 설정 요약
dva validate               # dva.yml 스키마 + 시맨틱 검증 (`dva config validate`도 지원)
dva provision              # 프로비저닝 실행
dva config docs            # AI 에이전트 가이드(CLAUDE.md) 생성
dva doctor                 # 환경 사전조건 진단
```

전체 커맨드 레퍼런스: **[USAGE.md](USAGE.md)**

## Configuration

`dva.yml`은 재사용 가능한 실행 대상과 사용자가 호출하는 명령을 선언합니다.

### Stack (인프라 오케스트레이션)

`stack:` 섹션에서 여러 플러그인을 `order` 순서대로 실행합니다:

```yaml
stack:
  compose:
    default_runner: compose
    order: 10
    runners:
      compose:
        files: [docker-compose.yml]
        project_name: myapp
  kubectl:
    order: 20
    namespace: myapp-dev
  my-staging:
    default_runner: compose
    order: 30
    runners:
      compose:
        files: [docker-compose.staging.yml]
```

지원 플러그인: `compose`, `kubectl`, `helm`, `kustomize`, `tilt`, `skaffold`, `podman-compose`, `process`, `script`, `docker`, `vagrant`, `sam`, `serverless`, `multipass`

### Applications (앱 프로세스 관리)

`applications:` 섹션에서 네이티브/Docker 앱 프로세스를 정의하고 `dva app`으로 관리합니다:

```yaml
applications:
  api:
    description: "REST API server"
    port: 11200
    depends_on: []
    run:
      native: "cargo run --release -p api-server"
      docker: { service: api-rs, profile: rust }
    dev: "cargo watch -x 'run -p api-server'"
    health:
      type: http
      url: "http://localhost:11200/health"
```

> `run`은 기본 실행 명령, `dev`는 hot-reload 명령입니다. `dva app up`은 `run`을 실행하고, `dva app up <app> --dev`가 `dev`를 실행합니다 — 개발 모드는 **옵트인**이며 `dva app up`이 자동으로 dev로 뜨지 않습니다. 무엇을 dev/prod로 볼지는 이 설정이 정합니다. (별도의 `dva dev` 명령은 없습니다.)

### 기타 설정

- **Modes** (`--mode/-M`): 운영 모드별 compose profiles + 서비스 필터 + 환경변수 + stack 엔트리 필터 + 앱 전략
- **Environments** (`--env/-E`): 환경변수 프리셋 + stack 엔트리 필터
- **Tags** (`--tags/-T`): 태그 기반 특정 서비스/앱 그룹 필터링 (`--tag` 별칭 지원)
- **Health Checks**: 비-compose 서비스 상태 확인 및 자동 시작
- **Subprojects**: 모노레포 서브프로젝트 참조 (`dva api:test`)
- **Modules**: `.sb/dva/*.yml` 파일로 설정 분리
- **Override**: `dva.override.yml`로 로컬 오버라이드 (필드 레벨 deep merge)

상세 설정 가이드: **[USAGE.md](USAGE.md)**

## LLM Integration

```bash
am run dva-discover          # 프로젝트 분석 및 옵션 탐색
am run dva-improve           # AI로 dva.yml 자동 생성/개선
am run dva-diagnose          # 에러 분석 및 설정 자동 수정
dva config docs              # AI 에이전트 가이드(CLAUDE.md) 생성
dva manifest                 # 구조화된 커맨드 매니페스트
dva config show -f yaml      # 스키마 키를 보존한 병합 최종 설정 출력 (JSON도 지원)
```

AI 스킬은 `skills/`에 단일 소스로 두고 `make generate`(→ `tools/skillgen`)로 각 플랫폼에 투영합니다.

- **`claude-plugin/`**: Claude Code 플러그인 (`claude --plugin-dir ./claude-plugin`)
- **Antigravity**: `.agents/skills/dva/SKILL.md`
- **OpenCode**: `.opencode/skills/dva/SKILL.md`
- **Cursor**: `.cursor/rules/dva.mdc`
- **Codex**: `AGENTS.md` (자동 생성 `skills:auto` 섹션)

## Documentation

- [PRODUCT.md](PRODUCT.md) — 제품 가치, 대상 사용자, 현재 범위
- [SOUL.md](SOUL.md) — 변하지 않는 설계 철학
- [ARCHITECTURE.md](ARCHITECTURE.md) — 시스템 경계와 데이터 흐름
- [USAGE.md](USAGE.md) — 명령과 설정 레퍼런스

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
