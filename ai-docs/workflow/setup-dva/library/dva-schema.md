# DVA Configuration Schema Reference

> Canonical schema: `internal/config/schema.json` — always validate against it.

## dva.yml Structure

```yaml
version: "0.1.0"

environment:                    # Global environment variables
  VAR_NAME: value

compose:
  files:
    - compose.yml               # Primary compose file
    # - compose.tools.yml       # Optional dev tools overlay
  project_name: myapp           # Optional COMPOSE_PROJECT_NAME override

interaction:
  {name}:
    description: "{human-readable description}"
    service: {compose-service-name}
    command: {shell command to execute}
    subcommands:                # Optional nested commands
      {sub-name}:
        command: {sub command}

provision:                      # Setup automation
  {profile-name}:
    - command 1                 # String form
    - step: Step name           # Step form (v0.1.0+)
      run: command
    - step: Multi-command step
      run:
        - command 1
        - command 2

modules:                        # Module imports (.dva/*.yml)
  - module-name

kubectl:                        # Kubernetes config (optional)
  namespace: myapp-dev
```

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
- `examples/MIGRATE.md` — Migration guide from hip/legacy configs
- `internal/config/schema.json` — Canonical JSON schema
