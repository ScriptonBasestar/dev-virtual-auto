# DVA Reference Examples by Language/Pattern

> guided improve Stage 30에서 자동 참조하는 레퍼런스 스니펫.
> 프로젝트 archetype과 development_pattern에 맞는 섹션을 선택하여 구조적 가이드로 사용.
> **값(포트, 서비스명, 경로)은 복사하지 말 것** — 구조와 패턴만 참조.

## File Header Template

모든 dva.yml은 이 헤더로 시작:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/ScriptonBasestar/dva/master/internal/config/schema.json
# =============================================================================
# DVA Configuration — {project-name} ({role: Root DevBox | Subproject})
# =============================================================================
# Pattern: {devbox | standalone | monorepo}
# Root manages: {what this config manages}
# Development pattern: {hybrid | container-first | native}
# =============================================================================
```

## Section Order (Canonical)

모든 dva.yml은 이 순서를 따름 (미사용 섹션은 생략 가능):

정본은 `internal/config/validate_warnings.go`의 `canonicalSectionOrder` — `dva validate`가
이 순서를 벗어난 섹션에 경고를 냅니다.

1. `version:`
2. `vars:` (선택)
3. `environment:` (선택)
4. `env_file:`
5. `stack:` (services tags-only, NO ports — 앱 서버/워커도 여기에 native runner로)
6. `plans:` (선택)
7. `default_plan:` (선택)
8. `environments:` (선택)
9. `sites:` (선택)
10. `checks:` (dva doctor)
11. `default_mode:` (dva up 기본 모드 — legacy)
12. `suggestion_ignore:` (선택)
13. `modes:` (legacy — 마이그레이션 중에만 유지)
14. `health_checks:` (DVA가 실행하지 않는 외부 서비스 전용)
15. `interaction:` (organized by category)
16. `provision:` (default, full, reset)
17. `modules:` (선택)
18. `subprojects:` (if devbox pattern)
19. `endpoints:` (user-facing access URLs)
20. `infra:` (선택)
21. `ssh:` (선택)
22. `devcontainer:` (선택)

`applications:`는 제거됐습니다 (docs/43). 앱은 `stack.<name>.default_runner: native` +
`runners.native`로 선언하고, 기존 파일은 `dva config migrate`로 변환하세요.

---

## Rust — Hybrid Pattern (devbox)

> archetype: service-daemon | microservices
> development_pattern: hybrid (infra Docker, app native via cargo)
> Typical: Rust workspace with multiple crates, compose for DB/cache/monitoring

### Stack + Endpoints

```yaml
stack:
  compose:
    default_runner: compose
    order: 10
    tags: [infra]
    runners:
      compose:
        files:
          - compose.yml
          # - compose.monitoring.yml  # optional overlays
        project_name: {project}
        up_options: ["-d", "--wait"]
        services:                    # Tags only — NO ports here
          postgres:  { tags: [infra, data] }
          redis:     { tags: [infra, data] }
          {app-api}: { tags: [app, rust, backend] }

# Endpoints — user-facing access URLs (port metadata lives here, NOT in services)
endpoints:
  postgres:
    source: "postgres:{PORT}"
    label: "PostgreSQL"
    tags: [infra, data]
  redis:
    source: "redis:{PORT}"
    label: "Redis"
    tags: [infra, data]
  {app-api}:
    source: "{app-api}:{PORT}"
    label: "API"
    tags: [app]
    paths:
      /healthz: "Health check"
```

### Modes

```yaml
default_mode: hybrid    # dva up → infra Docker + apps native (most common dev workflow)

modes:
  # Pure native: no Docker at all (e.g., SQLite for local-only dev)
  native:
    description: "All natively, no Docker (SQLite/in-memory)"
    stack: [api, worker]          # App entries only — no compose entry
    environment:
      DB_TYPE: sqlite
      DATABASE_URL: "sqlite:./dev.db"

  # Hybrid: infrastructure in Docker, app entries native (DEFAULT for most devbox)
  hybrid:
    description: "Infra in Docker, apps run natively"
    stack: [compose, api, worker] # Docker infra + native app entries
    compose_services: [postgres, redis]
    environment:
      DB_TYPE: postgres
      DATABASE_URL: "postgresql://{user}:{pass}@localhost:{port}/{db}"
      REDIS_URL: "redis://localhost:{port}"
    provision: default

  # Mixed: some entries native, some as compose services
  hybrid-mixed:
    description: "API in Docker, frontend natively"
    stack: [compose, frontend]    # `api` is a compose service here, not an entry
    compose_services: [postgres, redis, api]
    environment:
      API_URL: "http://localhost:{port}"

  # All Docker: production-like environment
  docker:
    description: "All services in Docker"
    stack: [compose]              # No native app entries selected
    compose_profiles: [app]
    provision: default

  # Infrastructure only: for CI or shared infra testing
  infra-only:
    description: "Infrastructure only (DB, Redis)"
    compose_services: [postgres, redis]
```

> **Key pattern:** a mode's `stack:` is the whole selection. Modes used to carry a separate
> `applications:` strategy field, which meant an app could be started by the mode's app
> strategy *and* by a compose entry in the same run. App entries are stack entries now, so
> one list decides everything and the double-owner shape cannot be written.
>
> **Environment switching:** Native entries connect via `localhost:{host_port}`.
> Compose services use internal service names (e.g., `postgres:5432`).

### App entries (Rust hybrid — native dev servers)

App servers are stack entries with a `native` runner, declared alongside the compose entry
above. Ordering lives in the plan that runs them, not in the declaration.

```yaml
stack:
  api:
    description: "REST API server"
    tags: [app, api]
    default_runner: native
    runners:
      native:
        dir: "{workspace}"
        build: "cargo build --release -p {exact-package-name}"
        run: "cargo run --release -p {exact-package-name}"
        env:
          RUST_LOG: "info"
    health_checks:
      api:
        type: http
        url: "http://localhost:{PORT}/healthz"
        timeout: 5
        ready_timeout: 120    # Rust compilation needs longer
  worker:
    description: "Background job processor"
    tags: [app, worker]
    default_runner: native
    runners:
      native:
        dir: "{workspace}"
        build: "cargo build -p {worker-package-name}"
        run: "cargo run -p {worker-package-name}"
    health_checks:
      worker:
        type: command
        command: "pgrep -f {binary-name}"
        timeout: 5
        ready_timeout: 120

plans:
  dev:
    entries:
      - name: compose
      - name: api
        order: 10
      - name: worker
        order: 20            # Worker starts after API
```

> **Hot-reload variants:** an entry declares one `run:` command, so `cargo watch` is its own
> entry (e.g. `api-watch`) selected by a different plan — not a second command inside `api`.
>
> **Migration note:** `health_checks.<name>.start` still auto-starts a background process
> and is the right home for a service DVA only supervises. Declare a stack entry when you
> want the full lifecycle (stop/down/restart/build) instead.

### Health Checks (services DVA does not own)

```yaml
# health_checks at top level is for external services DVA probes but does not run.
# An app server DVA starts should be a stack entry with entry-scoped health_checks.
health_checks:
  external-api:
    type: http
    url: "http://localhost:{PORT}/health"
    start_hint: "Start the external service: see docs/setup.md"
    timeout: 5
```

### Interaction

```yaml
interaction:
  # --- Database ---
  db:
    description: "PostgreSQL console"
    service: postgres
    command: "psql -U {user} -d {db}"
    subcommands:
      reset:
        description: "Drop and recreate database"
        service: postgres
        command: "psql -U {user} -d postgres -c 'DROP DATABASE IF EXISTS {db};' && psql -U {user} -d postgres -c 'CREATE DATABASE {db};'"
      tables:
        description: "Show table structure"
        service: postgres
        command: "psql -U {user} -d {db} -c '\\dt+'"
  redis:
    description: "Redis CLI"
    service: redis
    command: "redis-cli"

  # --- Build (reserved — replace + subcommands) ---
  build:
    replace:
      - step: "Build all binaries (release)"
        run: "cd {workspace} && cargo build --release"
    tags: [build]
    subcommands:
      api:
        description: "Build API binary"
        runner: local
        command: "cd {workspace} && cargo build --release --bin {api-binary}"
        tags: [build]
      docker:
        description: "Build Docker images"
        runner: local
        command: "docker compose build"
        tags: [build]

  # --- Test ---
  test:
    description: "Run all tests"
    runner: local
    command: "cd {workspace} && cargo test"
    tags: [test]
    subcommands:
      unit:
        description: "Unit tests only"
        runner: local
        command: "cd {workspace} && cargo test --lib"
        tags: [test]
      integration:
        description: "Integration tests (requires Docker)"
        runner: local
        command: "cd {workspace} && cargo test --test '*' -- --test-threads=1"
        tags: [test]
      e2e:
        description: "End-to-end tests"
        runner: local
        command: "cd {workspace} && cargo test -p {e2e-crate}"
        tags: [test]

  # --- Quality ---
  lint:
    description: "Run clippy (warnings-as-errors)"
    runner: local
    command: "cd {workspace} && cargo clippy -- -D warnings"
    tags: [quality]
  fmt:
    description: "Format code"
    runner: local
    command: "cd {workspace} && cargo fmt"
    tags: [quality]
  check:
    description: "Type-check without building"
    runner: local
    command: "cd {workspace} && cargo check"
    tags: [quality]

  # --- Logs (reserved) ---
  logs:
    replace:
      - step: "Tail compose logs"
        run: "docker compose logs -f"

  # --- Clean (reserved) ---
  clean:
    replace:
      - step: "Clean artifacts and volumes"
        run: "cd {workspace} && cargo clean && docker compose down -v"
```

### Provision

```yaml
provision:
  default:
    - step: "Copy environment file"
      run: "cp -n .env.example .env 2>/dev/null || true"
    - step: "Start infrastructure"
      run: "docker compose up -d --wait postgres redis"
    - step: "Install dev tools"
      run: "cargo install sqlx-cli --no-default-features --features rustls,postgres --locked 2>/dev/null || true"
    - step: "Fetch dependencies"
      run: "cd {workspace} && cargo fetch"
    - step: "Run migrations"
      run: "cd {workspace}/{api-crate} && sqlx migrate run"
    - step: "Verify"
      run: "cd {workspace} && cargo check"
  full:
    - step: "Copy environment file"
      run: "cp -n .env.example .env 2>/dev/null || true"
    - step: "Start full stack"
      run: "docker compose --profile rust up --build -d"
    - step: "Wait for API"
      run: "sleep 20 && curl -sf http://localhost:{PORT}/healthz || echo 'API not ready'"
  reset:
    - step: "Stop all services and remove volumes"
      run: "docker compose down -v"    # Add overlay files if used: -f compose.yml -f compose.X.yml
    - step: "Clean Rust artifacts"
      run: "cd {workspace} && cargo clean"
    - step: "Re-setup"
      note: "Run 'dva provision default' to re-setup"
```

---

## Go — Hybrid Pattern (devbox)

> archetype: service-daemon | web-app
> development_pattern: hybrid
> Typical: Go module with cmd/ binaries, compose for DB/cache

### Stack + Endpoints

Same structure as Rust section above — use `stack.<entry>.runners.compose` -> `services` for tags only, `endpoints:` for port/URL metadata.

### App entries (Go hybrid)

```yaml
stack:
  api:
    description: "HTTP API server"
    tags: [app, api]
    default_runner: native
    runners:
      native:
        build: "make build"          # or: go build -o bin/{name} ./cmd/{name}
        run: "go run ./cmd/{binary}"
    health_checks:
      api:
        type: http
        url: "http://localhost:{PORT}/health"
        timeout: 5
        ready_timeout: 60            # Go compiles faster than Rust

  # Hot-reload as its own entry, selected by a dev plan
  api-watch:
    description: "HTTP API server (air hot-reload)"
    tags: [app, api, dev]
    default_runner: native
    runners:
      native:
        run: "air"                   # or: go run ./cmd/{binary} with an fsnotify watcher
```

### Key Differences from Rust

```yaml
# Interaction — Go toolchain
interaction:
  build:
    replace:
      - step: "Build binary"
        run: "make build"   # or: go build -o bin/{name} ./cmd/{name}
  test:
    description: "Run tests"
    runner: local
    command: "go test ./..."
    tags: [test]
    subcommands:
      race:
        description: "Tests with race detector"
        runner: local
        command: "go test -race ./..."
  lint:
    description: "Run linters"
    runner: local
    command: "golangci-lint run"
    tags: [quality]
  fmt:
    description: "Format code"
    runner: local
    command: "gofmt -w ."
    tags: [quality]
```

---

## Python/Django — Container-First Pattern

> archetype: web-app
> development_pattern: container-first (app runs in Docker)
> Typical: Django/FastAPI with Docker for everything

### Stack + Endpoints

Same structure as Rust section — use `stack.<entry>.runners.compose` -> `services` for tags only, `endpoints:` for port/URL metadata.

### Key Differences

```yaml
# Modes — container-first uses compose_profiles for overlays
default_mode: infra    # dva up → minimal infra only

modes:
  infra:
    description: "Dependencies only (DB, cache, auth)"
    compose_services: [postgres, redis]
  full-stack:
    description: "Core app + infra"
  full-stack-tools:
    description: "Core + dev tools (adminer, redis-commander)"
    compose_profiles: [tools]

# Health checks — typically not needed (app is in Docker)
# Use compose healthcheck: instead

# Interaction — runs INSIDE container
interaction:
  shell:
    description: "Open shell in app"
    service: app
    command: /bin/bash
  test:
    description: "Run tests"
    service: app
    command: uv run pytest
  migrate:
    description: "Run migrations"
    service: app
    command: uv run python manage.py migrate
  manage:
    description: "Django management"
    service: app
    command: uv run python manage.py

# Provision — starts containers
provision:
  default:
    - step: "Copy environment file"
      run: "cp -n .env.example .env 2>/dev/null || true"
    - step: "Start infrastructure"
      run: "docker compose up -d --wait postgres redis"
    - step: "Start application"
      run: "docker compose up -d --wait app"
    - step: "Run migrations"
      run: "docker compose exec app uv run python manage.py migrate"
```

---

## Node.js/TypeScript — Hybrid or Container-First

> archetype: web-app | ui
> development_pattern: varies

### Stack + Endpoints

Same structure as Rust section — use `stack.<entry>.runners.compose` -> `services` for tags only, `endpoints:` for port/URL metadata.

### Hybrid (local dev server)

```yaml
health_checks:
  dev:
    type: http
    url: "http://localhost:{PORT}"
    start: "pnpm dev"
    timeout: 5
    ready_timeout: 30

interaction:
  build:
    replace:
      - step: "Build project"
        run: "pnpm build"
  test:
    description: "Run tests"
    runner: local
    command: "pnpm test"
    tags: [test]
  lint:
    description: "Run linter"
    runner: local
    command: "pnpm lint"
    tags: [quality]
  dev:
    description: "Start dev server"
    runner: local
    command: "pnpm dev"
```

### Container-First

```yaml
interaction:
  shell:
    description: "Open shell"
    service: app
    command: /bin/sh
  test:
    description: "Run tests"
    service: app
    command: pnpm test
  dev:
    description: "Start dev server"
    service: app
    command: pnpm dev
```

---

## Multi-Component (devbox with subprojects)

> Pattern: devbox parent manages shared infra; imported subprojects have their own dva.yml

```yaml
# Root dva.yml — manages shared infrastructure
subprojects:
  {component-1}:
    path: {component-1}
    exclude_tags: [infra]    # Prevents duplicate infra when running from parent
    import:                  # Only add non-empty imports after {component-1}/dva.yml exists
      interactions: [test]
  {component-2}:
    path: {component-2}
    exclude_tags: [infra]

# Subproject dva.yml — app-specific commands only, NO stack section needed
# NOTE: Subprojects do NOT re-declare the parent's compose stack.
# The parent's `subprojects.{name}.exclude_tags: [infra]` prevents duplicate infra.
# Subproject dva.yml only needs interaction commands for app-specific operations.
# `version:` is optional and is NOT compared to the parent's — DVA checks each file's
# floor against the running binary independently. Omitted here on purpose.

# stack: is OPTIONAL in subprojects — omit if the subproject relies entirely on parent infra
# Only add stack: if the subproject has its own compose services

interaction:
  # Only app-specific commands — no db/redis (those are in parent)
  build:
    replace:
      - step: "Build {component}"
        run: "make build"
  test:
    description: "Run tests"
    runner: local
    command: "{test command}"
    tags: [test]
  lint:
    description: "Lint"
    runner: local
    command: "{lint command}"
    tags: [quality]

provision:
  default:
    - step: "Fetch dependencies"
      run: "{dependency install command}"
    - step: "Check"
      run: "{type check command}"
```

---

## Checks (dva doctor) — Common Patterns

```yaml
checks:
  # Always include
  - name: Docker daemon accessible
    type: docker_socket
    fix_hint: "Start Docker Desktop or ensure dockerd is running"
  - name: .env file exists
    type: file_exists
    path: .env
    fix_hint: "cp .env.example .env"
  - name: compose.yml exists
    type: file_exists
    path: compose.yml
    fix_hint: "compose.yml should be at repo root"

  # Language-specific
  # Rust:
  - name: Rust toolchain available
    type: command
    command: "rustc --version >/dev/null 2>&1"
    fix_hint: "curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh"
  # Go:
  - name: Go toolchain available
    type: command
    command: "go version >/dev/null 2>&1"
    fix_hint: "Install Go: https://go.dev/dl/"
  # Node.js:
  - name: Node.js available
    type: command
    command: "node --version >/dev/null 2>&1"
    fix_hint: "Install Node.js via mise or nvm"
  # Docker Compose:
  - name: Docker Compose available
    type: command
    command: "docker compose version >/dev/null 2>&1"
    fix_hint: "Install Docker Compose plugin"
```
