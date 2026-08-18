# DVA Naming Presets

> improve 워크플로우가 dva.yml을 생성할 때 참조하는 네이밍 규약 — 현재 `plans` +
> `environments` + `sites` 모델 기준. 새/rewrite 설정에 **절대 legacy `modes:` /
> `compose_services` / `default_mode`를 생성하지 않는다**(shared-guardrails 규칙 2,
> skills/config 참조). `modes`는 explicit migration(preserve) 시에만 유지.

## Service Tags

서비스를 분류하는 기본 단위. 하나의 서비스에 여러 태그 가능. `stack.<entry>.runners.compose`
의 `services`와 interaction에 모두 적용. 프로젝트에 해당 역할이 없으면 해당 태그 생략.

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
- `infra` is the base tag — DB/cache/queue는 항상 `infra`
- subproject에서 `exclude_tags: [infra]`로 부모 infra 중복 방지(규칙 26)
- compose runner의 주 entry는 `tags: [infra]` 필수(규칙 16)

## Service Tier (default_plan 선택 기준)

`default_plan`과 각 plan에 어떤 서비스를 포함할지 결정. Tier 1만 기본 plan에 포함 권장.
Tier는 생성 시점의 판단 기준일 뿐 DVA가 검사하지 않는다 — default plan에 Tier 4 서비스를
넣어도 `dva validate`는 통과한다(2026-08-03 측정). 그래서 이 선택은 여기서 옳아야 한다.

| Tier | Classification | Tags | Examples | 기본 plan 포함? |
|------|---------------|------|----------|----------------|
| **Tier 1: Core Data** | 앱 실행에 필수 — 없으면 시작 불가 | `infra` | postgres, mysql, redis, memcached | **포함** (local-infra/local-dev) |
| **Tier 2: Event/Queue** | 비동기 — API는 없어도 동작 | `infra` | kafka, rabbitmq, nats, pulsar | local-dev / full-stack |
| **Tier 3: Observability** | 모니터링/추적 — 기능과 무관 | `monitoring` | prometheus, grafana, jaeger, loki | observability / full-stack |
| **Tier 4: Storage** | 백업/파일 저장 — 대부분 불필요 | `data` | minio, elasticsearch | full-stack |

**판단 기준:**
1. 이 서비스 없이 `cargo run` / `go run` / `npm start`로 앱이 시작되는가? → 아니오면 Tier 1
2. API 코드에서 `Option<T>` / behind-feature로 선언되어 있는가? → Tier 2+
3. 앱 코드와 직접 통신하지 않는가? → Tier 3-4

## Plan Presets

`dva up <plan>` / `down <plan>` 용(named lifecycle, 규칙 33). **하나의 stack entry +
plans로 운영 변형을 모델링**(dva-schema 15 — multi-stack split 금지). `plans.*.entries[].services`
로 서비스 선택. 새/rewrite 설정에는 실행 가능한 `plans:` 섹션 필수(규칙 2).

| Plan | Includes | Use Case |
|------|----------|----------|
| `local-infra` | Tier 1(infra-tagged) only | **기본값 후보.** DB/cache만 Docker, 앱은 native 실행 |
| `local-dev` | Tier 1 + dev apps(native) | 일반 개발. infra는 Docker, 앱은 로컬 직접 실행 |
| `full-stack` | 모든 서비스 Docker | CI, 데모, 또는 native 셋업이 복잡할 때 |
| `observability` | monitoring tier 추가 | 성능/메트릭 — 보통 `full-stack` 위 overlay |

**Rules:**
- `default_plan`은 보통 `local-dev`(앱이 Docker 필수면 `full-stack`)
- 워크스페이스 실행 순서는 `plans.entries[].depends_on` + `order`(규칙 30)
- Compose 서비스는 반드시 compose stack entry + plan이 기동(단일 lifecycle owner, 규칙 28)
- 프로젝트 특화 plan은 서비스 그룹에 따라 추가 가능 — 이름은 프로젝트 어휘(README, Makefile 참조)

**단일 stack + plans 패턴(권장):**
```yaml
stack:
  compose:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
        tags: [infra]
        services:        # tag metadata only — 포트는 endpoints에서
          postgres: { tags: [infra, data] }
          redis: { tags: [infra] }

plans:
  local-infra:
    description: "Infra only (DB, Redis)"
    entries:
      - services: [data]        # tag filter → postgres
  local-dev:
    description: "Infra + native app"
    entries:
      - services: [infra]
  full-stack:
    description: "All services in Docker"
    entries:
      - services: [infra, api, worker, ui]
default_plan: local-dev
```

**멀티 백엔드(compose + kubectl) — 다른 인프라 백엔드일 때만 별도 entry(dva-schema 15 예외):**
```yaml
stack:
  compose:
    runners: { compose: { files: [compose.yml], tags: [infra] } }
  kubectl:
    namespace: myapp-dev

plans:
  local:    { entries: [ { services: [infra] } ] }       # compose only
  cluster:  { entries: [ { services: [kubectl] } ] }     # kubectl only
  full:     { entries: [ { services: [infra] }, { services: [kubectl] } ] }
default_plan: local
```

## Environment Presets

`environments:` 섹션용(dev/stg/prd 변수 오버라이드). host/loc 차이는 `sites:`(규칙 35).
plan은 `environment:`/`site:`로 environment·site를 선택한다(dva-schema 212-213).

| Name | Description | Typical Variables |
|------|-------------|-------------------|
| `dev` | Development | `LOG_LEVEL=debug`, `ENABLE_HOT_RELOAD=true`, `DEBUG=true` |
| `test` | Testing | `DATABASE_URL=*_test`, `LOG_LEVEL=warn`, `DISABLE_CACHE=true` |
| `stg` | Staging config (local) | Staging API endpoints, `LOG_LEVEL=info` |
| `prd` | Production config (local validation) | Production-like config for local validation |

**Rules:**
- `dev`가 기본 — 별도 지정 없으면 dev
- DVA는 로컬 개발·유지보수 도구다(규칙 41) — stg/prd는 "로컬에서 해당 설정으로 실행"을
  뜻하며, 그 환경을 조작할 권한이 아니다
- 프로젝트에 stg/prd 구분이 불필요하면 `dev`, `test`만으로 충분
- env 이름은 프로젝트 기존 `.env` 네이밍에 맞춤(`.env.staging` → `stg`)

## Combination Examples

CLI(named plan — 규칙 33):
```bash
dva up local-dev       # 기본: infra Docker + 앱 native
dva up full-stack      # 전체 Docker
dva up local-infra     # 인프라만
```

```yaml
plans:
  local-infra:
    description: "Infra only"
    entries: [{ services: [data] }]
  local-dev:
    description: "Infra + native app"
    entries: [{ services: [infra] }]
  full-stack:
    description: "All in Docker"
    entries: [{ services: [infra, api, worker, ui] }]
default_plan: local-dev

environments:
  dev:  { environment: { LOG_LEVEL: debug } }
  test: { environment: { LOG_LEVEL: warn, DATABASE_SUFFIX: _test } }
```
