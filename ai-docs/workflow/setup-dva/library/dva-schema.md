# DVA Configuration Schema Reference

> Canonical schema: `internal/config/schema.json` — always validate against it.

## Critical Rules (MUST follow)

1. **`modes:` NOT `profiles:`** — `profiles:` is deprecated and triggers a warning. Always use `modes:`.
2. **`compose.yml` MUST have `name:`** — Top-level `name: {project}` in compose.yml is required. Without it, `docker compose up` uses the directory name as project, causing port conflicts with DVA's `project_name`.
3. **`version:` field** — Use the current DVA version: `"0.1.22"`. Subprojects should match.
4. **`health_checks`: always provide BOTH `start` and `start_hint`** — `start` enables DVA auto-start (background process with PID tracking). `start_hint` is shown to users when auto-start is not available. If you know the start command, always set both.
5. **Port conventions** — Never use common default ports (5432, 6379, 8080, 3000, etc.) as host ports. Use project-specific port ranges (e.g., 11100-11199).

## dva.yml Structure

```yaml
version: "0.1.22"

environment:                    # Global environment variables
  VAR_NAME: value

env_file:                       # .env file loading (optional)
  files:
    - path: .env.example
      required: true
    - path: .env
      required: false
  priority: before_environment  # before_environment | after_environment
  interpolate: true

compose:
  files:
    - compose.yml               # Primary compose file
    # - compose.tools.yml       # Optional dev tools overlay
  project_name: myapp           # MUST match compose.yml top-level `name:`
  up_options: ["-d", "--wait"]  # Default options for `dva up` (detached mode)
  tags: [infra]                 # Default tags for all services
  services:                     # Per-service metadata (tags, ports, related, hint)
    {service-name}:
      tags: [infra, data]
      ports:
        {host-port}:
          label: "Human-readable label"
          http: true            # nil=auto-detect, true=http://, false=raw
          paths:
            /: "Description"
            /health: "Health endpoint"
      related: [other-service]  # Services shown as hints when this runs
      hint: "Why this service matters"

interaction:
  {name}:
    description: "{human-readable description}"
    service: {compose-service-name}
    command: {shell command to execute}
    tags: [build]               # Optional tags for filtering
    subcommands:                # Optional nested commands
      {sub-name}:
        description: "{description}"
        service: {service}
        command: {sub command}

provision:                      # Setup automation
  {profile-name}:
    - command 1                 # String form
    - step: Step name           # Step form
      run: command
    - step: Multi-command step
      run:
        - command 1
        - command 2

modules:                        # Module imports (.dva/*.yml)
  - module-name

# IMPORTANT: Use `modes:` — NOT `profiles:` (deprecated)
modes:                          # Operational modes (--mode/-M flag)
  {mode-name}:
    description: "{human-readable description}"
    compose_profiles: [profile1]  # Maps to docker compose --profile
    compose_services: [svc1]      # nil=all, []=skip compose, [svcs]=only those
    health_checks: [check1]       # Health checks to run in this mode
    environment:                  # Extra env vars for this mode
      VAR: value
    provision: default            # Suggest provision profile on first run

environments:                   # Environment configs (--env/-E flag)
  {env-name}:
    description: "{human-readable description}"
    environment:                  # Env var overrides
      VAR: value

health_checks:                  # Non-compose service health checks
  {name}:
    type: http|tcp|command
    url: http://localhost:PORT   # for http type
    address: localhost:PORT      # for tcp type
    command: "check command"     # for command type
    start: "start command"       # Auto-start command (background, PID tracked)
    start_hint: "manual start instructions"  # Shown when start is not set
    timeout: 5                   # Health check timeout in seconds (default: 2)
    ready_timeout: 120           # Max wait after auto-start in seconds (default: 30)

checks:                         # Environment checks for `dva doctor`
  - name: "Check description"
    type: docker_socket|file_exists|command
    path: "file/path"           # for file_exists
    command: "shell command"    # for command type
    fix_hint: "How to fix"

subprojects:                    # Subproject references
  {name}:
    path: relative/path
    exclude_tags: [infra]       # Tags to exclude when running from parent

kubectl:                        # Kubernetes config (optional)
  namespace: myapp-dev
```

## compose.yml Requirements

When generating or modifying `compose.yml`, ensure:

```yaml
# REQUIRED: top-level name must match dva.yml compose.project_name
name: myapp

services:
  # ... service definitions ...
```

- **`name:` is mandatory** — prevents directory-name-based project naming conflicts
- Use `compose.yml` (not `docker-compose.yml`) — DVA convention
- No `version:` key — Compose Specification doesn't require it
- All core services should have `healthcheck:` defined
- Use profile-gated services for optional app services: `profiles: ["rust"]`

## Modes & Environments — CLI Flag Reference

### --mode/-M (Modes)
Selects a named mode from `modes:` section. Determines HOW to run infrastructure.

```bash
dva up --mode native     # Skip compose, health checks only
dva up -M docker         # Full docker with compose_profiles
dva up -M hybrid         # Partial compose + health checks
```

**Mode resolution logic:**
1. If `compose_services: []` (empty list) → skip compose entirely, run health_checks only
2. If `compose_services: [svc1, svc2]` → start only listed services
3. If `compose_profiles: [prof1]` → pass `--profile prof1` to docker compose
4. If `environment:` present → merge into compose environment

**Common mode patterns:**
| Mode Name | compose_services | compose_profiles | health_checks | Use Case |
|-----------|-----------------|------------------|---------------|----------|
| infra-only | [list of infra] | — | — | Infra only, app runs natively |
| full-stack | — | [app-profile] | — | Everything in Docker |
| hybrid | [list of infra] | — | [api, worker] | Infra in Docker, app natively |
| native | [] (empty) | — | [api, worker] | No Docker, health checks only |
| dev | [minimal infra] | — | [api] | Minimal infra for dev |

### --env/-E (Environments)
Selects a named environment from `environments:` section. Determines WHAT settings to use.

```bash
dva up --env dev         # Development settings
dva up -E stg            # Staging settings
dva up -M native -E stg  # Combined: native mode + staging env
```

**Environment resolution logic:**
1. Lookup name in `environments:` map
2. Merge `environment:` vars into compose context
3. Can combine with `--mode` (both flags applied independently)

## Health Checks — Auto-Start Pattern

When a service runs natively (not in Docker), define health checks with BOTH `start` and `start_hint`:

```yaml
health_checks:
  api:
    type: http
    url: "http://localhost:11100/health/live"
    start: "cd my-app && cargo run -p api-server"      # DVA auto-starts this
    start_hint: "cd my-app && cargo run -p api-server"  # Shown to user
    timeout: 5
    ready_timeout: 120  # Rust/Go builds need longer timeouts
```

- `start` → DVA runs this in background, tracks PID, logs to `.dva/logs/{name}.log`
- `start_hint` → displayed when `start` is absent or when user runs `dva status`
- `dva down` automatically kills PID-tracked processes

## Interaction Patterns by Project Type

### Go
```yaml
interaction:
  shell:
    description: "Open shell in app container"
    service: app
    command: /bin/bash
  test:
    description: "Run tests"
    service: app
    command: go test ./...
  build:
    description: "Build binary"
    service: app
    command: make build
```

### Python
```yaml
interaction:
  shell:
    description: "Open shell"
    service: app
    command: /bin/bash
  test:
    description: "Run tests"
    service: app
    command: uv run pytest
  migrate:
    description: "Run database migrations"
    service: app
    command: uv run python manage.py migrate
```

### Node.js
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

### Ruby/Rails
```yaml
interaction:
  shell:
    description: "Open shell"
    service: app
    command: /bin/bash
  console:
    description: "Rails console"
    service: app
    command: bundle exec rails console
  test:
    description: "Run tests"
    service: app
    command: bundle exec rspec
```

### Database Services
```yaml
interaction:
  db:
    description: "PostgreSQL console"
    service: postgres
    command: psql -U ${POSTGRES_USER:-dev} -d ${POSTGRES_DB:-devdb}
  redis:
    description: "Redis CLI"
    service: redis
    command: redis-cli
```

## Validation

```bash
# Validate using DVA CLI
dva validate

# Validate specific file
DVA_FILE=path/to/dva.yml dva validate

# Validate against schema directly
# Schema location: internal/config/schema.json
```

## See Also

- `examples/` — Complete configuration examples by use case
- `examples/MIGRATE.md` — Migration guide from legacy configs
- `internal/config/schema.json` — Canonical JSON schema
