# DVA Naming and Plan Presets

> `dva.yml` 생성기가 사용하는 결정 규약. 새/rewrite 설정은 현재 `stack` + named
> `plans` 모델만 생성하며 legacy `modes`, `compose_services`, `default_mode`를 만들지 않는다.
> 이 문서의 분류는 생성 규칙이며 DVA validator가 대신 판단해 주지 않는다.

## Service Tags

| Tag | Meaning | Examples |
|-----|---------|----------|
| `infra` | Runtime infrastructure | postgres, redis, rabbitmq, oidc-provider |
| `api` | HTTP/gRPC handlers | api-server, gateway, graphql |
| `worker` | Background processing | sidekiq, consumer, scheduler |
| `ui` | User-facing dev servers | next, vite, storybook |
| `data` | Database/search/object storage | postgres, meilisearch, minio |
| `monitoring` | Observability | prometheus, grafana, jaeger |
| `build` | Build-time only | builder, compiler, asset-pipeline |

Use every tag supported by project evidence. The primary Compose runner has `tags: [infra]`.
Subprojects exclude parent-owned infrastructure with `exclude_tags: [infra]`.

## Capability Closure

Classify discovered services before writing plans. A plan is complete only when every selected
capability's required capabilities have a single lifecycle provider.

| Capability | Typical evidence | Requires | Plan class |
|------------|------------------|----------|------------|
| `database` | DATABASE_URL, migrations, SQL driver | — | core |
| `cache` | REDIS_URL, cache client | — | core when startup requires it |
| `identity` | OIDC issuer/client settings, identity-provider service | `database` unless explicitly external/self-contained | core when normal development authenticates |
| `queue` | broker URL, producer/consumer | — | core only when startup requires it; otherwise extended |
| `object-storage` / `search` | S3, MinIO, Elasticsearch settings | — | core only when startup requires it; otherwise extended |
| `app-native` | verified local run command | its runtime dependencies | local-dev |
| `app-compose` | app service in a verified Compose model | its runtime dependencies | full-stack |
| `observability` | metrics/tracing Compose files | observed targets | opt-in |
| `tools` | admin UI, mail viewer, debug utility | its declared dependencies | opt-in |

Do not infer capability requirements from a service name alone. Verify Compose `depends_on`,
environment variables, manifests, README/Makefile targets, and existing DVA declarations. Do not
select two database providers, or both native and Compose ownership for the same app.

## Provider Resolution

Resolve each required capability deterministically:

1. An explicit project provider contract or documented platform-policy opt-out wins.
2. Preserve mode keeps a working existing lifecycle owner and reports a conflicting binding.
3. Fresh/rewrite mode applies an accepted portfolio/platform binding before generic inference.
4. Otherwise use a verified local service already present in the project's Compose model.
5. If none resolves, report the capability; never invent a service, path, or command.

Rewrite/new generation applies a platform binding only when its target and lifecycle command or
imported plan are verified. A remote/shared provider is represented as an external dependency or
imported plan, not by generating a second local database.

### Injected platform bindings

Portfolio-specific relationships enter discovery as `capability_bindings`; product names and
provider choices do not belong in generic DVA defaults. Each binding supplies `capability`,
`provider`, `consumer`, `ownership`, and concrete `evidence`. Apply this deterministic table:

| Evidence | Result |
|----------|--------|
| Provider and consumer are separate parent-owned stack entries | Include both in one plan and make the consumer entry depend on the provider entry |
| Provider and consumer are services of one Compose entry | Select both services; ordering remains in Compose `depends_on`, not plan `depends_on` |
| Either side is an external/shared runtime or separately imported plan | Emit verified prerequisite commands or health contracts separately; do not pretend two plans are one atomic plan |
| Existing config already owns a local/embedded DB | Preserve it in preserve mode and report the provider-policy mismatch; migrate only in rewrite mode after the orchestrator target is verified |
| Project explicitly opts out of the platform provider | Use the project's verified provider and record the exception in the generated summary |
| No verified provider target exists | Report the unresolved capability; do not guess a service or sibling path |

Bindings are generation inputs and are never emitted as new `dva.yml` keys. Without an injected
binding, use the generic provider-resolution order above.

## Deterministic Plan Matrix

Each plan is a self-contained list. DVA does not compose or inherit plans, so repeat the resolved
entry/service closure explicitly and keep matching `up`, `stop`, and `down` names symmetric.

| Plan | Exact inclusion rule | Default policy |
|------|----------------------|----------------|
| `local-infra` | Verified core providers needed by the normal native workflow; no app, monitoring, or tool entries | Preferred `default_plan` when all providers are local and non-destructive |
| `local-dev` | Complete `local-infra` closure plus every verified native app entry and its dependencies | Generate when native run commands exist; not the default merely because it is convenient |
| `full-stack` | Runtime capability closure plus verified Compose-hosted app services | Explicit opt-in; never the generated default |
| `observability` | The complete base closure needed by observed targets plus monitoring entries/services | Explicit opt-in; not an overlay at runtime |
| `tools` | Only verified tool services plus their capability closure | Explicit opt-in |

Omit a plan whose execution path has no evidence. Add project-specific plans only for a distinct,
documented workflow and use project vocabulary. If several plans exist but no safe local default can
be proven, omit `default_plan` so bare `dva up` fails and forces a name.

### Command policy

| Level | Rule |
|-------|------|
| Required | Discover names with `dva ls`, `dva show`, or `dva manifest -f json`; run lifecycle as `dva up <plan>`, `dva stop <plan>`, and `dva down <plan>` |
| Recommended | Use `dva up local-infra` for dependencies only and `dva up local-dev` for the verified native workflow |
| Discouraged | Bare `dva up`; it hides which `default_plan` was selected and is suitable only for a documented human shortcut |
| Forbidden | `dva up *`, guessed plan names, raw Compose/Kubernetes lifecycle duplicates, and making `full-stack` the generated default |

## Canonical Hybrid Example

```yaml
stack:
  core-compose:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
        tags: [infra]
        services:
          postgres: { tags: [infra, data] }
          redis: { tags: [infra, data] }
          api: { tags: [api] }
  api:
    default_runner: native
    runners:
      native: { run: make run }

plans:
  local-infra:
    entries:
      - name: core-compose
        runner: compose
        services: [postgres, redis]
  local-dev:
    entries:
      - name: core-compose
        runner: compose
        services: [postgres, redis]
      - name: api
        runner: native
        depends_on: [core-compose]
  full-stack:
    entries:
      - name: core-compose
        runner: compose
        services: [postgres, redis, api]
default_plan: local-infra
```

The service lists contain concrete Compose service names, not tags. Native entries are separate
stack entries. Replace every example value with names and commands verified in the target project.

## Environment and Site Presets

| Environment | Meaning |
|-------------|---------|
| `dev` | Local development variables |
| `test` | Test-specific database/logging variables |
| `stg` | Staging-shaped variables used for local validation |
| `prd` | Production-shaped variables used for local validation |

Use `environments` for variable sets and `sites` for host/location differences. DVA is a local
development and maintenance tool: a remote site does not grant deployment or incident authority.
Match existing project vocabulary such as `.env.staging` when evidence is stronger than this preset.
