<!-- v:2026-03-23 -->

<constants>
SELF = ai-docs/workflow/setup-dva/stages/30-configure.md
DVA_ROOT = {DVA project root}
WORKFLOW_ROOT = ai-docs/workflow/setup-dva
SCHEMA_REF = internal/config/schema.json
EXAMPLES_DIR = examples/
TARGET = {resolved from entry.md or auto.md — target project path}
PORT_REGISTRY = {optional — global-port-mappings.yaml path if available}
</constants>

[EXECUTE IMMEDIATELY - NO QUESTIONS]

<role>DVA configuration generator — produce compose.yml + dva.yml from analysis and proposal</role>

<input>
| Source | Description |
|--------|-------------|
| Analysis | `tmp/setup-dva/00-analysis-{project-name}.md` |
| Proposal | `tmp/setup-dva/10-proposal-{project-name}.md` |
| Transform log | `tmp/setup-dva/20-transform-log-{project-name}.md` |
| DVA schema | SCHEMA_REF for validation |
| DVA examples | EXAMPLES_DIR for reference patterns |
| Port registry | PORT_REGISTRY (optional) |
</input>

<objective>
Generate production-ready compose.yml and dva.yml for the target project.
Use DVA's own examples as reference and validate against DVA schema.
</objective>

<steps>
## Phase 1: Load Context

- Read proposal's compose services plan table
- Read DVA schema from SCHEMA_REF for valid configuration fields
- Read recommended DVA example from EXAMPLES_DIR (identified in stage 00)
- Read PORT_REGISTRY for port allocation if available

## Phase 2: Generate compose.yml

### Compose Specification compliance
- No `version:` key (Compose Specification)
- Service naming: `${COMPOSE_PROJECT_NAME:-project}-{service}`
- Env vars: `${VAR:-default}` pattern with fallbacks

### Port mapping
- Use PORT_REGISTRY allocations if available
- Otherwise use project-specific non-conflicting ports
- Avoid common default ports on host: 5432, 6379, 8080, 8000, 3000, 3306, 27017

### Healthchecks (required for core services)
```yaml
# postgres
healthcheck:
  test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-dev}"]
  interval: 10s
  timeout: 5s
  retries: 5

# redis
healthcheck:
  test: ["CMD", "redis-cli", "--raw", "incr", "ping"]
  interval: 10s
  timeout: 3s
  retries: 3
```

### Profiles for optional services
```yaml
profiles: [dev-tools]  # adminer, mailhog, etc.
```

### Override files (if needed)
- `compose.yml` — primary (core services)
- `compose.tools.yml` — dev tools (adminer, pgadmin)
- `compose.monitor.yml` — monitoring (prometheus, grafana)

## Phase 3: Generate .env.example

```bash
COMPOSE_PROJECT_NAME={project-name}
# ─── Database ───
POSTGRES_VERSION=15-alpine
POSTGRES_PORT={allocated}
POSTGRES_USER=dev
POSTGRES_PASSWORD=dev
POSTGRES_DB={project}_dev
```

## Phase 4: Generate dva.yml

Based on DVA schema and detected project type:

```yaml
version: "0.1.0"
compose:
  files:
    - compose.yml
interaction:
  shell:
    description: "Open shell in {primary-service} container"
    service: {primary-service}
    command: /bin/bash
  logs:
    description: "Tail service logs"
    service: {primary-service}
    command: ""
  db:
    description: "Database console"
    service: {db-service}
    command: {db-cli-command}
```

Adapt interactions by project type:
| Project | Interactions |
|---------|-------------|
| Go | shell, test (go test ./...), build (make build) |
| Python | shell, test (uv run pytest), migrate |
| Node.js | shell, test (pnpm test), dev (pnpm dev) |
| Ruby/Rails | shell, console (bundle exec rails console), test (bundle exec rspec) |
| Rust | shell, test (cargo test), build (cargo build) |

## Phase 5: Write Files

Write generated files to target project:
- `$TARGET/compose.yml`
- `$TARGET/compose.tools.yml` (if dev tools needed)
- `$TARGET/.env.example`
- `$TARGET/dva.yml`

If files already exist, create `.new` suffixed versions and note in log.

## Phase 6: Validation

```bash
# Syntax check
docker compose -f $TARGET/compose.yml config --quiet 2>&1
# DVA config validation (if dva binary available)
DVA_FILE=$TARGET/dva.yml dva validate 2>/dev/null
```
</steps>

<constraints>
- Generated dva.yml must conform to DVA schema (SCHEMA_REF)
- Never use common default ports as host ports
- If compose.yml already exists and is functional, create compose.yml.new instead
- dva.yml interactions must reference actual compose services
- Reference DVA examples for idiomatic configuration patterns
</constraints>

<gate>
- [ ] compose.yml generated and passes `docker compose config`
- [ ] .env.example generated with all required vars
- [ ] dva.yml generated and valid against DVA schema
- [ ] No banned ports used as host ports
- [ ] Config log generated
</gate>

<output>
| Artifact | Path |
|----------|------|
| Compose file | `$TARGET/compose.yml` |
| Tools overlay | `$TARGET/compose.tools.yml` (if applicable) |
| Env template | `$TARGET/.env.example` |
| DVA config | `$TARGET/dva.yml` |
| Config log | `tmp/setup-dva/30-config-log-{project-name}.md` |
</output>

<trigger>Load context → generate compose.yml → generate dva.yml → validate syntax.</trigger>
