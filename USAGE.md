# DVA 사용 가이드

> DVA CLI 전체 커맨드 레퍼런스 및 설정 가이드.
> DVA는 개발 환경 오케스트레이터 — Docker Compose, Kubernetes, Helm 등 여러 인프라 도구를 `stack:` 파이프라인으로 통합 관리합니다.
> 빠른 시작은 [README.md](README.md) 참조.

## Global Flags

| Flag | Description |
|------|-------------|
| `--debug` | 디버그 로깅 활성화 |
| `--dry-run` | 실행 계획만 표시 (실제 실행하지 않음) |
| `--json` | JSON 출력 (LLM 최적화) |

## Commands

### Core Commands

| Command | Description |
|---------|-------------|
| `dva config init` | 현재 디렉토리에 `dva.yml` 생성 (`dva init` alias 지원) |
| `dva config discover` | 프로젝트를 분석해 가능한 DVA 옵션 후보 출력 |
| `dva run CMD [ARGS]` | `dva.yml`에 정의된 interaction 커맨드 실행 |
| `dva ls` | 사용 가능한 interaction 커맨드 목록 |
| `dva version` | 버전 표시 |

`dva run`은 생략 가능합니다. `dva shell`은 `dva run shell`과 동일합니다.
`namespace:command` 문법도 지원합니다 (예: `dva engine:test`).

#### init (config init)

```bash
dva config init                  # 자동 감지 기반 dva.yml 생성
dva init                         # 위와 동일 (backward compat alias)
dva config init -t node          # 템플릿 지정 (minimal, rails, node, python, go)
dva config init --recursive      # 서브프로젝트에도 dva.yml 생성
dva config init --devcontainer   # .devcontainer/devcontainer.json 포함 생성
dva config init --all            # 가능한 모든 기능 통합 활성화 (devcontainer 등)
```

생성 후 `dva config improve`로 AI 기반 최적화를 실행할 수 있습니다.

#### discover (config discover)

```bash
dva config discover          # JSON 형식으로 분석 결과 출력
dva config discover -f yaml  # YAML 형식으로 출력
```

`discover`는 `dva.yml`을 직접 생성하지 않고, 프로젝트에서 감지한 다음 후보를 정리해 보여줍니다.

- 감지된 compose 파일과 서비스
- package.json / Makefile 기반 실행 커맨드
- 앱 후보와 native/docker 실행 경로
- 추천 mode (`native`, `docker`, `hybrid`, `local-test` 등)
- 추천 environment (`dev`, `local-test` 등)

권장 흐름:

```bash
dva config discover   # 1단계: 옵션 분석
dva config improve    # 2단계: 최종 dva.yml 생성/개선
```

`improve`는 내부적으로 discovery 결과 리포트를 재사용합니다.

#### config improve

```bash
dva config improve                # Claude Code CLI로 개선 실행 (기본)
dva config improve --print        # 프롬프트만 stdout 출력 (수동 붙여넣기용)
dva config improve --docs-only    # CLAUDE.md/AGENTS.md만 갱신 (dva.yml 미변경)
dva config improve --rewrite      # dva.yml을 처음부터 재작성
dva config improve --interactive  # Claude Code 인터랙티브 모드 (세션 유지)
dva config improve --recursive    # 서브프로젝트 dva.yml도 함께 개선
dva config improve --model MODEL  # AI 모델 지정
dva config improve -v             # 상세 진행 표시
```

`--print`, `--docs-only`, `--interactive`는 상호 배타적입니다 (먼저 매칭된 것이 실행).
`--interactive`와 `--recursive`는 동시 사용 불가합니다.

#### run

```bash
dva run shell             # interaction 커맨드 실행
dva shell                 # 위와 동일 (run 생략)
dva run -e test           # --explain: --dry-run 별칭으로 실행 계획 표시
dva run -p 8080:80 web    # --publish: 포트 퍼블리시
dva run --project api test  # 서브프로젝트 커맨드 실행
dva api:test              # 위와 동일 (namespace 문법)
```

#### ls

```bash
dva ls                    # 테이블 형식
dva ls -f json            # JSON 출력
dva ls -f yaml            # YAML 출력
dva ls -d                 # 상세 정보 (runner type, service, command)
```

### Project Management

| Command | Description |
|---------|-------------|
| `dva show` | 설정 요약 (modes, environments, commands 등) |
| `dva status` | 워크스페이스 상태 (컨테이너, 서비스 상태) |
| `dva config show` | 최종 병합된 설정 출력 (modules + override 적용 후) |

```bash
dva show                  # 등록된 설정 전체 요약
dva show --json           # JSON 출력
dva status                # docker compose ps + 헬스체크 상태
dva status --json         # JSON 출력
dva config show           # JSON 형식 (기본)
dva config show -f yaml   # YAML 형식
```

### Lifecycle (Stack + App)

#### dva stack — 인프라 관리

`stack:` 섹션에 정의된 플러그인들을 `order` 순서대로 실행합니다.

| Command | Description |
|---------|-------------|
| `dva stack up [NAME...]` | stack 엔트리 시작 (order 순서) |
| `dva stack stop [NAME...]` | stack 엔트리 중지 (상태 보존) |
| `dva stack down [NAME...]` | stack 엔트리 중지 및 제거 (역순) |
| `dva stack status [NAME...]` | stack 엔트리 상태 표시 |
| `dva stack log [NAME]` | stack 엔트리 로그 조회 |

```bash
dva stack up                    # 전체 stack 시작 (order 순서)
dva stack up compose            # 특정 엔트리만
dva stack up --force            # 강제 재시작
dva stack up --no-wait          # 즉시 리턴
dva stack up -M backend         # 모드별 stack 필터 적용
dva stack up -T infra           # 태그 기반 필터
dva stack up --exclude-tags db  # 태그 제외
dva stack down -v               # + 볼륨 제거
dva stack status                # 전체 상태 표시
dva stack log compose           # compose 엔트리 로그
```

#### dva app — 애플리케이션 관리

`applications:` 섹션에 정의된 앱 프로세스를 관리합니다.

| Command | Description |
|---------|-------------|
| `dva app ls` | 앱 목록 (상태, 헬스, 포트, PID) |
| `dva app up [APP...]` | 앱 시작 (의존성 순서, 동시 실행) |
| `dva app stop [APP...]` | 앱 중지 (PID 보존, 빠른 재시작 가능) |
| `dva app down [APP...]` | 앱 중지 및 리소스 제거 |
| `dva app build [APP...]` | 앱 빌드 |
| `dva app restart [APP...]` | 앱 재시작 (stop + start) |
| `dva app log <APP>` | 앱 로그 (최근 100줄) |

```bash
dva app ls                      # 전체 앱 상태
dva app up                      # 전체 앱 시작 (native 기본)
dva app up api --dev            # dev 모드 (hot-reload)
dva app up -M docker            # docker 전략으로 시작
dva app build api               # 특정 앱 빌드
dva app build --docker          # docker 빌드
dva app stop api                # 앱 중지 (상태 보존)
dva app down                    # 전체 앱 중지 + 리소스 제거
dva app restart api --dev       # dev 모드로 재시작
dva app log api                 # 앱 로그 조회
```

#### dva up/down (통합 명령)

| Command | Description |
|---------|-------------|
| `dva up [SERVICE]` | stack + app 통합 시작 |
| `dva down` | stack + app 통합 중지 |
| `dva stop [SERVICE]` | 서비스 중지 (제거하지 않음) |
| `dva restart [SERVICE]` | 서비스 재시작 (stop + start) |
| `dva logs [SERVICE]` | 컨테이너 로그 보기 |
| `dva build [SERVICE]` | 이미지 빌드 |
| `dva clean` | 전체 정리 (containers, networks) |

```bash
dva up                    # stack + app 전체 시작
dva up -M backend         # 모드 적용
dva up -E staging         # 환경 프리셋 적용
dva up -T backend,ui      # 태그 필터 (--tag 별칭)
dva up --exclude-tags db  # 태그 제외 (--exclude-tag 별칭)
dva up -M backend -E staging  # 모드 + 환경 조합
```

#### clean

```bash
dva clean                 # containers + networks 제거
dva clean -v              # + 볼륨 제거 (데이터 손실 주의)
dva clean -i              # + 로컬 빌드 이미지 제거
dva clean -f              # 확인 프롬프트 스킵
```

#### --mode / --env / --tags 플래그

`--mode`(`-M`), `--env`(`-E`), `--tags`(`-T`), `--exclude-tags`는 `stack`, `app`, `up`, `down`, `stop`, `restart`에서 사용 가능합니다.

- `--mode`: `modes` 섹션에 정의된 운영 모드 적용 (compose profiles, 서비스 필터, 환경변수, 앱 전략)
- `--env`: `environments` 섹션에 정의된 환경변수 프리셋 적용 (stack 엔트리 필터 포함)
- `--tags`: `tags` 속성 기반으로 특정 컨테이너/앱 그룹만 선택 실행 (별칭: `--tag`)
- `--exclude-tags`: 특정 태그 그룹 배제 (별칭: `--exclude-tag`)

### Integration Tools

| Command | Description |
|---------|-------------|
| `dva compose ARGS` | docker compose 직접 패스스루 |
| `dva ktl ARGS` | kubectl 패스스루 |
| `dva infra up/down/update SVC` | 공유 인프라 서비스 관리 |
| `dva ssh up/down/status` | SSH agent 컨테이너 관리 |

### Advanced Utilities

| Command | Description |
|---------|-------------|
| `dva manifest` | LLM용 커맨드 매니페스트 출력 |
| `dva console start/inject` | 셸 통합 |
| `dva provision [PROFILE]` | 프로비저닝 스크립트 실행 |
| `dva config validate` | dva.yml 스키마 + 시맨틱 검증 |
| `dva doctor` | 환경 사전조건 및 설정 문제 진단 (`--fix` 자동 수정) |

#### provision

```bash
dva provision             # default 프로필 실행
dva provision setup       # 특정 프로필 실행
dva provision --list      # 사용 가능한 프로필 목록
```

#### doctor

```bash
dva doctor                # 환경 사전조건 체크 (Docker, compose 파일, .env 등)
dva doctor --fix          # 수정 가능한 문제 자동 해결
dva doctor --json         # JSON 출력
```

빌트인 체크 항목:
- Docker 소켓 권한 및 데몬 접근 가능 여부
- Compose 파일 존재 여부 및 project name 정합성
- `.env` 파일 존재 여부
- Stack 엔트리 참조 파일 존재 여부
- `.sb/dva/`가 `.gitignore`에 포함되어 있는지
- devcontainer 설정 시 `devcontainer.json` 존재 여부
- `dva.yml`의 `checks` 섹션에 정의된 사용자 커스텀 체크

#### config validate

```bash
dva config validate          # 스키마 + 시맨틱 검증
dva config validate --fix    # compose 파일 project name 불일치 자동 수정
dva config validate --strict # drift 경고 시에도 검증 실패 처리
```

스키마 검증 외에 13개 시맨틱 경고를 검사합니다:
- 중복 stack order, 다중 compose 엔트리 분할 권고
- `default_mode` 누락, 기본 모드에 무거운 인프라 포함 경고
- 미해결 환경변수 (`${MISSING_VAR}`), 비지원 셸 문법 감지
- 깊은 서브커맨드 중첩 (5단계 초과), 도달 불가능 커맨드
- 정규 섹션 순서 검증

## Configuration (`dva.yml`)

### 기본 구조

```yaml
version: "0.1.44"         # 최소 DVA 버전

stack:
  compose:                 # 엔트리 이름으로 플러그인 자동추론
    order: 10
    files: [docker-compose.yml]
    project_name: myproject

interaction:
  shell:
    description: "Open shell"
    service: app
    command: /bin/bash
```

### 설정 섹션 레퍼런스

정규 섹션 순서 (validate에서 검증):

| Section | Description |
|---------|-------------|
| `version` | 최소 DVA 버전 |
| `environment` | 글로벌 환경변수 |
| `env_file` | .env 파일 로딩 |
| `stack` | 인프라 오케스트레이션 파이프라인 (플러그인 기반) |
| `checks` | `dva doctor` 환경 사전조건 체크 |
| `applications` | 앱 프로세스 정의 (native/docker 전략) |
| `default_mode` | 기본 모드명 |
| `modes` | 운영 모드 (`--mode` 플래그용) |
| `environments` | 환경 프리셋 (`--env` 플래그용) |
| `health_checks` | 비-compose 서비스 헬스체크 |
| `interaction` | 커맨드 정의 (command, command list, script, script_file, steps, subcommands 등) |
| `provision` | 프로비저닝 프로필 및 스텝 정의 |
| `modules` | `.sb/dva/*.yml` 모듈 분리 |
| `subprojects` | 서브프로젝트 참조 (모노레포) |
| `endpoints` | 사용자 노출 URL 정의 |
| `infra` | 공유 인프라 서비스 (git 기반) |
| `ssh` | SSH agent 설정 |
| `devcontainer` | devcontainer 통합 (실험적) |

### stack (인프라 파이프라인)

`stack:` 섹션에서 여러 플러그인 엔트리를 `order` 순서대로 실행합니다.

```yaml
stack:
  compose:                      # 이름 = 플러그인 자동추론
    order: 10
    files: [docker-compose.yml]
    project_name: myapp
    services:
      api:
        tags: [app]
        related: [worker]
  kubectl:                      # kubectl 플러그인 자동추론
    order: 20
    namespace: myapp-dev
    context: my-cluster
    kubeconfig: ~/.kube/config
  staging-compose:              # 이름 ≠ 플러그인명이면 plugin: 명시
    plugin: compose
    order: 30
    files: [docker-compose.staging.yml]
```

지원 플러그인:

| Tier | Plugins |
|------|---------|
| Core | `compose`, `kubectl`, `helm`, `process`, `script`, `docker` |
| Extended | `kustomize`, `tilt`, `skaffold`, `podman-compose`, `vagrant` |
| Niche | `sam`, `serverless`, `multipass` |

플러그인 타입 결정 우선순위:
1. 중첩 포맷 (legacy): `compose:` 서브키가 있으면 해당 플러그인
2. 플랫 포맷 + `plugin:` 명시
3. 엔트리 이름이 알려진 플러그인명과 일치하면 자동추론

### modes (--mode)

`dva up -M <name>`으로 활성화. Compose profiles, 서비스 필터, 환경변수, stack 엔트리 필터를 묶어서 관리합니다.

```yaml
modes:
  backend:
    description: "Backend services only"
    compose_profiles: [backend]
    compose_services: [api, postgres, redis]
    health_checks: [api-server]
    stack: [compose]            # 특정 stack 엔트리만 실행
    applications: native        # 앱 실행 전략 (문자열: 전체 적용)
    environment:
      LOG_LEVEL: debug
  native:
    description: "No compose, local services only"
    compose_services: []  # 빈 배열 = compose 스킵
    health_checks: [local-api]
  hybrid:
    description: "Mixed native/docker"
    applications:              # 앱별 전략 맵
      api: native
      worker: docker
      web: native
```

### applications

`dva app` 명령으로 관리하는 앱 프로세스를 정의합니다.

```yaml
applications:
  api:
    description: "REST API server"
    tags: [app, api]
    port: 11200
    dir: "rust-workspace"          # 작업 디렉토리
    depends_on: []                 # 의존 앱/서비스 (토폴로지 정렬)
    run:
      native: "cargo run --release -p api-server"
      docker:
        service: api-rs
        profile: rust
    dev:                           # --dev 모드용 (hot-reload)
      native: "cargo watch -x 'run -p api-server'"
    build:
      native: "cargo build -p api-server"
      docker:
        service: api-rs
        command: "cargo build"
    environment:
      PORT: "11200"
    health:
      type: http                   # http, tcp, command
      url: "http://localhost:11200/health"
      timeout: 5
      ready_timeout: 120
```

`run`, `dev`, `build`는 문자열 축약형도 지원합니다:

```yaml
  web:
    run: "pnpm build && pnpm preview"   # = run.native
    dev: "pnpm dev --port 11300"        # = dev.native
```

### environments (--env)

`dva up -E <name>`으로 활성화. 환경변수 프리셋입니다.

```yaml
environments:
  ci:
    description: "CI environment"
    environment:
      CI: "true"
      LOG_LEVEL: warn
    stack: [compose]               # 특정 stack 엔트리만 실행 (선택)
```

### health_checks

비-compose 서비스(로컬 프로세스 등)의 상태를 확인합니다. `start` 필드가 있으면 자동 시작도 합니다.

```yaml
health_checks:
  local-api:
    type: http           # http, tcp, command
    url: http://localhost:3000/health
    start: "npm run dev"
    start_hint: "Run 'npm run dev' in another terminal"
    timeout: 2           # 헬스체크 타임아웃 (초)
    ready_timeout: 30    # 시작 후 대기 (초)
```

### subprojects

모노레포에서 서브프로젝트별 dva.yml을 참조합니다.

```yaml
subprojects:
  api:
    path: ./services/api
    exclude_tags: [heavy]
```

실행: `dva api:test` 또는 `dva run --project api test`

### 특수 변수

| Variable | Description |
|----------|-------------|
| `DVA_OS` | 현재 OS (`linux`, `darwin`, `windows`) |
| `DVA_WORK_DIR_REL_PATH` | 작업 디렉토리 상대 경로 |
| `DVA_CURRENT_USER` | 현재 사용자명 (`username`) |
| `DVA_CURRENT_UID` | 현재 사용자 UID (숫자) |

### 설정 파일 로딩 순서

1. `DVA_FILE` 환경변수 (설정 시)
2. 현재 디렉토리에서 루트까지 `dva.yml` 탐색
3. `.sb/dva/*.yml` 모듈 병합
4. `dva.override.yml` 오버라이드 적용

## LLM Integration

DVA는 LLM 에이전트(Claude, Cursor 등)와의 통합을 위한 기능을 제공합니다.

- `dva config improve` — Claude Code CLI로 dva.yml AI 개선
- `dva config improve --print` — 기존 dva.yml 개선용 프롬프트 출력 (수동 붙여넣기용)
- `dva config improve --docs-only` — CLAUDE.md/AGENTS.md만 갱신
- `dva config improve --interactive` — Claude Code 인터랙티브 모드 (세션 유지)
- `dva config improve --rewrite` — dva.yml을 처음부터 재작성
- `dva manifest` — 구조화된 커맨드 매니페스트 (JSON/YAML)
- `dva config show` — 병합된 최종 설정 출력
- `--json` 글로벌 플래그 — 모든 출력을 JSON으로
- `claude-plugin/` — Claude Code 플러그인
