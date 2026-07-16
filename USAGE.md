# DVA 사용 가이드

> DVA CLI 전체 커맨드 레퍼런스 및 설정 가이드.
> 현재 권장 모델은 `stack`을 선언 저장소로 두고, 실제 실행은 `plans`의 이름을 대상으로 수행하는 구조입니다.
> 빠른 시작은 [README.md](README.md), 설계 배경은 [docs/40-declarative-stack-and-plans.md](docs/40-declarative-stack-and-plans.md) 참조.

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
| `dva config docs` | 프로젝트 AI 파트너용 CLAUDE.md/AGENTS.md 생성/갱신 |
| `dva run CMD [ARGS]` | `dva.yml`에 정의된 interaction 커맨드 실행 |
| `dva ls` | 실행 가능한 이름과 interaction 목록 표시 |
| `dva show <NAME>` | 특정 실행 이름 또는 설정 개요 표시 |
| `dva up <NAME>` | named execution entry 실행 |
| `dva down <NAME>` | named execution entry teardown |
| `dva stop <NAME>` | named execution entry 중지 |
| `dva status [NAME]` | 실행 상태 표시 |
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

생성 후 `am run dva-improve`로 AI 기반 최적화를 실행할 수 있습니다.

#### docs (config docs)

```bash
dva config docs                  # CLAUDE.md/AGENTS.md 가이드 생성/갱신
```

`docs`는 AI 에이전트가 DVA 환경을 인식하게 만드는 기본 문서를 생성합니다.
(과거 `dva config improve --docs-only`와 동일)

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
| `dva show` | 설정 요약 또는 특정 실행 이름 상세 표시 |
| `dva status` | 워크스페이스 상태 (컨테이너, 서비스 상태) |
| `dva config show` | 최종 병합된 설정 출력 (modules + override 적용 후) |

```bash
dva show                  # 등록된 설정 전체 요약
dva show local-dev        # 특정 named execution entry 상세
dva show --json           # JSON 출력
dva status                # 전체 상태 요약
dva status local-dev      # 특정 named execution entry 상태
dva status --json         # JSON 출력
dva config show           # JSON 형식 (기본)
dva config show -f yaml   # YAML 형식
```

### Lifecycle

#### 권장 실행 모델

권장 구조:

- `stack` = 재사용 가능한 실행 대상 선언
- `plans` = 실제 실행 가능한 이름
- `environments` = dev/stg/prd 같은 환경 차이
- `sites` = local/office/remote/cloud 같은 실행 host 차이
- `interactions` = 단발성 편의 명령
- `provision` = 준비/초기화 절차

```bash
dva ls
dva show local-dev
dva up local-dev
dva status local-dev
dva stop local-dev
dva down local-dev
```

#### named execution entry

실행 명령의 직접 대상은 `stack`이 아니라 `plans.<name>` 입니다.

예:

```bash
dva up local-dev
dva up backend/local-dev
```

`stack`은 선언 저장소이므로 `dva stack up`은 더 이상 권장 모델이 아닙니다.

| Command | Description |
|---------|-------------|
| `dva up <NAME>` | named execution entry 실행 |
| `dva down <NAME>` | named execution entry teardown |
| `dva stop <NAME>` | 중지 (제거하지 않음) |
| `dva restart <NAME>` | 재시작 |
| `dva logs [NAME]` | 로그 보기 |
| `dva build [NAME]` | 빌드 수행 |
| `dva clean` | 전체 정리 |

```bash
dva up local-dev
dva down local-dev
dva stop local-dev
```

#### stack 서브커맨드

`stack` 엔트리를 개별적으로 제어해야 할 때 사용합니다.

| Command | Description |
|---------|-------------|
| `dva stack up [NAME...]` | stack 엔트리 시작 (이름 생략 시 전체) |
| `dva stack stop [NAME...] [OPTIONS]` | 리소스를 제거하지 않고 stack 엔트리 중지 |
| `dva stack down [NAME...]` | stack 리소스 중지 및 제거 |
| `dva stack status` | stack 엔트리 상태 표시 |
| `dva stack log [NAME] [OPTIONS]` | stack 엔트리 로그 보기 |

```bash
dva stack stop                  # 전체 중지 (상태 보존)
dva stack log compose           # 특정 stack 엔트리 로그 보기
```

#### app 서브커맨드

`applications` 섹션에 정의된 앱 프로세스를 제어합니다.

| Command | Description |
|---------|-------------|
| `dva app ls` | 앱 목록을 status, health, PID와 함께 표시 |
| `dva app up [APP...] [--dev]` | 앱 시작 (이름 생략 시 전체), `--dev`는 hot-reload 모드 |
| `dva app build [APP...]` | 앱 빌드 (`--docker` 지정 시 컨테이너 빌드) |
| `dva app restart [APP...] [--dev]` | 앱 재시작 (중지 후 시작) |
| `dva app stop [APP...]` | 상태를 제거하지 않고 앱 중지 (PID 보존, 빠른 재시작용) |
| `dva app down [APP...]` | 앱 중지 및 리소스(PID 파일, 로그) 제거 |
| `dva app log <APP>` | 앱의 최근 로그 표시 (마지막 100줄) |

```bash
dva app build myapp       # 특정 앱 빌드
dva app restart myapp     # 특정 앱 재시작
dva app stop myapp        # 중지 (빠른 재시작을 위해 상태 보존)
dva app log myapp         # 최근 로그 확인
```

#### clean

```bash
dva clean                 # containers + networks 제거
dva clean -v              # + 볼륨 제거 (데이터 손실 주의)
dva clean -i              # + 로컬 빌드 이미지 제거
dva clean -f              # 확인 프롬프트 스킵
```

#### `--env` / `--site` / `--var`

새 구조에서는 실행 이름이 기본 컨텍스트를 담고 있으므로, 환경 분기는 주로 설정의 `plans`에서 결정합니다.

- `environment`: `environments.<name>` 선택
- `site`: `sites.<name>` 선택
- `vars`: 실행 시점 변수 override

권장 방식:

- 기본은 `plans.<name>` 안에 `environment`, `site`를 정의
- 추가 일회성 조정이 필요하면 `--var KEY=VALUE` 같은 명시적 override 사용

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
| `dva validate` | dva.yml 스키마 + 시맨틱 검증 (`dva config validate`도 지원) |
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
- 실행 계획 누락 또는 과도하게 무거운 기본 실행 구성 경고
- 미해결 환경변수 (`${MISSING_VAR}`), 비지원 셸 문법 감지
- 깊은 서브커맨드 중첩 (5단계 초과), 도달 불가능 커맨드
- 정규 섹션 순서 검증

## Configuration (`dva.yml`)

### 기본 구조

```yaml
version: "0.1.44"         # 최소 DVA 버전

env_file:
  - .env

stack:
  core-compose:
    default_runner: compose
    runners:
      compose:
        files: [docker-compose.yml]
        project_name: myproject

plans:
  local-dev:
    environment: dev
    site: local
    entries:
      - name: core-compose
        runner: compose
        services: [postgres, redis]

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
| `vars` | 글로벌 환경변수 |
| `env_file` | .env 파일 로딩 |
| `stack` | 재사용 가능한 실행 대상 선언 |
| `plans` | 실제 실행 가능한 이름 |
| `checks` | `dva doctor` 환경 사전조건 체크 |
| `default_mode` | `--mode` 미지정 시 적용할 기본 `modes` 엔트리 |
| `modes` | 런타임 전략 프리셋 (`--mode`로 선택) |
| `environments` | 환경 프리셋 (`dev/stg/prd`) |
| `sites` | 실행 host 프리셋 (`local/remote/cloud`) |
| `health_checks` | 비-compose 서비스 헬스체크 |
| `interaction` | 커맨드 정의 (command, command list, script, script_file, steps, subcommands 등) |
| `provision` | 프로비저닝 프로필 및 스텝 정의 |
| `modules` | `.sb/dva/*.yml` 모듈 분리 |
| `subprojects` | 서브프로젝트 참조 (모노레포) |
| `endpoints` | 사용자 노출 URL 정의 |
| `infra` | 공유 인프라 서비스 (git 기반) |
| `ssh` | SSH agent 설정 |
| `devcontainer` | devcontainer 통합 (실험적) |

### stack (선언 저장소)

`stack:`은 logical unit 선언 모음입니다.
직접 실행 대상이 아니라 `plans.entries[].name`에서 참조됩니다.

```yaml
stack:
  core-compose:
    description: infra bundle
    default_runner: compose
    runners:
      compose:
        files: [docker-compose.yml, docker-compose.dev.yml]

  api:
    description: backend api
    default_runner: native
    runners:
      native:
        dir: apps/api
        run: go run ./cmd/api
      docker:
        image: myorg/api:dev
        run: docker run --rm myorg/api:dev
      helm:
        chart: ./charts/api
        release: api
        namespace: default
```

핵심 규칙:

- 하나의 `stack` 엔트리는 multi-runner logical unit이 될 수 있음
- `default_runner`는 기본 실행 백엔드
- 실제 실행 runner는 plan/site에서 override 가능
- 정의되지 않은 runner 선택은 validation error

지원 가능한 runner 예:

| Tier | Plugins |
|------|---------|
| Core | `compose`, `kubectl`, `helm`, `process`, `script`, `docker` |
| Extended | `kustomize`, `tilt`, `skaffold`, `podman-compose`, `vagrant` |
| Niche | `sam`, `serverless`, `multipass` |

### plans

`plans`는 실제 실행 가능한 이름입니다.

```yaml
plans:
  local-dev:
    environment: dev
    site: local
    vars:
      LOG_LEVEL: debug
    entries:
      - name: core-compose
        runner: compose
        order: 10
        services: [postgres, redis]
      - name: api
        runner: native
        order: 20
        depends_on: [core-compose]
```

`dva up local-dev`처럼 직접 실행합니다.

### default_mode

`default_mode`는 `--mode`(`-M`)를 지정하지 않았을 때 적용할 `modes` 엔트리를 선택합니다.

```yaml
default_mode: infra

modes:
  infra:
    ...
  full:
    ...
```

- 기본값이 없습니다. 설정하지 않으면 어떤 mode도 적용되지 않으며, `dva up`은 모든 compose 파일의 모든 서비스를 시작합니다.
- `modes`가 정의되어 있는데 `default_mode`가 비어 있으면 `dva validate`가 경고합니다. 최소 인프라 mode(예: `infra`)를 지정하는 것을 권장합니다.
- `modes`에 없는 이름을 지정하면 경고가 아니라 검증 에러입니다.

### environments / sites

```yaml
environments:
  dev:
    environment:
      APP_ENV: dev
      LOG_LEVEL: debug

sites:
  local:
    vars:
      DVA_SITE: local
    entry_overrides:
      api:
        runner: native
```

`vars` 우선순위 (낮음 → 높음):

```text
env_file < global vars < environment vars < site vars < plan vars < CLI vars < OS 환경 변수
```

OS 환경 변수가 가장 높은 우선순위입니다. 같은 키가 OS에 설정되어 있으면
`dva.yml`의 어떤 레이어(`--var` 포함)도 그 값을 덮어쓰지 못합니다.

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
기본 연결 대상은 `plans`, `interactions`, 필요시 `provision`입니다.

```yaml
subprojects:
  backend:
    path: ./services/backend
    import:
      plans: [local-dev]
      interactions: [shell, logs]
      provision: [setup]
```

실행 이름은 canonical namespace를 사용합니다.

```bash
dva up backend/local-dev
dva run backend/shell
dva provision backend/setup
```

subproject의 `interaction`과 `provision`은 해당 subproject root 기준으로 실행됩니다.

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

- `am run dva-improve` — dva.yml AI 개선 (기존 파일 수정)
- `am run dva-improve param.mode=rewrite` — dva.yml AI 개선 (처음부터 재작성)
- `dva config docs` — CLAUDE.md/AGENTS.md 가이드 생성/갱신
- `am run dva-improve-guided` — Claude Code 대화형 가이드 모드
- `dva manifest` — 구조화된 커맨드 매니페스트 (JSON/YAML)
- `dva config show` — 병합된 최종 설정 출력
- `--json` 글로벌 플래그 — 모든 출력을 JSON으로
- `claude-plugin/` — Claude Code 플러그인
