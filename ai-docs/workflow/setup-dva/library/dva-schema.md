# DVA Configuration Schema Reference

> Canonical schema: `internal/config/schema.json` — always validate against it.

## Critical Rules (MUST follow)

1. **`modes:` NOT `profiles:`** — `profiles:` is deprecated and triggers a warning. Always use `modes:`.
2. **`compose.yml` MUST have `name:`** — Top-level `name: {project}` in compose.yml is required. Without it, `docker compose up` uses the directory name as project, causing port conflicts with DVA's `project_name`.
3. **`version:` field** — Use the current DVA version: `"0.1.26"`. Subprojects should match.
4. **`health_checks`: always provide BOTH `start` and `start_hint`** — `start` enables DVA auto-start (background process with PID tracking). `start_hint` is shown to users when auto-start is not available. If you know the start command, always set both.
5. **Port conventions** — Never use common default ports (5432, 6379, 8080, 3000, etc.) as host ports. Use project-specific port ranges (e.g., 11100-11199).
6. **`stack:` NOT top-level `compose:`** — Infrastructure plugins MUST be declared under `stack:` section. Top-level `compose:` is deprecated (auto-converted but generates warnings).
7. **`runner: local` for host commands** — Interaction commands that run on the host (not inside containers) MUST use `runner: local`. Never wrap host commands in `echo 'Run: ...'`.

## dva.yml Structure

```yaml
version: "0.1.26"

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
# `stack:` replaces the deprecated top-level `compose:` / `kubectl:` sections.
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

  # --- Multi-stack entries (multiple compose files as separate entries) ---
  # Use when compose files serve different purposes (e.g., infra vs dev-full):
  # compose:                        # Primary infra
  #   order: 10
  #   files: [compose.yml]
  #   project_name: myapp
  # compose-dev-full:               # Dev overlay (custom name → explicit plugin)
  #   plugin: compose
  #   order: 20
  #   files: [compose.yml, compose.dev-full.yml]
  #   project_name: myapp

  # --- Other plugin examples ---
  # kubectl:                      # Auto-inferred as kubectl plugin
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
  # Hookable DVA commands: up, down, stop, restart, build, clean, logs
  # Each supports: before (pre-hook), replace (full override), after (post-hook)
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
    - command 1                 # String form (legacy)
    - step: Step name           # Step form (preferred)
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

# IMPORTANT: Use `modes:` — NOT `profiles:` (deprecated)
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

**Mode resolution logic (evaluated in order):**
1. If `stack: [entry1]` → run only listed stack entries (selects which stack entries to activate)
2. If `compose_services` omitted (nil) → start all services in compose files
3. If `compose_services: []` (empty list) → skip compose entirely, run health_checks only
4. If `compose_services: [svc1, svc2]` → start only listed services
5. If `compose_profiles: [prof1]` → pass `--profile prof1` to docker compose
6. If `environment:` present → merge into compose environment
7. `compose_services` and `compose_profiles` can be combined (both apply independently)

**Multi-stack mode pattern:** When a project has multiple compose stack entries (e.g., `compose` for infra, `compose-dev-full` for full dev overlay), use `stack:` in modes to select which entry to activate:
```yaml
modes:
  infra-only:
    stack: [compose]              # Only the base infra compose
    compose_services: [db, redis]
  dev-full:
    stack: [compose-dev-full]     # Uses overlay compose with app services
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

- `start` → DVA runs this in background, tracks PID, logs to `.dva/logs/{name}.log`
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

## Non-Standard Field Migration

When upgrading existing dva.yml files, you may encounter fields that are not in the current schema. These were either from early DVA versions or custom extensions.

### Interaction Non-Standard Fields

| Non-Standard Field | Standard Equivalent | Example |
|-------------------|--------------------|---------|
| `host_command: "cmd"` | `command: "cmd"` + `runner: local` | `host_command: "make test"` → `command: "make test"` + `runner: local` |
| `compose_up: { tags: [infra] }` | Convert to `modes:` section | Tag-based service selection belongs in modes, not interaction. Resolve tags to explicit service names. |
| `compose_logs: { services: [svc], follow: true }` | `command: "docker compose logs -f svc"` + `runner: local` | For reserved `logs` command, use `replace:` hook instead |
| `endpoints: [...]` in interaction | Move to top-level `endpoints:` section | Convert array-of-objects to named-keys format: `{name}: { url: ..., label: ... }` |
| `echo 'Run: ...'` as command | Actual command + `runner: local` | Always execute, never just echo instructions |
| `service: local` | `runner: local` (remove `service:`) | Old convention for host commands |
| `shell: true` (on host commands) | `runner: local` (remove `shell: true`) | `runner: local` already executes via shell. Keep `shell: true` only for container commands needing shell interpolation |

### Prefixed Command Migration

Commands prefixed to avoid DVA reserved names should be migrated:

```yaml
# BEFORE (workaround for reserved command collision)
app-build:
  command: "echo 'Run: make build'"
  service: some-service
app-clean:
  command: "echo 'Run: make clean'"
  service: some-service

# AFTER (use replace: hooks on the actual reserved command)
build:
  replace:
    - step: "Build application"
      run: "make build"
  subcommands:           # subcommands coexist with replace:
    api:
      command: "make build-api"
      runner: local
clean:
  replace:
    - step: "Clean build artifacts"
      run: "make clean"
```

### Subproject Cascade Upgrade

When upgrading a root dva.yml, ALL subproject dva.yml files MUST be checked and upgraded too:
- Version must match root (`"0.1.26"`)
- Same format rules apply (stack, runner:local, no echo wrappers)
- `service: local` (old convention) → `runner: local`

### Provision Step Fields

`compose_up:` and `compose_exec:` ARE valid in the schema as provision step fields. However, `run:` is preferred for clarity and portability:

| Schema-Valid Field | Preferred `run:` Equivalent | When to Convert |
|-------------------|-----------------------------|-----------------|
| `compose_up: [svc1, svc2]` | `run: "docker compose up -d --wait svc1 svc2"` | When upgrading for consistency |
| `compose_exec: "svc cmd"` | `run: "docker compose exec svc cmd"` | When upgrading for consistency |

**When upgrading, ALWAYS convert to `run:` format.** While both forms work at runtime, `run:` is the standard and `compose_up:`/`compose_exec:` create inconsistency in upgraded configs.

### env_file Format

```yaml
# BEFORE (string — old format)
env_file: ".env"

# AFTER (object — current format)
env_file:
  files:
    - path: .env.example
      required: true
    - path: .env
      required: false
  priority: before_environment
  interpolate: true
```

### Valid but Often Misused Fields

These fields ARE in the schema but are frequently used incorrectly:

- **`shell: true`** — Valid. Enables shell interpolation for multi-line or piped commands. Use when command contains `&&`, `||`, `|`, or shell variables.
- **`compose: { method: "up", profiles: [] }`** — Valid. Controls how DVA invokes docker compose for this command.
- **`environment:`** in interaction — Valid. Per-command environment variable overrides.

## Deprecated Formats — Migration Guide

### Top-level `compose:` → `stack:` (0.1.26)

The top-level `compose:` section is deprecated. Wrap it under `stack:`:

```yaml
# BEFORE (deprecated)
compose:
  files: [compose.yml]
  project_name: myapp
  services:
    postgres:
      tags: [infra, data]

# AFTER (current)
stack:
  compose:
    order: 10
    files: [compose.yml]
    project_name: myapp
    services:
      postgres:
        tags: [infra, data]
```

All compose fields remain the same — just nest under `stack: { compose: { ... } }` and add `order:`.

### `lifecycle:` → `stack:` (0.1.26)

The `lifecycle:` key was an intermediate name. Replace with `stack:`:
- `lifecycle:` → `stack:`
- `modes.*.lifecycle` → `modes.*.stack`

### `profiles:` → `modes:` (0.1.16)

Replace top-level `profiles:` with `modes:`. Same structure, just renamed.

### Prefixed command workarounds → Reserved command hooks (0.1.26)

Early DVA users prefixed commands to avoid reserved name conflicts. Migrate these to `replace:` hooks:

```yaml
# BEFORE (workaround — prefixed names)
interaction:
  app-build:
    description: "Build application"
    service: app
    command: "echo 'Run: cargo build --release'"
  app-clean:
    description: "Clean artifacts"
    service: app
    command: "echo 'Run: cargo clean'"

# AFTER (current — replace hooks + runner: local)
interaction:
  build:
    replace:
      - step: "Build application"
        run: "cargo build --release"
    subcommands:
      docker:
        description: "Build Docker images"
        command: "docker compose build"
        runner: local
  clean:
    replace:
      - step: "Clean artifacts"
        run: "cargo clean"
```

Common prefixed names: `app-build` → `build`, `app-clean` → `clean`. `app-logs` can stay (not reserved).

### Module directory `.dva/` → `.sb/dva/` (0.1.26)

Module files moved from `.dva/*.yml` to `.sb/dva/*.yml`.

## Validation

```bash
# Validate using DVA CLI
dva validate

# Detect legacy format and show migration guide
dva migrate

# Validate specific file
DVA_FILE=path/to/dva.yml dva validate

# Validate against schema directly
# Schema location: internal/config/schema.json
```

## See Also

- `examples/` — Complete configuration examples by use case
- `internal/config/schema.json` — Canonical JSON schema
