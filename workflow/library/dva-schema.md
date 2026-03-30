# DVA Configuration Schema Reference

> Canonical schema: `internal/config/schema.json` — always validate against it.

## Critical Rules (MUST follow)

1. **`modes:` NOT `profiles:`** — Always use `modes:`.
2. **`compose.yml` MUST have `name:`** — Top-level `name: {project}` in compose.yml is required. Without it, `docker compose up` uses the directory name as project, causing port conflicts with DVA's `project_name`.
3. **`version:` field** — Use the current DVA version: `"0.1.29"`. Subprojects should match.
4. **`health_checks`: always provide BOTH `start` and `start_hint`** — `start` enables DVA auto-start (background process with PID tracking). `start_hint` is shown to users when auto-start is not available. If you know the start command, always set both.
5. **Port conventions** — Never use common default ports (5432, 6379, 8080, 3000, etc.) as host ports. Use project-specific port ranges (e.g., 11100-11199).
6. **`stack:` NOT top-level `compose:`** — Infrastructure plugins MUST be declared under `stack:` section. Use `stack:`.
7. **`runner: local` for host commands** — Interaction commands that run on the host (not inside containers) MUST use `runner: local`. Never wrap host commands in `echo 'Run: ...'`.
8. **Complete reserved command list** — These DVA command names are ALL reserved and MUST NOT appear as plain interaction commands: `up`, `down`, `stop`, `restart`, `build`, `clean`, `logs`, `status`, `show`, `ls`, `run`, `config`, `doctor`, `provision`, `add`, `version`. If the project needs a similar function, either use `replace:` hooks (for hookable ones: up/down/stop/restart/build/clean/logs) or rename (e.g., `service-status` instead of `status`, `app-show` instead of `show`).
9. **Health check URLs: literal values only** — Health check `url:` and `address:` fields must use literal port numbers (e.g., `http://localhost:14000/health`), NOT `${VAR:-DEFAULT}` shell variable patterns. DVA resolves environment separately; shell variables in URLs will not be interpolated.
10. **`stack.compose.tags: [infra]`** — The compose-level `tags:` field MUST be present on the primary stack entry. This sets default tags for all services under that entry. Typically `tags: [infra]` for the main infrastructure compose.
11. **Stack compose.files: verify existence** — Every file listed in `stack.{entry}.files` MUST actually exist in the TARGET project. Do NOT assume overlay files exist.
12. **Multi-stack entries: no duplicate base files** — When creating separate stack entries for overlays, each entry should list ONLY its own overlay file(s). Do NOT repeat the base `compose.yml` in every entry. Exception: if an overlay file uses `extends` or `depends_on` referencing base services, include the base but document why.
13. **`services:` is tags-only** — `stack.compose.services` exists ONLY for tag-based filtering (subprojects, modes). Port information is read from compose.yml at runtime.
14. **`endpoints:` for access metadata** — Use the top-level `endpoints:` section to declare user-facing URLs, labels, tags, and sub-paths. For compose services, use `source: "{service}:{host_port}"` to reference compose ports. For non-compose services, specify `url:` directly.
15. **One stack entry + modes, not multi-stack split** — Do NOT create separate stack entries (e.g., `compose` + `compose-full`) to model different operational configurations. Use ONE stack entry with ALL compose files and control service selection via `modes.*.compose_services`. Exception: genuinely different infrastructure backends (e.g., compose for local + kubectl for staging).
16. **`default_mode` for minimal startup** — Always set `default_mode` to a minimal infrastructure mode (e.g., `infra`). Without it, `dva up` starts ALL services from ALL compose files. The default mode should only include core data services (DB, cache). Heavy infrastructure (monitoring, Kafka, Redis Sentinel/Cluster, PostgreSQL replicas, HA setups) MUST be in separate modes like `full-stack` or `full-stack-monitoring`.

## dva.yml Structure

**Canonical section order** (omit unused sections, but keep this order):
`version → environment → env_file → stack → checks → default_mode → suggestion_ignore → modes → environments → health_checks → interaction → provision → modules → subprojects → endpoints`

```yaml
version: "0.1.29"

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

# --- Infrastructure Orchestration Pipeline ---
# Each entry is a plugin with an execution order.
# Plugin type is auto-inferred from entry name (e.g., "compose" → compose plugin).
# Use `plugin:` field when entry name differs from plugin type.
stack:
  compose:                        # Entry name = "compose" → auto-inferred as compose plugin
    order: 10                     # Execution order (ascending)
    files:
      - compose.yml               # Primary compose file
      # - compose.tools.yml       # Optional dev tools overlay
    project_name: myapp           # MUST match compose.yml top-level `name:`
    up_options: ["-d", "--wait"]  # Default options for `dva up` (detached mode)
    tags: [infra]                 # Default tags for all services
    services:                     # Per-service TAG metadata only
      {service-name}:
        tags: [infra, data]       # Used for tag-based filtering (subprojects, modes)
        related: [other-service]  # Services shown as hints when this runs
        hint: "Why this service matters"

  # --- Multi-stack: only for different backends ---
  # Use ONE stack entry + modes for operational variants (NOT separate entries).
  # Only separate stack entries for genuinely different infrastructure:
  # compose:                      # Local Docker infra
  #   order: 10
  #   files: [compose.yml]
  # k8s:                          # Staging Kubernetes
  #   plugin: kubectl
  #   order: 20
  #   namespace: myapp-dev

interaction:
  # --- Container commands (service-based) ---
  {name}:
    description: "{human-readable description}"
    service: {compose-service-name}   # Runs inside this compose service
    command: {shell command to execute}
    tags: [build]                     # Optional tags for filtering
    subcommands:                      # Optional nested commands
      {sub-name}:
        description: "{description}"
        service: {service}
        command: {sub command}

  # --- Host commands (runner: local) ---
  # Use `runner: local` when the command runs on the HOST, not inside a container.
  # Typical for: build, test, lint, fmt, check — commands using local toolchains.
  {name}:
    description: "{human-readable description}"
    command: {shell command}
    runner: local                     # Executes on host, not in container
    tags: [build]

  # --- Namespace-only parent (subcommands only, no direct command) ---
  db:
    description: "Database management"
    subcommands:                      # Parent has no command — acts as namespace
      shell:
        service: postgres             # Container command
        command: psql -U dev -d mydb
      migrate:
        command: "make db-migrate"    # Host command
        runner: local

  # --- Reserved command hooks ---
  # ALL reserved DVA commands (MUST NOT use as plain interaction keys):
  #   up, down, stop, restart, build, clean, logs, status, show, ls, run,
  #   config, doctor, provision, add, version
  # Hookable subset (supports before/replace/after): up, down, stop, restart, build, clean, logs
  # Non-hookable (rename if needed): status→service-status, show→app-show, ls→app-ls
  # `replace:` and `subcommands:` can coexist — subcommands remain accessible.
  # NEVER use `run: "dva <command>"` in replace: hooks — use direct commands instead.
  build:
    replace:                          # Replaces DVA's built-in build
      - step: "Build application"
        run: "make build"
    subcommands:                      # Accessible as `dva build api`, `dva build docker`
      api:
        command: "make build-api"
        runner: local

provision:                      # Setup automation
  {profile-name}:
    - command 1                 # String form (shorthand)
    - step: Step name           # Step form (recommended)
      run: command
    - step: Multi-command step
      run:
        - command 1
        - command 2
    - step: "Information"       # Display-only step (no run:)
      note: "Run 'dva dev' to start development"
    # AVOID: `run: "dva <command>"` in provision — creates bootstrap circular dependency
    # Use direct commands instead: `run: "cargo build"` not `run: "dva build"`

modules:                        # Module imports (.sb/dva/*.yml)
  - module-name

modes:                          # Operational modes (--mode/-M flag)
  {mode-name}:
    description: "{human-readable description}"
    compose_profiles: [profile1]  # Maps to docker compose --profile
    compose_services: [svc1]      # nil=all, []=skip compose, [svcs]=only those
    health_checks: [check1]       # Health checks to run in this mode
    environment:                  # Extra env vars for this mode
      VAR: value
    stack: [entry1, entry2]       # Stack entries to include for this mode
    provision: default            # Suggest provision profile on first run

environments:                   # Environment configs (--env/-E flag)
  {env-name}:
    description: "{human-readable description}"
    environment:                  # Env var overrides
      VAR: value
    stack: [entry1]               # Stack entries to include for this environment

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

endpoints:                      # User-facing access URLs
  # For compose services: use source to reference compose service:port
  {endpoint-name}:
    source: "{service}:{host_port}"  # Auto-resolves URL from compose port
    label: "Human-readable label"
    tags: [app]                      # Inherited from services if omitted
    paths:                           # Optional sub-paths
      /health: "Health endpoint"
      /api/v1: "REST API"
  # For non-compose services: specify url directly
  {endpoint-name}:
    url: "http://localhost:8080"     # Manual URL (local process, external)
    label: "Local Dev Server"
    tags: [app, local]
```

### Endpoints vs Services — When to Use What

| Information | Where it belongs | Example |
|-------------|-----------------|---------|
| Service tags (filtering) | `stack.compose.services` | `postgres: { tags: [infra, data] }` |
| Port labels, URLs | `endpoints` | `db: { source: "postgres:15432", label: "PostgreSQL" }` |
| Sub-paths | `endpoints` | `paths: { /health: "Health check" }` |
| HTTP flag | `endpoints` | Inferred from source port or explicit in url |

**Anti-pattern:** Do NOT create `compose` + `compose-full` as separate stack entries — this duplicates service definitions and compose file references.

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

**Mode resolution logic (evaluated in order):**
1. If `stack: [entry1]` → run only listed stack entries (selects which stack entries to activate)
2. If `compose_services` omitted (nil) → start all services in compose files
3. If `compose_services: []` (empty list) → skip compose entirely, run health_checks only
4. If `compose_services: [svc1, svc2]` → start only listed services
5. If `compose_profiles: [prof1]` → pass `--profile prof1` to docker compose
6. If `environment:` present → merge into compose environment
7. `compose_services` and `compose_profiles` can be combined (both apply independently)

**Single stack + modes pattern (preferred):** Use ONE stack entry with all compose files and control service selection via `compose_services` in modes:
```yaml
stack:
  compose:
    files: [compose.yml, compose.dev-full.yml]  # All overlays in one entry
    services:
      db:    { tags: [infra] }
      redis: { tags: [infra] }
      app:   { tags: [app] }
default_mode: infra                      # dva up (no -M) → minimal infra only
modes:
  infra:
    description: "Core infrastructure only (DB + cache)"
    compose_services: [db, redis]       # Start only infra
  full-stack:
    description: "Full dev environment"  # Omit compose_services = start all
```
**`default_mode` (required):** Specifies which mode is applied when `dva up` is called without `--mode/-M`. This ensures `dva up` starts only minimal infrastructure by default. Heavy services (monitoring, Kafka, Redis Sentinel/Cluster, HA setups) must be in explicit modes like `full-stack` or `full-stack-monitoring`. Users run `dva up -M full-stack` when they need everything.

**Anti-pattern:** Do NOT create `compose` + `compose-full` as separate stack entries — this duplicates service definitions and compose file references.

**`suggestion_ignore` (optional):** Array of glob patterns for Makefile/package.json targets to suppress from `config suggestion` warnings. Use when targets are intentionally not mapped to DVA interactions:
```yaml
suggestion_ignore:
  - "*-release"     # CI-only release builds
  - "clippy*"       # covered by lint interaction
  - "test-e2e-*"    # covered by e2e interaction
```

**`compose_profiles` semantics:**
- `compose_profiles: [rust]` → activate the "rust" profile
- `compose_profiles: []` (empty) → no profiles activated (only default services start)
- Omitted → no `--profile` flag passed (same as empty)

**"All services" mode:** Omit `compose_services` entirely (nil = all). Do NOT use `compose_profiles: []` as a substitute — empty profiles ≠ all services. If a mode should start everything, simply omit both `compose_services` and `compose_profiles`.

**Avoid redundant modes:** Each mode must have a distinct service set. If two modes would start identical services, merge them or differentiate by adding/removing specific services.

**Common mode patterns:**
| Mode Name | compose_services | compose_profiles | health_checks | Use Case |
|-----------|-----------------|------------------|---------------|----------|
| infra-only | [list of infra] | — | — | Infra only, app runs natively |
| full-stack | (omitted = all) | — | — | Everything in Docker |
| full-stack-tools | (omitted = all) | [tools] | — | Everything + dev tools profile |
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

- `start` → DVA runs this in background, tracks PID, logs to `.sb/dva/logs/{name}.log`
- `start_hint` → displayed when `start` is absent or when user runs `dva status`
- `dva down` automatically kills PID-tracked processes

## Interaction Patterns by Project Type

### Runner Selection Rule

Commands fall into two categories:

| Category | `runner` | `service` | When to use |
|----------|----------|-----------|-------------|
| **Container command** | (omit) | required | Runs INSIDE a compose service (db console, redis-cli) |
| **Host command** | `local` | (omit) | Runs on HOST machine (cargo build, go test, npm test) |

**Anti-pattern:** Never use `echo 'Run: ...'` as a command. If the command runs on the host, use `runner: local`.

### Hybrid Pattern (devbox with native app development)

When infra runs in Docker but the app is built/tested natively on the host:

```yaml
interaction:
  # Container commands — run inside compose services
  db:
    description: "PostgreSQL console"
    service: postgres
    command: psql -U ${POSTGRES_USER:-dev} -d ${POSTGRES_DB:-devdb}
  redis:
    description: "Redis CLI"
    service: redis
    command: redis-cli

  # Host commands — run on the developer's machine
  build:
    replace:
      - step: "Build application"
        run: "make build"
  test:
    description: "Run all tests"
    command: "cargo test"
    runner: local
    tags: [test]
  lint:
    description: "Run linter"
    command: "cargo clippy -- -D warnings"
    runner: local
    tags: [quality]
```

### Go (host build)
```yaml
interaction:
  test:
    description: "Run tests"
    command: "go test ./..."
    runner: local
  build:
    replace:
      - step: "Build binary"
        run: "make build"
  lint:
    description: "Run linters"
    command: "golangci-lint run"
    runner: local
```

### Go (container build)
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

### Rust (host build — typical for devbox pattern)
```yaml
interaction:
  test:
    description: "Run all tests"
    command: "cargo test"
    runner: local
    tags: [test]
    subcommands:
      unit:
        description: "Unit tests only"
        command: "cargo test --lib"
        runner: local
      integration:
        description: "Integration tests (requires Docker)"
        command: "cargo test --test '*' -- --test-threads=1"
        runner: local
  build:
    replace:
      - step: "Build release binaries"
        run: "cargo build --release"
  lint:
    description: "Run clippy"
    command: "cargo clippy --all-targets -- -D warnings"
    runner: local
    tags: [quality]
  fmt:
    description: "Format code"
    command: "cargo fmt"
    runner: local
    tags: [quality]
  check:
    description: "Type-check without building"
    command: "cargo check"
    runner: local
    tags: [quality]
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

## Invalid Field Reference

The following fields are NOT valid in dva.yml. Use the correct equivalents:

| Invalid Field | Correct Equivalent | Notes |
|-------------------|--------------------|---------|
| `host_command: "cmd"` | `command: "cmd"` + `runner: local` | Host commands use `runner: local` |
| `compose_up: { tags: [infra] }` | `modes:` section | Tag-based service selection belongs in modes. Resolve tags to explicit service names. |
| `compose_logs: { services: [svc], follow: true }` | `command: "docker compose logs -f svc"` + `runner: local` | For reserved `logs` command, use `replace:` hook instead |
| `endpoints: [...]` in interaction | Top-level `endpoints:` section | Use named-keys format: `{name}: { url: ..., label: ... }` |
| `echo 'Run: ...'` as command | Actual command + `runner: local` | Always execute, never just echo instructions |
| `service: local` | `runner: local` (no `service:`) | Host commands use `runner: local`, not `service: local` |
| `shell: true` (on host commands) | `runner: local` (no `shell: true`) | `runner: local` already executes via shell. Keep `shell: true` only for container commands needing shell interpolation |
| `profiles:` | `modes:` | Use `modes:` |
| top-level `compose:` | `stack: { compose: { ... } }` | Use `stack:` |
| `lifecycle:` | `stack:` | Use `stack:` |

### Subproject Consistency

All subproject dva.yml files MUST follow the same rules as the root:
- Version must match root (`"0.1.29"`)
- Same format rules apply (stack, runner:local, no echo wrappers)

### Provision Step Fields

`compose_up:` and `compose_exec:` ARE valid in the schema as provision step fields. However, `run:` is preferred for clarity and portability:

| Schema-Valid Field | Preferred `run:` Equivalent |
|-------------------|-----------------------------|
| `compose_up: [svc1, svc2]` | `run: "docker compose up -d --wait svc1 svc2"` |
| `compose_exec: "svc cmd"` | `run: "docker compose exec svc cmd"` |

Always use `run:` format. While both forms work at runtime, `run:` is the standard.

### env_file Format

```yaml
env_file:
  files:
    - path: .env.example
      required: true
    - path: .env
      required: false
  priority: before_environment
  interpolate: true
```

### Commonly Misused Fields

These fields ARE valid but are frequently used incorrectly:

- **`shell: true`** — Valid. Enables shell interpolation for multi-line or piped commands. Use when command contains `&&`, `||`, `|`, or shell variables.
- **`compose: { method: "up", profiles: [] }`** — Valid. Controls how DVA invokes docker compose for this command.
- **`environment:`** in interaction — Valid. Per-command environment variable overrides.

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
- `internal/config/schema.json` — Canonical JSON schema
