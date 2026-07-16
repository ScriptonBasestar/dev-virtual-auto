# DVA Advanced Patterns

Conceptual documentation for DVA's configuration model, execution architecture, and advanced usage patterns.

## Plans (current model)

Plans are executable names. Each entry selects a declared stack runner, service
subset, order, and dependencies. Environments and sites are referenced by the plan.

```yaml
plans:
  local-dev:
    environment: dev
    site: local
    entries:
      - name: infra
        runner: compose
        order: 10
        services: [postgres, redis]
      - name: api
        runner: native
        order: 20
        depends_on: [infra]
```

Run symmetrically with `dva up local-dev`, `dva stop local-dev`, and
`dva down local-dev`.

## Modes (`--mode` / `-M`) — legacy migration reference

Modes define the deprecated execution model. Preserve them only during migration;
do not generate them for new configurations.

```yaml
modes:
  backend:
    description: "Backend services only"
    compose_profiles: [backend]
    compose_services: [api, postgres, redis]
    health_checks: [api-server]
    stack: [compose]              # run only these stack entries
    environment:
      LOG_LEVEL: debug
  native:
    description: "No compose, local services only"
    compose_services: []          # empty array = skip compose entirely
    health_checks: [local-api]
```

Mode properties:

| Property | Description |
|----------|-------------|
| `compose_profiles` | Docker Compose profiles to activate |
| `compose_services` | Specific services to include (empty = skip compose) |
| `health_checks` | Health checks to run for this mode |
| `stack` | Limit to specific stack entries |
| `environment` | Mode-specific environment variables |
| `provision` | Mode-specific provision profile |
| `build` | Mode-specific build configuration |
| `run` | Mode-specific run configuration |
| `applications` | Mode-specific application settings |
| `endpoint_tags` | Filter endpoints by tag |

### Orthogonality with `--env`

`--mode` and `--env` are **independent, orthogonal axes**:
- `--mode` = infrastructure topology (native, docker, hybrid)
- `--env` = environment configuration (dev, ci, staging)

Combine freely: `dva up -M backend -E ci`. Never treat these as the same axis or merge them into a single flag.

## Environments (`--env` / `-E`)

Environments define **environment variable presets**. Activate with `dva up -E <name>`.

```yaml
environments:
  ci:
    description: "CI environment"
    environment:
      CI: "true"
      LOG_LEVEL: warn
  staging:
    description: "Staging-like config"
    environment:
      DATABASE_URL: "postgres://localhost:5432/myapp_staging"
```

Environment variables are merged in ascending precedence order (later overrides earlier).
**OS environment variables outrank every layer, including `--var`** — if a key is set in the
OS environment, nothing in `dva.yml` can override it.

Which layers apply depends on the command path:

**`dva up` / `dva stack up`** — the `-M` / `-E` flag path:

```text
environment: < env_file: < environment preset (-E) < mode (-M) < OS environment
```

**`dva run <command>`** — the interaction path (`dva run` has no `-M` / `-E`):

```text
environment: < env_file: < interaction command-level environment: < OS environment
```

**`dva up <plan>`** — the plan path, which resolves the separate `vars:` system:

```text
env_file < global vars < environment vars < site vars < plan vars < --var < OS environment
```

Here `environment vars` means `environments.<name>.environment`, which is distinct from the
top-level `environment:` block. `--var` is accepted only on the plan path.

OS precedence comes from `MergeVars` in `internal/config/environment.go`, which applies the OS
value for a key whenever one is set and only otherwise takes the configured value. A stray
exported shell variable will silently win over `--var` with no warning.

## Tags (`--tags` / `-T`)

Filter services by tag for selective operations. Tags are defined on stack service entries.

```yaml
stack:
  compose:
    default_runner: compose
    runners:
      compose:
        services:
          api:
            tags: [app, backend]
          postgres:
            tags: [db, backend]
          redis:
            tags: [cache]
          frontend:
            tags: [app, ui]
```

Usage:

```bash
dva up -T app              # start api + frontend
dva up -T backend          # start api + postgres
dva up --exclude-tags db   # start everything except postgres
```

Aliases: `--tag` = `--tags`, `--exclude-tag` = `--exclude-tags`.
Available on: `up`, `down`, `stop`, `restart`.

## Subprojects

Monorepo support for projects with multiple `dva.yml` files.

```yaml
subprojects:
  api:
    path: ./services/api
    exclude_tags: [heavy]
  web:
    path: ./services/web
```

Execute subproject commands with namespace syntax:

```bash
dva api:test           # run test in api subproject
dva api:build          # run build in api subproject
dva run --project api test  # explicit form
```

Each subproject references its own `dva.yml` at the specified `path`. Commands are filtered by `exclude_tags` if set.

## Configuration

### `dva.yml` Structure

Top-level sections:

| Section | Description |
|---------|-------------|
| `version` | Minimum DVA version required |
| `stack` | Infrastructure orchestration pipeline (plugin-based) |
| `interaction` | User-facing command definitions |
| `environment` | Global environment variables |
| `env_file` | External env file reference |
| `provision` | Provisioning profiles and steps |
| `plans` | Current named execution plans |
| `modes` | Legacy operational modes (`--mode` flag) |
| `environments` | Environment presets (`--env` flag) |
| `health_checks` | Non-compose service health checks |
| `endpoints` | User-facing URL definitions |
| `checks` | Environment prerequisite checks (`dva doctor`) |
| `subprojects` | Monorepo subproject references |
| `infra` | Shared infrastructure services (git-based) |
| `modules` | `.sb/dva/*.yml` module file patterns |
| `ssh` | SSH agent configuration |
| `devcontainer` | Dev container integration (experimental) |
| `applications` | Legacy long-running application processes |

### Configuration Loading Order

1. `DVA_FILE` environment variable (if set)
2. Walk up from current directory to root, find first `dva.yml`
3. Merge `.sb/dva/*.yml` module files
4. Apply `dva.override.yml` overrides (local, typically in `.gitignore`)

### Modules

Split large configurations into modular files:

```yaml
# dva.yml
modules:
  - ".sb/dva/*.yml"
```

Module files in `.sb/dva/` are merged into the base configuration. This keeps `dva.yml` lean while allowing team-specific or feature-specific overrides.

### Override File

`dva.override.yml` provides local overrides (developer-specific, not committed):

- Merged last, highest precedence
- Typically added to `.gitignore`
- Use for personal port mappings, environment tweaks, etc.

## Stack Pipeline

The `stack:` section defines reusable execution declarations. Compose must be declared through `runners.compose`.

```yaml
stack:
  infra:
    default_runner: compose
    runners:
      compose:
        files: [docker-compose.yml]
        project_name: myapp
        services:
          api:
            tags: [app]
            related: [worker]
  kubectl:                     # kubectl plugin auto-inferred
    order: 20
    namespace: myapp-dev
    context: my-cluster
  staging-compose:
    default_runner: compose
    runners:
      compose:
        files: [docker-compose.staging.yml]
```

### Plugin Type Resolution

Priority order:
1. `runners.<name>` declarations for plan-based execution
2. Flat format + explicit `plugin:` key for non-compose legacy plugins
3. Entry name matches known plugin name → auto-inferred for non-compose legacy plugins

### Supported Plugins

| Tier | Plugins |
|------|---------|
| Core | `compose`, `kubectl`, `helm`, `process`, `script`, `docker` |
| Extended | `kustomize`, `tilt`, `skaffold`, `podman-compose`, `vagrant` |
| Niche | `sam`, `serverless`, `multipass` |

## Runners

DVA routes interaction commands to the appropriate runner based on command definition:

| Runner | Trigger | Example |
|--------|---------|---------|
| DockerCompose | `service:` key present | `service: app` → `docker compose exec app <cmd>` |
| Kubectl | `pod:` key present | `pod: api-pod` → `kubectl exec api-pod -- <cmd>` |
| Local | Neither `service:` nor `pod:` | Direct shell execution |

```yaml
interaction:
  shell:
    service: app           # → DockerCompose runner
    command: /bin/bash
  test:
    command: make test     # → Local runner (no service/pod)
  k8s-logs:
    pod: api-pod           # → Kubectl runner
    command: tail -f /var/log/app.log
```

### Compose Method Options

When using DockerCompose runner, control execution method:

```yaml
interaction:
  shell:
    service: app
    command: /bin/bash
    compose:
      method: exec         # exec (default), run, up
```

## Lifecycle Hooks

The 7 hookable lifecycle commands (`up`, `down`, `stop`, `restart`, `build`, `clean`, `logs`) support hooks defined in the `interaction:` section:

```yaml
interaction:
  build:
    before:
      - step: "Generate code"
        command: "make generate"
    replace:
      - step: "Custom build"
        command: "make build-all"
    after:
      - step: "Verify build"
        command: "make check"
```

| Hook | Timing | Behavior |
|------|--------|----------|
| `before:` | Before the lifecycle command | Additional setup steps |
| `after:` | After the lifecycle command | Verification or cleanup |
| `replace:` | Instead of the lifecycle command | Completely override default behavior |

Each hook step has `step:` (description) and `command:` (shell command) fields.

## Health Checks

Define health checks for non-compose services (local processes, external services):

```yaml
health_checks:
  local-api:
    type: http              # http, tcp, command
    url: http://localhost:3000/health
    start: "npm run dev"
    start_hint: "Run 'npm run dev' in another terminal"
    timeout: 2              # health check timeout (seconds)
    ready_timeout: 30       # startup wait timeout (seconds)
  database:
    type: tcp
    address: localhost:5432
  custom-check:
    type: command
    command: "curl -sf http://localhost:8080/ready"
```

Health checks with `start:` field auto-start the service during `dva up`.

- `start` (optional): DVA auto-start command
- `start_hint` (optional): human-readable text shown when service is not ready
- `start_hint` is only needed when it differs from `start` (e.g., friendlier wording)
- Having both with identical values is redundant — use `start` only in that case
- Neither field is mandatory

## Applications

Long-running application processes managed separately from stack services:

```yaml
applications:
  api:
    description: "API server"
    tags: [backend]
    run:
      native: "go run ./cmd/api"
      docker: "docker compose exec app go run ./cmd/api"
    build:
      native: "go build -o ./build/api ./cmd/api"
    dev:
      native: "air -c .air.toml"
    health:
      type: http
      url: http://localhost:8080/health
    depends_on: [postgres]
    environment:
      PORT: "8080"
    dir: "."
```

Manage applications with `dva app`; start the workspace with `dva up`, then start applications with `dva app up`.

Applications support native and docker execution paths, selected by the current mode.

## Special Variables

Available in `dva.yml` command definitions:

| Variable | Description |
|----------|-------------|
| `DVA_OS` | Current OS (`linux`, `darwin`, `windows`) |
| `DVA_WORK_DIR_REL_PATH` | Working directory relative path |
| `DVA_CURRENT_USER` | Current username |
| `DVA_CURRENT_UID` | Current user UID (numeric) |

## Provision Profiles

One-time setup scripts organized into named profiles:

```yaml
provision:
  default: setup
  profiles:
    setup:
      description: "Initial project setup"
      steps:
        - step: "Install dependencies"
          command: "npm install"
        - step: "Run migrations"
          command: "dva compose exec app rails db:migrate"
          parallel: false
    reset:
      description: "Reset development data"
      steps:
        - step: "Drop and recreate"
          command: "dva compose exec app rails db:reset"
```

Steps with `parallel: true` execute concurrently within their batch.

## Troubleshooting

| Issue | Resolution |
|-------|------------|
| Environment not set up | `dva doctor --fix` |
| Compose project name mismatch | `dva config validate --fix` |
| Legacy config format | `dva config validate` |
| Unknown available commands | `dva ls` or `dva manifest -f json` |
| Service won't start | `dva up --force` (skip health checks) |
| Configuration inspection | `dva config show -f yaml` |
| Debug execution flow | `dva --debug <command>` |
