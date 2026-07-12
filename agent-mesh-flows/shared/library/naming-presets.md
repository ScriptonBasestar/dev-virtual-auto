# DVA Naming Presets

> improve 워크플로우가 dva.yml을 생성할 때 참조하는 네이밍 규약.
> 새 프로젝트마다 일관된 이름과 패턴을 적용하기 위한 프리셋.

## Service Tags

서비스를 분류하는 기본 단위. 하나의 서비스에 여러 태그 가능.

| Tag | Description | Examples |
|-----|-------------|----------|
| `infra` | Infrastructure dependencies | postgres, redis, rabbitmq, kafka, zookeeper |
| `api` | HTTP/gRPC request handlers | web, api-server, gateway, graphql |
| `worker` | Background job processors | sidekiq, celery, consumer, scheduler |
| `ui` | Frontend dev servers | next, vite, webpack-dev-server, storybook |
| `data` | Search/storage beyond DB | elasticsearch, meilisearch, minio, s3 |
| `monitoring` | Observability | grafana, prometheus, jaeger, loki |
| `build` | Build-time only | builder, compiler, asset-pipeline |

**Rules:**
- `infra` is the base tag — DB, cache, queue는 항상 `infra`
- 태그는 `stack.<entry>.runners.compose` -> `services` 와 interaction에 모두 적용
- 프로젝트에 해당 역할이 없으면 해당 태그 생략

## Infrastructure Tier Classification

서비스를 기본 모드에 포함할지 결정하는 기준. `default_mode`는 Tier 1만 포함해야 한다.

| Tier | Classification | Tags | Examples | default_mode |
|------|---------------|------|----------|-------------|
| **Tier 1: Core Data** | 앱 실행에 필수 — 없으면 API가 시작 불가 | `data` | postgres, mysql, redis, memcached | **포함** |
| **Tier 2: Event/Queue** | 비동기 처리에 필요 — API는 없어도 동작 가능 | `kafka`, `queue` | kafka, rabbitmq, nats, pulsar | **제외** (worker 모드에) |
| **Tier 3: Observability** | 모니터링/추적 — 기능과 무관 | `monitoring` | prometheus, grafana, jaeger, alertmanager, loki, tempo | **제외** (full-stack에) |
| **Tier 4: Storage** | 백업/파일 저장 — 대부분의 개발 흐름에 불필요 | `storage` | minio, elasticsearch | **제외** (full-stack에) |

**판단 기준:**
1. 이 서비스 없이 `cargo run` / `go run` / `npm start`로 앱이 시작되는가?
2. API 코드에서 `Option<T>`으로 선언되어 있는가? → Tier 2 이상
3. 앱 코드와 직접 통신하지 않는가? → Tier 3-4

**모드별 포함 예시:**
```yaml
modes:
  infra:                    # Tier 1 only
    compose_services: [postgres, redis]
  dev:                      # Tier 1 + 2 (worker 개발 시)
    compose_services: [postgres, redis, kafka]
  full-stack:               # Tier 1 + 2 + 3 + 4
    compose_services: [postgres, redis, kafka, prometheus, grafana, jaeger, minio]
```

**DVA validation**: `dva config validate`가 default_mode에 Tier 2+ 서비스가 포함되면 경고를 발생시킨다.

## Mode Presets

`--mode/-M` 플래그용. 어떤 서비스를 어떻게 실행할지 결정.

### Core Modes (항상 적용 가능)

> compose_services, health_checks 컬럼은 **개념 표기** — 실제 dva.yml 생성 시 프로젝트의 서비스 이름으로 대체.

| Name | Description | compose_services | health_checks | Use Case |
|------|-------------|-----------------|---------------|----------|
| `infra` | Infrastructure only | infra 태그 서비스 나열 | — | **기본값.** DB/cache만 Docker, 앱은 네이티브 개발 |
| `full-stack` | All services in Docker | omit (=all) | — | CI, 데모, 또는 네이티브 셋업이 복잡할 때 |
| `hybrid` | Infra Docker + app native | infra 태그 서비스 나열 | app 관련 체크 | Infra는 Docker, 앱은 로컬에서 직접 실행 |
| `native` | No Docker at all | `[]` (empty) | 전체 체크 | Docker 없이 전부 로컬 |

### Domain Modes (프로젝트에 서비스 그룹이 있을 때)

| Name | Description | Includes | When to use |
|------|-------------|----------|-------------|
| `backend` | Server-side full stack | api + worker + infra | **웹앱** — "프론트 반대편" 전체 |
| `server` | Request-serving processes | api (+ infra) | **서비스/데몬** — API가 아닌 서버도 포함 |
| `worker` | Background processors | worker + infra | 워커만 독립 실행할 때 |
| `ui` | Frontend development | ui + api (dependency) | 프론트 개발 시 백엔드를 API로 연결 |

### Mode 선택 가이드

```
웹앱 (SPA + API + Worker):
  → infra, backend, ui, full-stack

서비스/데몬 (gRPC, TCP server):
  → infra, server, full-stack

마이크로서비스 (여러 서비스):
  → infra, {service-name}, full-stack

단순 앱 (API only):
  → infra, full-stack
```

### Container-First Modes (앱이 Docker 안에서 실행될 때)

> Django, Rails 등 컨테이너 내부에서 개발하는 프로젝트용. 여러 compose overlay 파일을 조합.

| Name | Description | compose_services | When |
|------|-------------|-----------------|------|
| `infra` | Dependencies only (DB, cache, auth) | infra 태그 서비스 | 앱을 Docker 밖에서 연결할 때 |
| `full-stack` | Core app + infra | core compose 서비스 | 기본 개발 모드 |
| `full-stack-tools` | Core + dev tools | + adminer, redis-commander 등 | DB/cache 관리 필요 시 |
| `full-stack-monitoring` | Core + observability | + prometheus, grafana 등 | 성능/메트릭 확인 시 |
| `all` | Every service | `~` (null = 전체) | 전체 스택 데모/테스트 |

**Rules:**
- `backend`과 `server`는 겹침 — 프로젝트당 하나만 선택
  - 웹 프로젝트 → `backend` 선호
  - 서비스/데몬 프로젝트 → `server` 선호
- `infra`는 항상 포함 (가장 기본적인 모드)
- 프로젝트가 단순하면 core modes만으로 충분
- 모드 이름은 프로젝트의 실제 어휘에 맞춤 (README, Makefile 등 참조)
- Container-first 프로젝트는 compose overlay 파일 조합으로 모드 구성
- Hybrid 프로젝트는 compose_services + health_checks 조합으로 모드 구성

## Env Presets

`--env/-E` 플래그용. 환경변수 오버라이드 세트.

| Name | Description | Typical Variables |
|------|-------------|-------------------|
| `dev` | Development | `LOG_LEVEL=debug`, `ENABLE_HOT_RELOAD=true`, `DEBUG=true` |
| `test` | Testing | `DATABASE_URL=*_test`, `LOG_LEVEL=warn`, `DISABLE_CACHE=true` |
| `stg` | Staging config (local) | Staging API endpoints, `LOG_LEVEL=info` |
| `prd` | Production config (local) | Production-like config for local validation |

**Rules:**
- `dev`는 기본값 — 별도 지정 없으면 dev 환경
- DVA는 로컬 개발 도구이므로 stg/prd는 "로컬에서 해당 설정으로 실행"의 의미
- 프로젝트에 stg/prd 구분이 불필요하면 `dev`, `test`만으로 충분
- env 이름은 프로젝트의 기존 .env 파일 네이밍에 맞춤 (`.env.staging` → `stg`)

## Combination Examples

CLI 사용 예시:
```bash
dva up -M infra -E dev       # 웹앱: 인프라만 + 개발 환경
dva up -M backend -E test    # 웹앱: 백엔드 + 테스트 환경
dva up -M full-stack -E stg  # 전체 스택 + 스테이징 설정
dva up -M server -E dev      # 서비스: 서버 + 개발 환경
dva up -M native -E dev      # Docker 없이 전부 로컬
```

웹앱 프로젝트 dva.yml modes/environments 생성 예시:
```yaml
modes:
  infra:
    description: "Infrastructure only (DB, Redis)"
    compose_services: [postgres, redis]
  backend:
    description: "Backend stack (infra + API + worker)"
    compose_services: [postgres, redis, api, worker]
  ui:
    description: "Frontend dev (infra + API as dependency)"
    compose_services: [postgres, redis, api]
    health_checks: [frontend]
  full-stack:
    description: "All services in Docker"
    stack: [compose]              # Include all stack entries (explicit)

environments:
  dev:
    description: "Development"
    environment:
      LOG_LEVEL: debug
  test:
    description: "Testing"
    environment:
      LOG_LEVEL: warn
      DATABASE_SUFFIX: _test
```

멀티-스택 프로젝트 (compose + kubectl) 모드 예시:
```yaml
stack:
  compose:
    default_runner: compose
    order: 10
    runners:
      compose:
        files: [compose.yml]
        # ...
  kubectl:
    order: 20
    namespace: myapp-dev
    # ...

modes:
  local:
    description: "Local dev — compose only"
    stack: [compose]              # Skip kubectl
  cluster:
    description: "K8s cluster — kubectl only"
    stack: [kubectl]              # Skip compose
  full:
    description: "Both compose + kubectl"
    # stack: omitted = all entries
```
