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
| `dva config migrate` | legacy compose 선언을 `runners` 형태로 재작성 |
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
dva config init -t node          # --template: 템플릿 지정 (minimal, rails, node, python, go)
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

#### migrate (config migrate)

```bash
dva config migrate               # 변경 결과만 출력 (파일은 그대로)
dva config migrate --write       # 실제 적용
dva config migrate ../other-repo # 다른 프로젝트 미리보기
```

compose를 stack 항목에 직접 선언하던 세 가지 legacy 형태 — 이름이 `compose`인
항목에 compose 키를 그대로 둔 형태, `plugin: compose`, 중첩 `compose:` 하위 키 —
를 현재 스키마가 요구하는 `default_runner` + `runners.compose` 형태로 옮깁니다.

```yaml
stack:                          stack:
  compose:                        compose:
    files: [compose.yml]   ->       default_runner: compose
                                    runners:
                                      compose:
                                        files: [compose.yml]
```

바뀌는 항목만 재작성하므로 나머지 줄은 주석·빈 줄까지 원본 바이트 그대로
유지됩니다. `--write` 전에 결과를 메모리에서 먼저 로드해 검증하므로 DVA가 읽을 수
없는 상태로 파일이 남지 않습니다.

`tags`는 옮기지 않고 **양쪽에 복사**합니다. `LifecycleEntry.Tags`는 stack 항목
필터링에, `ComposePluginConfig.Tags`는 compose 서비스 필터 기본값에 쓰이는데
legacy 형태에서는 한 키가 두 역할을 겸했기 때문입니다.

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

#### 라이프사이클 플래그

플래그 집합은 **이름 없이 실행할 때**와 **named plan을 지정해 실행할 때**가 서로 다릅니다.

**이름 없이 실행 시** (`dva up`, `dva down`, `dva stop`, `dva restart`, `dva stack up/down/stop`)

`plans`가 정확히 하나이면 이름 없는 `dva up`/`down`/`stop`/`restart`/`status`는 그 plan을 기본 실행한다. 앞에 플래그만 두면 기본 plan 경로가 막히므로, `dva up <plan> --dev`처럼 plan 이름을 명시해야 한다.

| Flag | Description |
|---|---|
| `--mode`, `-M MODE` | `modes` 섹션의 named mode 적용 |
| `--env`, `-E ENV` | `environments` 섹션의 named environment 적용 |
| `--tag`, `--tags`, `-T TAG[,TAG]` | 해당 태그를 가진 lifecycle 엔트리만 포함 |
| `--exclude-tag`, `--exclude-tags TAG[,TAG]` | 해당 태그를 가진 lifecycle 엔트리 제외 |

`dva up`과 `dva stack up`은 위에 더해 다음을 인식합니다.

| Flag | Description |
|---|---|
| `--force` | 이미 실행 중이어도 강제로 재시작 |
| `--no-wait` | 서비스 시작 후 준비 상태를 기다리지 않고 즉시 반환 |
| `--dev` | 앱을 dev 모드(hot-reload)로 시작 |
| `--docker` | 앱을 docker 전략으로 강제 실행 |

**named plan 지정 시** (`dva up <NAME>`, `dva down <NAME>`, `dva stop <NAME>`)

| Flag | Description |
|---|---|
| `--force` | 이미 실행 중이어도 강제로 재시작 |
| `--no-wait` | 준비 상태를 기다리지 않고 즉시 반환 |
| `--var KEY=VAL` | 실행 시점 변수 override |
| `-v`, `--volumes` | teardown 시 볼륨까지 제거 |
| `--dry-run` | 실행 계획만 표시 |

환경/모드/태그는 plan 정의(`plans.<name>`)가 결정하므로, named plan 실행에는 `--mode`/`--env`/`--tag`를 쓸 수 없습니다.

```bash
dva up --tag db,cache          # db/cache 태그 엔트리만 시작
dva up --exclude-tag heavy     # heavy 태그 엔트리 제외하고 시작
dva up --force --no-wait       # 강제 재시작 후 대기 없이 반환
dva down -E staging            # staging environment 설정으로 teardown
dva up local-dev --force       # named plan을 강제 재시작
```

`--tag`/`--exclude-tag`은 `--exclude-tag=heavy,slow` 형태의 `=` 문법도 지원합니다.

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

> **`dva stack up`은 plans/`default_plan`을 참조하지 않습니다.** Compose 러너에서는
> `--profile` 없는 `docker compose up`이므로 profile 없는 서비스만 뜹니다. 기본을 최소로
> 유지하려면 **Docker Compose 네이티브 `profiles:`**로 계층을 나누세요 — 코어 데이터
> (postgres/redis)는 profile 없이 항상 시작하고, 무거운 계층은
> `profiles: [workflow|monitoring|dev-tools|apps]`로 opt-in 합니다. 명시적 서비스 서브셋
> 실행은 `dva up <plan>`(`plans.entries[].services`)을 쓰며, plan이 profile 걸린 서비스를
> 이름으로 지정하면 profile과 무관하게 시작됩니다.

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

`dva app up` waits for application health by default. An alive process that misses `ready_timeout` remains advisory (exit 0) unless `applications.<name>.health.required: true`, which makes the command fail with non-zero exit and `[FAIL]` output. `dva app up --no-wait` skips readiness. This contract is application-only; top-level `health_checks` does not support `required` (omitted/false = advisory/zero, required:true = strict failure/non-zero).

#### clean

```bash
dva clean                 # containers + networks 제거
dva clean -v              # + 볼륨 제거 (데이터 손실 주의)
dva clean -i              # + 로컬 빌드 이미지 제거
dva clean -f              # 확인 프롬프트 스킵
```

#### 환경 분기 (`environment` / `site` / `vars`)

새 구조에서는 실행 이름이 기본 컨텍스트를 담고 있으므로, 환경 분기는 주로 설정의 `plans`에서 결정합니다.
아래 세 항목은 CLI 플래그가 아니라 `plans.<name>` 안의 YAML 필드입니다.

- `environment`: `environments.<name>` 선택
- `site`: `sites.<name>` 선택
- `vars`: 실행 시점 변수 override

권장 방식:

- 기본은 `plans.<name>` 안에 `environment`, `site`를 정의
- 추가 일회성 조정이 필요하면 `--var KEY=VALUE` 같은 명시적 override 사용

### Integration Tools

| Command | Description |
|---------|-------------|
| `dva compose ARGS` | raw Docker Compose 패스스루 (escape hatch — 내가 소유한 compose를 직접 실행) |
| `dva ktl ARGS` | kubectl 패스스루 |
| `dva infra up/down [SVC]` | ⚠️ deprecated → stack `source:`로 흡수 ([stack.source](#stacksource-외부-스택-소싱) 참조). SVC 생략 시 `infra` 태그 전체 |
| `dva ssh up/down/status` | SSH agent 컨테이너 관리 |

#### ssh up

```bash
dva ssh up                        # 기본값으로 SSH agent 컨테이너 시작
dva ssh up -k ~/.ssh/id_ed25519   # --key: SSH 키 경로 (기본값 $HOME/.ssh/id_rsa)
dva ssh up -u devuser             # --user: ssh-agent 컨테이너에서 사용할 사용자
dva ssh up -v /workspace          # --volume: 마운트할 볼륨 (기본값 $HOME)
```

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
| `default_plan` | 플랜 이름 미지정 시 적용할 기본 `plans` 엔트리 (여러 plan 중 기본 선택) |
| `checks` | `dva doctor` 환경 사전조건 체크 |
| `default_mode` | `--mode` 미지정 시 적용할 기본 `modes` 엔트리 |
| `modes` | 런타임 전략 프리셋 (`--mode`로 선택) |
| `environments` | 환경 프리셋 (`dev/stg/prd`) |
| `sites` | 실행 host 프리셋 (`local/remote/cloud`) |
| `health_checks` | 비-compose 서비스 헬스체크 |
| `interaction` | 커맨드 정의 (command, command list, script, script_file, steps, subcommands 등) — 예약어/훅 규칙은 아래 [interaction](#interaction-예약어와-훅) 참조 |
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

### stack.source (외부 스택 소싱)

stack 엔트리는 `source:`로 **외부 소유 스택**(다른 repo나 로컬 디렉토리에 정의된
compose 스택)을 가져와 실행합니다. 소싱과 실행을 분리해, 정의는 외부 도구가
소유하고 DVA는 fetch와 수명 주기만 조정합니다.

```yaml
stack:
  postgres:
    default_runner: compose
    source:
      git: https://example.com/shared-infra.git
      ref: v1.2.0                    # 재현성 위해 SHA/tag 권장
    runners:
      compose:
        files: [docker-compose.yml]  # source 디렉토리 기준 (생략 시 자동 탐색)

  local-infra:
    default_runner: compose
    source:
      path: ../shared-infra          # 로컬 디렉토리 참조 (fetch 없음)
```

핵심 규칙:

- `git`과 `path`는 상호 배타 — 정확히 하나만 지정.
- git 소스는 `dva up` 시 **없을 때만 clone**하며 자동 pull하지 않습니다(재현성).
  명시적 갱신: `dva infra update <name>`.
- 소싱된 엔트리의 `runners.compose.files`와 실행 작업 디렉토리(`.env`,
  build context, 볼륨)는 **source 디렉토리 기준**으로 해석됩니다.
- git 캐시 위치: `.sb/dva/sources/<name>/`.

**`infra:` 마이그레이션** — 구 top-level `infra:` 맵은 deprecated입니다. 로드 시
`source:` 기반 stack 엔트리(태그 `infra`)로 자동 변환되며 경고를 출력하고,
`dva infra up/down`은 이 stack 엔트리로 위임됩니다. 새 설정은
`stack.<name>.source`를 직접 사용하세요.

### plans

`plans`는 실제 실행 가능한 이름입니다.

```yaml
plans:
  local-dev:
    environment: dev
    site: local
    endpoint_tags: [app]
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

성공한 `up`은 설정된 endpoint를 출력합니다. `endpoint_tags`를 생략하거나 빈 배열로
두면 모든 endpoint를 표시하고, 값을 지정하면 tag가 하나라도 일치하는 endpoint만
표시합니다. `--dry-run`과 실패한 startup은 endpoint 연결 정보를 출력하지 않습니다.

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

### default_plan

`default_plan`은 플랜 이름 없이 `dva up/down/stop/restart`를 실행할 때 적용할 `plans` 엔트리를 선택합니다.

```yaml
default_plan: dev

plans:
  dev:     { entries: [ { name: frontend-dev,     runner: process } ] }
  preview: { entries: [ { name: frontend-preview, runner: process } ] }
```

- `plans`가 정확히 1개면 그 플랜이 자동으로 기본값입니다. `default_plan`은 **여러 plan 중** 기본을 고를 때 씁니다.
- `plans`에 없는 이름을 지정하면 검증 에러입니다 (`dva config validate`).
- 무엇을 기본으로 둘지는 프로젝트 정책입니다 (예: devbox 로컬은 `dev`). DVA는 선택지를 표현할 뿐 기본을 강제하지 않습니다.

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

`vars` 우선순위 (낮음 → 높음) — plan 실행 경로(`dva up <plan>`) 기준:

```text
env_file < global vars < environment vars < site vars < plan vars < CLI vars < OS 환경 변수
```

여기서 `environment vars`는 `environments.<name>.environment`를 뜻하며, 최상위
`environment:` 블록과는 다릅니다. 최상위 `environment:`는 `dva run` 경로에서
`env_file`보다 **먼저** 적용되어 덮어써집니다 (`environment:` < `env_file` < OS).

OS 환경 변수가 가장 높은 우선순위입니다. 같은 키가 OS에 설정되어 있으면
`dva.yml`의 어떤 레이어(`--var` 포함)도 그 값을 덮어쓰지 못합니다.

#### 실제 적용 결과 확인

`--dry-run`은 실행 대신 **해석 결과**를 출력합니다. 위 순서의 각 레이어가 실제로 몇 개
키를 얹었는지, 어떤 레이어가 비어 있는지를 그대로 보여주므로 "이 변수가 왜 이 값인가"를
추측 없이 확인할 수 있습니다.

```bash
dva up <plan> --dry-run
```

```text
Resolution:
  plan: resolved "local-dev"
  vars: env_file — declared [.env], applied at config load below every layer here
  vars: environment: — not declared
  vars: global vars — merged (2 keys)
  vars: environments."dev" — merged (1 key)
  vars: sites."local".vars — merged (1 key)
  vars: plans."local-dev".vars — merged (1 key)
  vars: cli --var — none passed
  vars: OS environment overrides every layer above
```

`down`, `stop`, `restart`도 동일합니다. 출력은 stderr로 나가므로 `--json`을 함께 써도
stdout의 JSON은 그대로 파싱됩니다. 각 레이어의 의미는
[docs/31-execution-plan-resolution.md](docs/31-execution-plan-resolution.md#4-3-vars-병합)을
참조하세요.

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

### interaction (예약어와 훅)

`interaction:` 키는 `dva run <name>`으로 실행할 커맨드를 정의합니다. 이름이 내장
커맨드와 겹치면 `dva validate`가 exit 1로 실패하고, 설정을 읽을 때마다 경고가 출력됩니다.
선언이 버려지는 것은 아니며 짧은 형식만 내장 커맨드에게 넘어갑니다 — 아래 규칙을 따릅니다.

**예약어 27개** — 내장 커맨드 이름입니다:

```text
help  version  ls       compose  up      stop   down     build  clean
run   provision validate manifest ktl     ssh    infra    console
completion init  status  config   logs    restart show    doctor
app   stack
```

**훅 가능 7개** — 예약어 중 `before`/`replace`/`after` 훅을 받는 것:

```text
up  down  stop  restart  build  clean  logs
```

판정 규칙:

| `interaction:` 키 | 훅 필드 | 결과 | 도달하는 호출 |
| --- | --- | --- | --- |
| 예약어 아님 | — | 정상 등록 | `dva <name>` |
| 훅 가능 예약어 | `before`/`replace`/`after` 중 하나 이상 | 내장 커맨드를 감싸는 훅으로 동작 | `dva <name>` (내장이 훅을 실행) |
| 훅 가능 예약어 | 없음 (`command:`만) | **충돌** — `validate` 실패 | `dva run <name>` |
| 훅 불가 예약어 | 무관 | **충돌** — `validate` 실패 | `dva run <name>` |
| `app:build`처럼 `:` 앞이 예약어 | 무관 | **충돌** — `validate` 실패 | **없음** (아래 참조) |

즉 `build`처럼 **예약어이면서 훅 가능한** 이름은 `command:`로 재정의할 수 없고
`replace:`로만 대체할 수 있습니다.

충돌은 **경고가 아니라 에러**입니다 — `dva validate`(= `dva config validate`)가 exit 1로
실패합니다. 다만 `ls`·`manifest`·`run`은 같은 설정을 읽고도 종료 코드 0으로 동작하므로,
설정이 "무효인 상태로 실행 중"일 수 있습니다. 충돌 여부는 `dva validate`로만 확정됩니다.

선언이 버려지는 것은 아닙니다. 짧은 형식(`dva build`)만 내장 커맨드에게 넘어가고, 선언한 커맨드
자체는 `dva run build`로 그대로 실행됩니다. `dva ls`와 `dva manifest`는 충돌한 키를
계속 보여주되 도달 가능한 호출을 함께 표시합니다 — `manifest`의 경우
`usage_example: "dva run build"`와 `shadowed_by_builtin: "build"` 필드입니다.

`app:build`처럼 `:` 앞이 예약어인 경우만 예외로 **어떤 호출로도 도달할 수 없습니다**:
짧은 형식은 내장 커맨드가 아니고, `run` 형식은 `app:`을 서브프로젝트 참조로 읽어
`subproject 'app' not found`로 실패합니다. 구분자를 바꾸는 것(`app-build`)이 유일한
해결책입니다.

```yaml
interaction:
  build:                    # 예약어 + 훅 가능
    replace:                # command: 를 쓰면 충돌 → dva build 는 내장이 실행
      - step: "빌드"          # step: 은 라벨 — 실행할 명령은 run: 에 씁니다
        run: "make build"
    after:
      - step: "완료 알림"
        run: "echo built"

  my-build:                 # 예약어 아님 → 자유롭게 정의
    command: "make build"
```

실행 순서는 `before` → (`replace` 또는 내장 커맨드) → `after`입니다. 훅 스텝
안에서 `dva`를 다시 호출해도 재귀 가드가 걸려 안쪽 호출은 훅 없이 내장 커맨드만
실행합니다.

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
