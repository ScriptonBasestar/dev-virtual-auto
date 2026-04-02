# DVA Reference Examples by Language/Pattern

> guided improve Stage 30에서 자동 참조하는 레퍼런스 스니펫.
> 프로젝트 archetype과 development_pattern에 맞는 섹션을 선택하여 구조적 가이드로 사용.
> **값(포트, 서비스명, 경로)은 복사하지 말 것** — 구조와 패턴만 참조.

## File Header Template

모든 dva.yml은 이 헤더로 시작:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/ScriptonBasestar/dev-virtual-auto/master/schema.json
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

1. `version:`
2. `environment:` (선택)
3. `env_file:`
4. `stack:` (services tags-only, NO ports)
5. `checks:` (dva doctor)
6. `applications:` (앱 서버/워커 — port, run native/docker, dev, build, health)
7. `default_mode:` (dva up 기본 모드)
8. `suggestion_ignore:` (선택)
9. `modes:` (applications 필드로 앱 전략 지정 가능)
10. `health_checks:` (non-app 서비스 전용)
11. `interaction:` (organized by category)
12. `provision:` (default, full, reset)
13. `subprojects:` (if devbox pattern)
14. `endpoints:` (user-facing access URLs)

---

## Rust — Hybrid Pattern (devbox)

> archetype: service-daemon | microservices
> development_pattern: hybrid (infra Docker, app native via cargo)
> Typical: Rust workspace with multiple crates, compose for DB/cache/monitoring

### Stack + Endpoints

```yaml
stack:
  compose:
    order: 10
    tags: [infra]
    files:
      - compose.yml
      # - compose.monitoring.yml  # optional overlays
    project_name: {project}
    up_options: ["-d", "--wait"]
    services:                      # Tags only — NO ports here
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
default_mode: infra-only    # dva up → minimal infra only

modes:
  infra-only:
    description: "Infrastructure only (DB, Redis)"
    compose_services: [postgres, redis]
  full-stack:
    description: "All services in Docker"
    compose_profiles: [rust]
    provision: default
  hybrid:
    description: "Infra in Docker, app runs natively"
    compose_services: [postgres, redis]
    health_checks: [api, worker]
    environment:
      DATABASE_URL: "postgresql://{user}:{pass}@localhost:{port}/{db}"
      REDIS_URL: "redis://localhost:{port}"
  dev:
    description: "Minimal infra for fast native development"
    compose_services: [postgres, redis]
    health_checks: [api]
    environment:
      DATABASE_URL: "postgresql://{user}:{pass}@localhost:{port}/{db}"
      REDIS_URL: "redis://localhost:{port}"
```

### Applications (Rust hybrid — native dev servers)

```yaml
applications:
  api:
    description: "REST API server"
    tags: [app, api]
    port: {PORT}
    dir: "{workspace}"
    run:
      native: "cargo run --release -p {exact-package-name}"
      docker:
        service: {api-service}
        profile: rust
    dev:
      native: "cargo watch -x 'run -p {exact-package-name}'"
    build:
      native: "cargo build --release -p {exact-package-name}"
      docker:
        service: {api-service}
    health:
      type: http
      url: "http://localhost:{PORT}/healthz"
      timeout: 5
      ready_timeout: 120    # Rust compilation needs longer
    depends_on: []           # No app dependencies (infra is in stack)
    environment:
      RUST_LOG: "info"
  worker:
    description: "Background job processor"
    tags: [app, worker]
    dir: "{workspace}"
    run: "cargo run -p {worker-package-name}"
    dev: "cargo watch -x 'run -p {worker-package-name}'"
    build: "cargo build -p {worker-package-name}"
    health:
      type: command
      command: "pgrep -f {binary-name}"
      timeout: 5
      ready_timeout: 120
    depends_on: [api]        # Worker starts after API
```

> **Migration note:** If health_checks already has `start:` commands for app servers, migrate them to `applications:` section. The `applications:` section provides richer lifecycle control (stop/down/restart/build/dev mode).

### Health Checks (non-app services only)

```yaml
# health_checks is now primarily for external/non-app services.
# App servers should use applications.{name}.health instead.
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

Same structure as Rust section above — use `stack.compose.services` for tags only, `endpoints:` for port/URL metadata.

### Applications (Go hybrid)

```yaml
applications:
  api:
    description: "HTTP API server"
    tags: [app, api]
    port: {PORT}
    run:
      native: "go run ./cmd/{binary}"
      docker:
        service: {api-service}
    dev:
      native: "air"                  # or: go run ./cmd/{binary} with fsnotify watcher
    build:
      native: "make build"          # or: go build -o bin/{name} ./cmd/{name}
    health:
      type: http
      url: "http://localhost:{PORT}/health"
      timeout: 5
      ready_timeout: 60             # Go compiles faster than Rust
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

Same structure as Rust section — use `stack.compose.services` for tags only, `endpoints:` for port/URL metadata.

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

Same structure as Rust section — use `stack.compose.services` for tags only, `endpoints:` for port/URL metadata.

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

> Pattern: devbox parent manages shared infra, each subproject has its own dva.yml

```yaml
# Root dva.yml — manages shared infrastructure
subprojects:
  {component-1}:
    path: {component-1}
    exclude_tags: [infra]    # Prevents duplicate infra when running from parent
  {component-2}:
    path: {component-2}
    exclude_tags: [infra]

# Subproject dva.yml — app-specific commands only, NO stack section needed
# version MUST match root
# NOTE: Subprojects do NOT re-declare the parent's compose stack.
# The parent's `subprojects.{name}.exclude_tags: [infra]` prevents duplicate infra.
# Subproject dva.yml only needs interaction commands for app-specific operations.
version: "0.1.29"

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
