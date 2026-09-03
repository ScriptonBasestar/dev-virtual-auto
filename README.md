# DVA (Dev Virtual Auto)

개발 환경 오케스트레이터 — `dva.yml` 하나로 Docker Compose, Kubernetes, Helm, 로컬 프로세스 등을 통합 관리.

**제품 특성**: DVA는 동작을 코드가 아니라 설정으로 정하는 개발환경 *구성* 도구입니다. 실행 방식은 `dva.yml`에서 자유롭게 선언하고, 어떤 조합을 띄울지는 **named plan**으로 고릅니다 — 기본 실행과 hot-reload 실행처럼 서로 다른 실행 방식은 각각 엔트리로 선언한 뒤 plan이 선택합니다. 무엇이 개발용이고 무엇이 배포용인지는 도구가 아니라 설정이 결정합니다.

## Install

DVA는 Go toolchain에서 버전을 고정해 설치하는 방법을 권장합니다.

```bash
# Reproducible module install
go install github.com/ScriptonBasestar/dva/cmd/dva@v0.1.47

# Or build locally
make build
./bin/dva version
```

Go가 설치한 바이너리는 `GOBIN` 또는 `$(go env GOPATH)/bin`에 생성됩니다. 해당 디렉터리가
`PATH`에 없으면 추가한 뒤 `dva`를 실행하세요.

Go toolchain 없이 설치하려면 [v0.1.47 Release](https://github.com/ScriptonBasestar/dva/releases/tag/v0.1.47)에서
OS/architecture archive와 `checksums.txt`를 함께 내려받으세요. 자산 선택과 SHA-256 검증 절차는
[USAGE.md의 설치](USAGE.md#설치)를 따릅니다. 최신판을 자동 추적하려는 경우에만 `@latest`를
사용하세요.

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

`dva init`(= `dva config init`)이 프로젝트 루트에 `dva.yml`을 자동 감지로 스캐폴딩합니다.
`dva init -t node`처럼 템플릿(minimal, rails, node, python, go)을 지정할 수 있습니다.
생성 결과는 다음과 같은 형태입니다:

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

`dva --help`와 같은 다섯 그룹입니다. 이름과 한 줄 역할만 싣고, 플래그·동작 상세는
[USAGE.md](USAGE.md#command-quick-reference)를 참조하세요.

- **Core**: `run` — interaction 실행 (`dva shell` = `dva run shell`, run 생략 가능) · `ls` — 커맨드 목록 · `manifest` — LLM용 커맨드 매니페스트 · `version`
- **Project Management**: `show` — 설정 요약 · `config` — `init`/`docs`/`migrate`/`show`/`validate` 서브커맨드 (`dva init`은 top-level alias) · `doctor` — 환경 사전조건 진단
- **Lifecycle**: `up`/`down`/`stop`/`restart`/`status`/`logs`/`build` — 전부 `dva <verb> <plan>` 형태. `down --purge`는 확인 프롬프트를 거칩니다 (`--force`로 생략); `-v`만 붙인 경우는 프롬프트 없이 즉시 볼륨을 삭제합니다
- **Integration Tools**: `compose` — raw Docker Compose passthrough (escape hatch) · `ktl` — kubectl passthrough · `ssh` — SSH agent 컨테이너 관리
- **Advanced Utilities**: `console` — 셸 통합 · `provision` — 프로비저닝 실행 · `skill` — 내장 AI 스킬 설치 관리 · `validate` — dva.yml 검증 (`dva config validate`와 동일)

완전히 인자 없는 lifecycle 동사는 명시된 `default_plan`을, plan이 하나뿐이면 그 plan을
자동 선택합니다. 예외 경로와 `dva up --force`의 whole-stack 위험은
[무인자 lifecycle 선택](USAGE.md#무인자-lifecycle-선택)과
[라이프사이클 플래그](USAGE.md#라이프사이클-플래그)를 확인하세요.

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

지원 플러그인은 `compose`, `kubectl`, `helm`, `process` 등 3-tier 14종입니다 — 전체
tier 표는 [USAGE.md의 stack](USAGE.md#stack-선언-저장소)을 참조하세요.

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

`dva config docs`가 AI 에이전트 가이드(CLAUDE.md/AGENTS.md)를 생성하고, `dva manifest`가
자동화용 구조화 커맨드 매니페스트를 출력합니다. agent-mesh flow로는 `am run
dva-discover`/`dva-improve`(분석·개선)와 `am run dva-diagnose`(환경 점검·에러 분석)가
있습니다. `--json` 플래그 등 전체 통합 표면은
[USAGE.md](USAGE.md#llm-integration)를 참조하세요.

AI 스킬의 정본은 `skills/` 하나입니다. `make generate`는 DVA 소스 checkout 안에서만
생성물을 일괄 갱신합니다 — libgen(fact 블록), `library_reference.txt` 재조립,
flowgen(self-contained agent-mesh flow), skillgen(플랫폼별 스킬 아티팩트) — 사용자나 다른
프로젝트에 설치하는 명령이 아닙니다. checkout 산출물 상세는
[skills target 표](skills/README.md#targets)를 참조하세요.

설치된 바이너리의 `dva skill install`은 내장 `dva`·`dva-config` 스킬을 선택한 runtime의
user/project discovery path에 복사합니다. runtime별 경로, per-skill claim과 충돌 거부,
`--takeover` 백업·복원 규칙은 [AI 스킬 설치](USAGE.md#ai-스킬-설치)를 참조하세요.

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
