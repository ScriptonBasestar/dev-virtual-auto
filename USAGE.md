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
| `dva run CMD [ARGS]` | `dva.yml`에 정의된 interaction 커맨드 실행 |
| `dva ls` | 사용 가능한 interaction 커맨드 목록 |
| `dva version` | 버전 표시 |

`dva run`은 생략 가능합니다. `dva shell`은 `dva run shell`과 동일합니다.
`namespace:command` 문법도 지원합니다 (예: `dva engine:test`).

#### init (config init)

```bash
dva config init              # 자동 감지 기반 dva.yml 생성
dva init                     # 위와 동일 (backward compat alias)
dva config init -t node      # 템플릿 지정 (minimal, rails, node, python, go)
dva config init -p           # LLM용 프롬프트 출력
dva config init --ai         # Claude Code CLI로 dva.yml 자동 생성
dva config init --ai --no-ai-docs  # AI 생성 시 agent 문서 스킵
dva config init -v           # AI 생성 시 상세 진행 표시
dva config init --devcontainer   # .devcontainer/devcontainer.json 포함 생성
dva config init --all        # 자동 감지 시 가능한 모든 기능 통합 활성화

#### config improve

```bash
dva config improve           # Claude Code CLI로 개선 실행 (기본)
dva config improve --print   # 프롬프트만 stdout 출력 (수동 빧여넣기용)
dva config improve --docs-only  # CLAUDE.md/AGENTS.md만 갱신 (dva.yml 미변경)
```

#### run

```bash
dva run shell             # interaction 커맨드 실행
dva shell                 # 위와 동일 (run 생략)
dva run -e test           # --dry-run 별칭으로 실행 계획 표시
dva run -p 8080:80 web    # 포트 퍼블리시
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
| `dva add [FEATURE]` | 특정 통합 기능 추가 연동 (e.g., `devcontainer`) |
| `dva show` | 설정 요약 (modes, environments, commands 등) |
| `dva status` | 워크스페이스 상태 (컨테이너, 서비스 상태) |
| `dva config show` | 최종 병합된 설정 출력 (modules + override 적용 후) |
| `dva config dump` | (deprecated) `dva config show` 사용 |

```bash
dva show                  # 등록된 설정 전체 요약
dva show --json           # JSON 출력
dva status                # docker compose ps + 헬스체크 상태
dva status --json         # JSON 출력
dva config show           # JSON 형식 (기본)
dva config show -f yaml   # YAML 형식
```

### Lifecycle (Stack Orchestration)

`stack:` 섹션에 정의된 플러그인들을 `order` 순서대로 실행합니다.

| Command | Description |
|---------|-------------|
| `dva up [SERVICE]` | stack 시작 (compose up, kubectl apply 등) |
| `dva down` | stack 중지 및 제거 |
| `dva stop [SERVICE]` | 서비스 중지 (제거하지 않음) |
| `dva restart [SERVICE]` | 서비스 재시작 (stop + start) |
| `dva logs [SERVICE]` | 컨테이너 로그 보기 |
| `dva build [SERVICE]` | 이미지 빌드 |
| `dva clean` | 전체 정리 (containers, networks) |

#### up

```bash
dva up                    # stack 전체 시작 (order 순서)
dva up postgres redis     # 특정 서비스만
dva up -f                 # foreground 모드 (attached)
dva up --force            # 헬스체크 무시, 강제 재시작
dva up --no-wait          # 즉시 리턴 (wait 하지 않음)
dva up -M backend         # 모드 적용 (mode.stack 엔트리만 실행)
dva up -E staging         # 환경 프리셋 적용
dva up -T backend,ui      # 특정 태그 그룹만 실행 (--tag 별칭 허용)
dva up --exclude-tags db  # 특정 태그 그룹 제외 (--exclude-tag 별칭 허용)
dva up -M backend -E staging  # 모드 + 환경 조합
```

이미 모든 서비스가 running + healthy이면 자동으로 스킵하고 상태만 표시합니다.
`--force`로 강제 재시작할 수 있습니다.

#### clean

```bash
dva clean                 # containers + networks 제거
dva clean -v              # + 볼륨 제거 (데이터 손실 주의)
dva clean -i              # + 로컬 빌드 이미지 제거
dva clean -f              # 확인 프롬프트 스킵
```

#### --mode / --env / --tags 플래그

`--mode`(`-M`), `--env`(`-E`), `--tags`(`-T`), `--exclude-tags`는 `up`, `down`, `stop`, `restart`에서 사용 가능합니다.

- `--mode`: `modes` 섹션에 정의된 운영 모드 적용 (compose profiles, 서비스 필터, 환경변수)
- `--env`: `environments` 섹션에 정의된 환경변수 프리셋 적용
- `--tags`: `tags` 속성 기반으로 특정 컨테이너 그룹만 선택 실행 (별칭: `--tag`)
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
| `dva config validate` | dva.yml 스키마 검증 |
| `dva migrate` | 레거시 설정 포맷 감지 및 마이그레이션 가이드 출력 |
| `dva doctor` | 환경 사전조건 및 설정 문제 진단 |
| `dva completion bash\|zsh\|fish` | 셸 자동완성 스크립트 생성 |

#### provision

```bash
dva provision             # default 프로필 실행
dva provision setup       # 특정 프로필 실행
dva provision --list      # 사용 가능한 프로필 목록
```

#### config validate

```bash
dva config validate       # 스키마 + compose project name 검증
dva config validate --fix # compose 파일 project name 불일치 자동 수정
```

## Configuration (`dva.yml`)

### 기본 구조

```yaml
version: "0.1.0"          # 최소 DVA 버전

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

| Section | Description |
|---------|-------------|
| `stack` | 인프라 오케스트레이션 파이프라인 (플러그인 기반) |
| `environment` | 글로벌 환경변수 |
| `env_file` | .env 파일 로딩 |
| `interaction` | 커맨드 정의 (service, command, subcommands 등) |
| `provision` | 프로비저닝 프로필 및 스텝 정의 |
| `modes` | 운영 모드 (`--mode` 플래그용) |
| `environments` | 환경 프리셋 (`--env` 플래그용) |
| `health_checks` | 비-compose 서비스 헬스체크 |
| `endpoints` | 사용자 노출 URL 정의 |
| `checks` | `dva doctor` 환경 사전조건 체크 |
| `subprojects` | 서브프로젝트 참조 (모노레포) |
| `infra` | 공유 인프라 서비스 (git 기반) |
| `modules` | `.sb/dva/*.yml` 모듈 분리 |
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
    environment:
      LOG_LEVEL: debug
  native:
    description: "No compose, local services only"
    compose_services: []  # 빈 배열 = compose 스킵
    health_checks: [local-api]
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
5. deprecated `lifecycle` → `stack` 자동 변환

## LLM Integration

DVA는 LLM 에이전트(Claude, Cursor 등)와의 통합을 위한 기능을 제공합니다.

- `dva config init --ai` — Claude Code CLI로 dva.yml 자동 생성
- `dva config init -p` — LLM에게 전달할 프롬프트 출력
- `dva config improve --print` — 기존 dva.yml 개선용 프롬프트 출력
- `dva config improve --docs-only` — CLAUDE.md/AGENTS.md만 갱신
- `dva manifest` — 구조화된 커맨드 매니페스트 (JSON/YAML)
- `dva config show` — 병합된 최종 설정 출력
- `--json` 글로벌 플래그 — 모든 출력을 JSON으로
- `claude-plugin/` — Claude Code 플러그인
