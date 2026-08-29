# DVA (Dev Virtual Auto)

개발 환경 오케스트레이터 — `dva.yml` 하나로 Docker Compose, Kubernetes, Helm, 로컬 프로세스 등을 통합 관리.

**제품 특성**: DVA는 동작을 코드가 아니라 설정으로 정하는 개발환경 *구성* 도구입니다. 실행 방식은 `dva.yml`에서 자유롭게 선언하고, 어떤 조합을 띄울지는 **named plan**으로 고릅니다 — 기본 실행과 hot-reload 실행처럼 서로 다른 실행 방식은 각각 엔트리로 선언한 뒤 plan이 선택합니다. 무엇이 개발용이고 무엇이 배포용인지는 도구가 아니라 설정이 결정합니다.

## Install

DVA는 Go toolchain에서 직접 설치하거나 로컬 빌드로 사용합니다.

```bash
# From source
go install github.com/ScriptonBasestar/dva/cmd/dva@latest

# Or build locally
make build
./bin/dva version
```

`make install`은 DVA 바이너리만 설치합니다. 내장된 `dva`, `dva-config` 스킬은
AI 없이 별도 설치합니다:

```bash
dva skill install                         # 사용자 범위, 지원 런타임 전체
dva skill install --runtime codex,claude-code
dva skill install --scope project         # 현재 프로젝트에만 설치
dva skill status --json
dva skill backup list --runtime codex     # 보존된 takeover backup ID 조회
```

충돌 방지·삭제 소유권·런타임별 경로는 [USAGE.md의 스킬 설치](USAGE.md#ai-스킬-설치)를
참조하세요.

## Quick Start

프로젝트 루트에 `dva.yml` 생성:

```yaml
version: "0.1.44"

stack:
  compose:
    default_runner: compose
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
dva up              # 이 예시는 plans가 없어 stack 전체 시작 (compose up -d --wait 등)
dva down            # 이 예시는 plans가 없어 stack 전체 teardown
dva validate        # dva.yml 스키마 검증
dva manifest        # LLM용 전체 커맨드 매니페스트 출력
```

## Commands

```bash
# Lifecycle — 동사는 전부 plan 기준입니다
dva up local-dev           # named plan 시작
dva down local-dev         # named plan teardown
dva stop local-dev         # 컨테이너 유지한 채 정지
dva restart local-dev      # 재시작
dva build local-dev        # plan 엔트리 빌드
dva logs local-dev         # plan 엔트리 로그
dva status local-dev       # named plan 상태

dva up                     # default_plan 또는 유일한 plan 시작

# 파괴적 teardown (확인 프롬프트, --force로 생략)
dva down local-dev -v      # 볼륨까지 제거
dva down local-dev --purge # 볼륨 + 이미지 + provision 마커까지 제거

# Escape hatch
dva compose ps             # raw Docker Compose passthrough

# Interaction
dva ls                     # 사용 가능한 커맨드 목록
dva shell                  # = dva run shell (run 생략 가능)

# Utilities
dva status                 # 기본 plan 상태; plans가 없을 때 워크스페이스 상태
dva show                   # 설정 요약
dva validate               # dva.yml 스키마 + 시맨틱 검증 (`dva config validate`도 지원)
dva provision              # 프로비저닝 실행
dva config docs            # AI 에이전트 가이드(CLAUDE.md) 생성
dva doctor                 # 환경 사전조건 진단
```

이름 없는 `up`/`down`/`stop`/`restart`/`build`/`logs`/`status`는 명시된
`default_plan`을 선택하고, plan이 하나뿐이면 그 plan을 자동 선택합니다. 여러 plan이 있는데
`default_plan`이 없으면 이름을 요구하며, plan이 전혀 없을 때만 lifecycle 동사는 기존
whole-stack 경로를 사용합니다 (`status`는 워크스페이스 전체를 조회).

전체 커맨드 레퍼런스: **[USAGE.md](USAGE.md)**

## Configuration

`dva.yml`은 재사용 가능한 실행 대상과 사용자가 호출하는 명령을 선언합니다.

### Stack (인프라 오케스트레이션)

`stack:` 섹션은 실행 가능한 엔트리를 **선언**합니다. 무엇을 어떤 순서로 실행할지는
`plans:`가 정합니다:

```yaml
stack:
  compose:
    default_runner: compose
    runners:
      compose:
        files: [docker-compose.yml]
        project_name: myapp
  kubectl:
    namespace: myapp-dev
  my-staging:
    default_runner: compose
    runners:
      compose:
        files: [docker-compose.staging.yml]
```

> 엔트리에 `order:`를 직접 다는 legacy 형식은 plan 경로에서 읽히지 않습니다. 실행되는 것은
> `plans.*.entries[].order`이며, `dva config migrate`가 변환합니다.

지원 플러그인: `compose`, `kubectl`, `helm`, `kustomize`, `tilt`, `skaffold`, `podman-compose`, `process`, `script`, `docker`, `vagrant`, `sam`, `serverless`, `multipass`

### 앱 프로세스 (`native` 러너)

앱 프로세스도 stack 엔트리입니다 — `native` 러너를 쓰는 엔트리 하나가 앱 하나입니다.
전용 섹션이나 전용 명령은 없습니다:

```yaml
stack:
  api:
    description: "REST API server"
    default_runner: native
    runners:
      native:
        dir: services/api
        build: "cargo build --release -p api-server"
        run: "cargo run --release -p api-server"
        env:
          RUST_LOG: debug
    health_checks:
      api:
        type: http
        url: "http://localhost:11200/health"
```

> 엔트리는 `run` 명령 **하나**를 선언합니다. hot-reload처럼 실행 방식이 다르면 별도
> 엔트리(`api-watch` 등)로 선언하고 plan이 고릅니다 — 어느 쪽이 개발용인지는 도구가
> 아니라 plan이 정합니다. `--dev` 플래그는 없습니다.

### Plans (실행 가능한 이름)

`plans:` 섹션이 stack 엔트리·환경·site를 묶어 이름을 붙입니다. 모든 lifecycle 동사는
이 이름을 받습니다:

```yaml
plans:
  local-dev:
    environment: dev
    entries:
      - name: compose
        order: 10
      - name: api
        order: 20
```

```bash
dva up local-dev
dva logs local-dev api      # 엔트리 하나만
dva down local-dev --purge
```

### 기타 설정

- **Modes** (`--mode/-M`): 런타임 전략 선택 — compose profiles + 서비스 필터 + 환경변수 + stack 엔트리 필터 (dev 전용 도구이며 stg/prd 환경 개념은 없습니다)
- **Environments** (`--env/-E`): 환경변수 프리셋 + stack 엔트리 필터
- **Tags** (`--tags/-T`): 태그 기반 서비스/엔트리 그룹 필터링 (`--tag` 별칭 지원)
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

AI 스킬의 정본은 `skills/` 하나입니다. `make generate`는 DVA 소스 checkout 안에서만
Claude Code 플러그인 내부의 skills symlink, Antigravity·OpenCode용 symlink,
Cursor rule, Codex 호환 `AGENTS.md` 섹션을 만듭니다. 사용자나 다른 프로젝트에 설치하는
명령이 아닙니다. 정확한 checkout 산출물은 [skills target 표](skills/README.md#targets)를
참조하세요.

설치된 바이너리의 `dva skill install`은 내장된 `dva`와 `dva-config`를 선택한 runtime의
user/project discovery path에 복사합니다. Claude Code, Codex, OpenCode, Grok,
Antigravity, Agent Mesh의 정확한 설치 경로와 공유 경로 규칙은 [AI 스킬 설치](USAGE.md#ai-스킬-설치)를
참조하세요. Agent Mesh에는 같은 스킬을 flat Markdown으로 변환해 설치합니다.
공유 skill root는 어느 installer의 소유도 아니며, DVA는 per-skill XDG claim으로 `dva`와
`dva-config`만 표시합니다. receipt 없는 동명 충돌은 기본적으로 거부하고, 필요한 경우에만
`dva skill install --takeover`가 검증 가능한 백업을 만든 뒤 인수합니다. 복원은 자동이 아니라
`dva skill uninstall --restore-takeover-backup`로 명시합니다. 두 옵션 모두 변경 범위를 제한하도록
명시적 `--runtime`을 요구합니다.

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
