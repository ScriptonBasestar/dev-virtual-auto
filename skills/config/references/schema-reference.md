# DVA Configuration Schema Reference

> Canonical schema: `internal/config/schema.json` — always validate against it.

## Critical Rules (MUST follow)

1. **`plans:` are the current execution model** — Use named plans for new or
   rewritten configs. `modes:` and `applications:` are migration-only legacy
   sections.
2. **`compose.yml` MUST have `name:`** — Top-level `name: {project}` in compose.yml is required. Without it, `docker compose up` uses the directory name as project, causing port conflicts with DVA's `project_name`.
3. **`version:` field** — Optional, and it declares the **minimum** DVA version the config requires of its reader — not the version of the binary that generated it. Omit it for no compatibility gate. Pinning it to the running CLI version makes every generated config refuse to load on an older DVA. Subproject `version:` is checked against the running DVA independently; it is never compared to root. Canonical source: `internal/config/version.go`.
4. **`health_checks`: `start` and `start_hint` are both optional** — `start` enables DVA auto-start (background process with PID tracking). `start_hint` is human-readable text shown by `dva status` when the service is not ready. If `start` is set, `start_hint` is optional (only needed when the hint text should differ from the start command, e.g., friendlier instructions). If only `start_hint` is set, no auto-start occurs — DVA just displays the hint. Setting both to identical values is redundant and triggers a validation warning.
5. **Port conventions** — Never use common default ports as host ports: 2181, 3000, 3306, 5432, 6379, 8080, 8443, 9090, 9092, 9200, 15672, 27017. Pick a project-specific range instead (e.g. 11100-11199).
6. **`stack:` NOT top-level `compose:`** — Infrastructure compose MUST be declared under `stack.<entry>.runners.compose`.
7. **`runner: local` for host commands** — Interaction commands that run on the host (not inside containers) MUST use `runner: local`. Never wrap host commands in `echo 'Run: ...'`.
8. **Complete reserved command list** — These DVA command names are ALL reserved and MUST NOT appear as plain interaction commands: `up`, `down`, `stop`, `restart`, `build`, `clean`, `logs`, `status`, `show`, `ls`, `run`, `config`, `doctor`, `provision`, `version`, `console`, `infra`, `app`, `stack`, `help`, `compose`, `validate`, `manifest`, `ktl`, `ssh`, `completion`, `init`. If the project needs a similar function, either use `replace:` hooks (for hookable ones: up/down/stop/restart/build/clean/logs) or rename (e.g., `service-status` instead of `status`, `app-show` instead of `show`). Canonical source: `internal/config/reserved.go`.
9. **Health check URLs: literal values only** — Health check `url:` and `address:` fields must use literal port numbers (e.g., `http://localhost:14000/health`), NOT `${VAR:-DEFAULT}` shell variable patterns. DVA resolves environment separately; shell variables in URLs will not be interpolated.
10. **`stack.<entry>.runners.compose.tags: [infra]`** — The compose-level `tags:` field MUST be present on the primary compose runner. This sets default tags for all services under that entry. Typically `tags: [infra]` for the main infrastructure compose.
11. **Stack compose.files: verify existence** — Every file listed in `stack.{entry}.runners.compose.files` MUST actually exist in the TARGET project. Do NOT assume overlay files exist.
12. **Multi-stack entries: no duplicate base files** — When creating separate stack entries for overlays, each entry should list ONLY its own overlay file(s). Do NOT repeat the base `compose.yml` in every entry. Exception: if an overlay file uses `extends` or `depends_on` referencing base services, include the base but document why.
13. **`services:` is tags-only** — The `services` map under `stack.{entry}.runners.compose` exists ONLY for tag-based filtering. Port information is read from compose.yml at runtime.
14. **`endpoints:` for access metadata** — Use the top-level `endpoints:` section to declare user-facing URLs, labels, tags, and sub-paths. For compose services, use `source: "{service}:{host_port}"` to reference compose ports. For non-compose services, specify `url:` directly.
15. **One stack entry + plans, not multi-stack split** — Do NOT create separate stack entries (e.g., `compose` + `compose-full`) to model different operational configurations. Use ONE stack entry with ALL compose files and control service selection via `plans.*.entries[].services`. Exception: genuinely different infrastructure backends (e.g., compose for local + kubectl for staging).
16. **Single lifecycle owner** — A Compose service is started only by its
    compose stack entry through a plan. Do not also generate an
    `applications.*.run.docker.service`, standalone docker runner, raw
    `docker compose up` interaction, or provision start step for that service.
    A `docker` runner means standalone `docker run`, not Docker Compose.

## dva.yml Structure

**Canonical section order** (omit unused sections, but keep this order):
`version → vars → environment → env_file → stack → plans → environments → sites → checks → applications → default_mode → suggestion_ignore → modes → health_checks → interaction → provision → modules → subprojects → endpoints → infra → ssh → devcontainer`

`checks`, `applications`, `default_mode`, `suggestion_ignore`, and `modes` are legacy-compatible sections. Preserve them only when needed during migration; new configurations should use plans and stack runners.

```yaml
version: "{CURRENT_DVA_VERSION}"   # Use the version from the prompt's CRITICAL section

vars:                           # Template variables for interpolation (optional)
  PROJECT_NAME: myapp
  PORT_BASE: "11100"

environment:                    # Global environment variables
  VAR_NAME: value

env_file:                       # .env file loading (optional)
  files:
    - path: .env.example
      required: true
    - path: .env
      required: false
  required: false               # optional: mark all listed files required

# --- Infrastructure Orchestration Declarations ---
# Each entry declares one or more named runners. Plans choose what actually runs.
stack:
  infra:                          # Logical compose bundle
    default_runner: compose
    runners:
      compose:
        files:
          - compose.yml           # Primary compose file
          # - compose.tools.yml   # Optional dev tools overlay
        project_name: myapp       # MUST match compose.yml top-level `name:`
        up_options: ["-d", "--wait"]
        tags: [infra]             # Default tags for all services
        services:                 # Per-service TAG metadata only
          {service-name}:
            tags: [infra, data]   # Used for tag-based filtering

  # --- Multi-stack: only for different backends ---
  # Use ONE stack entry + plans for operational variants (NOT separate entries).
  # Only separate stack entries for genuinely different infrastructure:
  # infra:
  #   default_runner: compose
  #   runners:
  #     compose:
  #       files: [compose.yml]
  # k8s:                          # Staging Kubernetes
  #   plugin: kubectl
  #   namespace: myapp-dev

applications:                   # Long-running app processes (API servers, workers, etc.)
  {app-name}:                   # Short name: api, worker, web, scheduler
    description: "{human-readable description}"
    tags: [app]                 # For filtering
    port: 11200                 # Listening port (shown in dva app ls) — REQUIRED
    dir: "relative/path"        # Working directory (default: config dir)
    depends_on: [other-app]     # Startup dependency ordering
    environment:                # App-specific env vars
      PORT: "11200"
    run:                        # How to run in production-like mode
      native: "cargo run --release -p api"
      docker:
        service: api-rs         # Compose service name
        profile: rust           # Compose profile to activate
    dev:                        # How to run in dev mode (hot-reload, debug, etc.)
      native: "cargo watch -x 'run -p api'"
      docker: "docker compose up api-dev"  # String shorthand for docker command
    build:                      # How to build the app
      native: "cargo build -p api"
      docker:
        service: api-rs
        command: "cargo build"
    health:                     # Readiness check (probe fields shared with top-level health_checks; applications additionally support `required`)
      type: http
      url: "http://localhost:11200/health"
      timeout: 5
      ready_timeout: 120
      required: false           # default false (advisory on timeout); true promotes to strict failure (non-zero exit)

  # --- String shorthand for exec paths ---
  # run/dev/build accept a string shorthand that sets the native path only:
  # run: "cargo run -p api"  ← equivalent to  run: { native: "cargo run -p api" }

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
  #   config, doctor, provision, version, console, infra,
  #   app, stack, help, compose, validate, manifest, ktl, ssh, completion, init
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
      note: "Run 'dva up' to start development"
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
    applications: native          # App strategy: "native"/"docker" (string) or per-app map
    # applications:               # Per-app strategy override:
    #   api: native
    #   worker: docker
    #   _default: native          # Fallback for unlisted apps
    # NOTE: Without `applications:` field, apps will NOT auto-start in this mode.
    # Modes that should start apps (hybrid, full-stack, etc.) MUST set this field.

environments:                   # Environment configs (--env/-E flag)
  {env-name}:
    description: "{human-readable description}"
    environment:                  # Env var overrides
      VAR: value
    stack: [entry1]               # Stack entries to include for this environment

plans:                          # Named deployment plans (optional)
  {plan-name}:
    description: "{human-readable description}"
    environment: dev             # Environment to activate
    site: local                  # Site to activate
    vars:                        # Plan-specific variables
      KEY: value
    entries:                     # Ordered stack entries for this plan
      - name: infra
        runner: compose
        order: 10
        depends_on: []
        services: [postgres, redis]

sites:                          # Host-based execution conditions (optional)
  {site-name}:
    description: "{human-readable description}"
    vars:
      KEY: value
    entry_overrides:             # Site-specific stack entry overrides
      {entry-name}:
        runner: local
        vars:
          KEY: value

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
    import:                     # Optional; non-empty imports require child dva.yml
      plans: [local-dev]
      interactions: [shell]
      provision: [setup]

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

infra:                          # Remote infrastructure references (optional)
  {infra-name}:
    git: "https://github.com/org/repo"  # Git repository URL
    ref: main                          # Branch or tag
    path: "k8s/overlays/dev"           # Path within repo

ssh:                            # SSH configuration (optional)
  agent_image: "ghcr.io/org/ssh-agent:latest"
```

### Endpoints vs Services — When to Use What

| Information | Where it belongs | Example |
|-------------|-----------------|---------|
| Service tags (filtering) | `stack.<entry>.runners.compose` -> `services` | `postgres: { tags: [infra, data] }` |
| Port labels, URLs | `endpoints` | `db: { source: "postgres:15432", label: "PostgreSQL" }` |
| Sub-paths | `endpoints` | `paths: { /health: "Health check" }` |
| HTTP flag | `endpoints` | Inferred from source port or explicit in url |

**Anti-pattern:** Do NOT create `compose` + `compose-full` as separate stack entries — this duplicates service definitions and compose file references.

## compose.yml Requirements

When generating or modifying `compose.yml`, ensure:

```yaml
# REQUIRED: top-level name must match dva.yml stack.<entry>.runners.compose.project_name
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

**Single stack + plans pattern (preferred):** Use ONE stack entry with all compose files and control service selection via `plans.*.entries[].services`:
```yaml
stack:
  infra:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml, compose.dev-full.yml]  # All overlays in one entry
        services:
          db:    { tags: [infra] }
          redis: { tags: [infra] }
          app:   { tags: [app] }
plans:
  local-infra:
    entries:
      - name: infra
        runner: compose
        services: [db, redis]
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

## Health Checks — Start & Hint Patterns

When a service runs natively (not in Docker), use `start` and/or `start_hint` depending on the use case:

### Pattern 1: Auto-start only (most common)

```yaml
health_checks:
  api:
    type: http
    url: "http://localhost:11100/health/live"
    start: "cd my-app && cargo run -p api-server"  # DVA auto-starts this
    timeout: 5
    ready_timeout: 120
```

DVA runs the command in background, tracks PID, logs to `.sb/dva/logs/{name}.log`. `dva down` kills PID-tracked processes. When the service is starting, `dva status` shows the log path instead of a hint text. If you want a human-readable hint shown when the service is not ready, add `start_hint` with different text (Pattern 2).

### Pattern 2: Auto-start with different hint text

```yaml
health_checks:
  api:
    type: http
    url: "http://localhost:11100/health/live"
    start: "cd my-app && cargo run -p api-server"
    start_hint: "Run the API server from my-app/"  # Friendlier text for dva status
    timeout: 5
    ready_timeout: 120
```

Use this when you want `dva status` to show a human-friendly message that differs from the actual start command.

### Pattern 3: Hint only (no auto-start)

```yaml
health_checks:
  external-api:
    type: http
    url: "http://localhost:9090/health"
    start_hint: "Start the external API manually: see docs/setup.md"
    timeout: 5
```

No auto-start — DVA only displays the hint text when the service is not ready.

### Rules

- `start` → DVA runs this in background, tracks PID, logs to `.sb/dva/logs/{name}.log`
- `start_hint` → displayed by `dva status` when the service is not ready
- If `start` is set without `start_hint`, no hint text is shown (only log path while starting)
- Setting both to identical values is redundant and triggers a validation warning
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
| top-level `compose:` or `stack.<entry>.compose` | `stack.<entry>.runners.compose` | Use named runners |
| `lifecycle:` | `stack:` | Use `stack:` |

### Subproject Consistency

Imported or directly executed subproject dva.yml files MUST follow the same rules as the root:
- `version:` is optional and is **not** compared to root — DVA checks each file's floor
  against the running binary independently, so do not require the two to agree
- Same format rules apply (stack, runner:local, no echo wrappers)
- Declared-only subprojects without import entries (`import` omitted or `import: {}`) may be initialized later

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
```

### Commonly Misused Fields

These fields ARE valid but are frequently used incorrectly:

- **`shell: true`** — Valid. Enables shell interpolation for multi-line or piped commands. Use when command contains `&&`, `||`, `|`, or shell variables.
- **`compose: { method: "up", profiles: [] }`** — Valid. Controls how DVA invokes docker compose for this command.
- **`environment:`** in interaction — Valid. Per-command environment variable overrides. On the
  compose path these reach the **container**: DVA injects the merged declared environment as
  `-e KEY=VALUE` on every compose subcommand that accepts it (`run`, `exec`, including `steps:`
  which always builds `exec`). `up` — the method used when `profiles:` is configured — has no
  `-e` flag and is excluded, as is kubectl (`kubectl exec` has no env flag). What crosses is the
  whole merged variable set — `env_file`, global `vars`, `environment:`, site vars, plan vars,
  `--var`, and the command's own `environment:` — but only keys that were declared somewhere:
  an OS value overrides a declared key, it does not add one, so undeclared host variables stay
  out. `DVA_*` is excluded. A declared key overrides the image's own value — declaring `PATH`
  replaces the container's `PATH` on exec.

## Validation

```bash
# Validate using DVA CLI
dva validate

# Validate specific file
DVA_FILE=path/to/dva.yml dva validate

# Validate against schema directly
# Schema location: internal/config/schema.json
```

## Lifecycle CLI Commands

DVA has three levels of lifecycle management:

### `dva up` / `dva down` — Top-level lifecycle (with hooks)

```bash
dva up                    # Start infra (default_mode) — runs before/replace/after hooks
dva up -M full-stack      # Start all infra in Docker
dva down                  # Stop and remove infra
dva down -v               # Also remove volumes
dva stop                  # Stop infra (preserve state for quick restart)
dva restart               # Stop then start
```

`dva up` executes hook steps (before/replace/after from interaction section) then delegates to the stack orchestrator. This is the recommended entry point.

### `dva stack` — Infrastructure orchestrator (no hooks)

```bash
dva stack up                    # Start all stack entries (bypasses hooks)
dva stack up compose            # Start a specific stack entry by name
dva stack up -T infra           # Filter by tag (--tags/-T)
dva stack up -M hybrid          # Apply mode filtering
dva stack stop                  # Stop all (preserve state — fast restart)
dva stack down                  # Remove all stack resources
dva stack down -v               # Also remove volumes
dva stack status                # Show stack entry statuses
dva stack status compose        # Status for specific entry
dva stack log compose           # View logs for a stack entry
```

Stack entries are started in `order:` sequence. The orchestrator supports tag-based and mode-based filtering. Exports from earlier entries are available to later entries via environment accumulation.

### `dva app` — Application process manager

```bash
dva app ls                      # List all applications with status/health/port/PID
dva app up                      # Start all applications (dependency order)
dva app up myapp                # Start specific application
dva app up myapp --dev          # Start in dev mode (hot-reload)
dva app stop myapp              # Stop (SIGTERM, preserves PID for quick restart)
dva app down myapp              # Stop and remove PID/log files
dva app restart myapp           # Stop then start
dva app restart myapp --dev     # Restart in dev mode
dva app build myapp             # Build application (native)
dva app build myapp --docker    # Build application (docker)
dva app log myapp               # Show last 100 lines of app log
```

**Startup order:** Applications with `depends_on` are started in topological waves — independent apps within the same wave launch concurrently. Health checks are evaluated after each app starts if `health:` is configured and `--wait` is not disabled.

**Stop vs Down semantics:**

| Command | Signal | PID file | Log file | Use case |
|---------|--------|----------|----------|----------|
| `dva app stop` | SIGTERM | preserved | preserved | Development iteration (quick restart) |
| `dva app down` | SIGTERM | **deleted** | **deleted** | Full cleanup |

Same semantics apply to `dva stack stop` vs `dva stack down`.

### Typical Development Flow

```bash
dva up                   # 1. Start infrastructure (DB, Redis, etc.)
dva app up --dev         # 2. Start app servers in dev mode
# ... develop ...
dva app restart api      # 3. Restart specific app after changes
dva app stop             # 4. Stop apps (quick restart later)
dva down                 # 5. Full infrastructure teardown
```

### `dva up` vs `dva stack up` — When to Use

| Scenario | Command | Why |
|----------|---------|-----|
| Normal development startup | `dva up` | Runs hooks, applies default_mode |
| Debugging orchestrator behavior | `dva stack up` | Bypasses hooks, direct control |
| Starting specific entries | `dva stack up <name>` | Targeted infrastructure |
| Tag-based filtering | `dva stack up -T infra` | Subset by tag |

## See Also

- `examples/` — Complete configuration examples by use case
- `internal/config/schema.json` — Canonical JSON schema
