# Makefile Integration with DVA

This guide shows how to integrate DVA commands into your project's Makefile for a streamlined development workflow.

## Quick Start

### 1. Install DVA

```bash
# See README.md for installation methods
go install github.com/ScriptonBasestar/dva/cmd/dva@latest
```

### 2. Create dva.yml

Create a `dva.yml` file in your project root. See [makefile-integration.yml](makefile-integration.yml) for a complete example.

```yaml
version: '0.1.0'

compose:
  files:
    - docker-compose.yml

interaction:
  rails:
    description: Run Rails commands
    service: app
    command: bundle exec rails

  rspec:
    description: Run tests
    service: app
    command: bundle exec rspec

provision:
  - dva compose up -d postgres redis
  - dva bundle install
  - dva rails db:setup
```

### 3. Add Makefile Targets

Copy the DVA usage examples from DVA's `Makefile.dev.mk` to your project's Makefile:

```makefile
# DVA Development Environment
.PHONY: dva-up dva-down dva-clean dva-console

dva-up: ## Start DVA Docker environment
	@dva compose up -d

dva-down: ## Stop DVA Docker environment
	@dva compose down

dva-console: ## Open Rails console
	@dva rails console

dva-provision: ## Run provisioning
	@dva provision
```

## Available Make Targets

Once integrated, you can use these commands:

| Command | Description |
|---------|-------------|
| `make dva-up` | Start Docker environment |
| `make dva-down` | Stop Docker environment |
| `make dva-clean` | Clean containers, volumes, networks |
| `make dva-console` | Open Rails console |
| `make dva-rails ARGS="db:migrate"` | Run Rails commands |
| `make dva-test` | Run tests |
| `make dva-logs` | Show container logs |
| `make dva-provision` | Run provisioning scripts |
| `make dva-dev` | Full dev cycle (down, clean, up, provision) |
| `make dva-restart` | Quick restart |

## Example Workflows

### Daily Development

```bash
# Start environment
make dva-up

# Open console
make dva-console

# Run migrations
make dva-rails ARGS="db:migrate"

# Run tests
make dva-test

# View logs
make dva-logs

# Stop environment
make dva-down
```

### Fresh Environment Setup

```bash
# Full clean and provision
make dva-dev
```

### Quick Restart After Changes

```bash
make dva-restart
```

## Advanced Integration

### Custom Targets

You can create custom targets that combine multiple DVA commands:

```makefile
# Custom: Deploy to staging
deploy-staging: dva-test
	@echo "Deploying to staging..."
	@dva rails db:migrate RAILS_ENV=staging
	@dva rails assets:precompile RAILS_ENV=staging
	@echo "✓ Deployed to staging"

# Custom: Run full test suite
test-all: dva-up
	@make dva-test ARGS="--tag ~slow"
	@make dva-test ARGS="--tag slow"
```

### Environment-Specific Targets

```makefile
# Development environment
dev-env:
	@COMPOSE_EXT=development make dva-up

# Test environment
test-env:
	@COMPOSE_EXT=test make dva-up
```

## Troubleshooting

### DVA command not found

```bash
# Check if dva is installed
dva --version

# If not, install it
go install github.com/ScriptonBasestar/dva/cmd/dva@latest
```

### dva.yml not found

Make sure `dva.yml` exists in your project root or current directory. DVA searches up the directory tree for the config file.

### Docker commands fail

Ensure Docker is running:

```bash
docker ps
```

### Rails commands fail

Check that your service is defined in `dva.yml`:

```yaml
interaction:
  rails:
    service: app  # Make sure this service exists in docker-compose.yml
    command: bundle exec rails
```

## Complete Example

See [makefile-integration.yml](makefile-integration.yml) for a complete dva.yml example that works with all Makefile targets.

## References

- [DVA README](../README.md) - Main documentation
- [DVA Examples](README.md) - All configuration examples
- [DVA Installation](../README.md#install) - Installation guide
- [DVA Development Makefile](../Makefile.dev.mk) - Source of example targets
