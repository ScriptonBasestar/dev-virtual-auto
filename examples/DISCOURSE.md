# DVA Configuration for Discourse Plugin Development

This guide explains how to set up DVA for Discourse plugin development.

## Quick Start

### 1. Copy dva.yml to Your Project

```bash
# In your gorisa-plugins or discourse project directory
cp /path/to/dva/examples/discourse-plugin-dev.yml dva.yml
```

Or create it manually (see below).

### 2. Verify docker-compose.yml

Make sure your `docker-compose.yml` has a `discourse` service (or similar):

```yaml
services:
  discourse:
    image: discourse/discourse_dev:latest
    # or build from local
    volumes:
      - .:/src
    depends_on:
      - postgres
      - redis

  postgres:
    image: postgres:14
    environment:
      POSTGRES_USER: discourse
      POSTGRES_DB: discourse_development

  redis:
    image: redis:7
```

**Note**: If your service name is different (e.g., `app`, `web`), update `service: discourse` in dva.yml to match.

### 3. Start Environment

```bash
# First time setup
dva provision

# Or manually
dva compose up -d
dva bundle install
dva rails db:create db:migrate
```

## Available Commands

### Rails Commands
```bash
dva rails console          # Open Rails console
dva rails server           # Start Rails server
dva rails db:migrate       # Run migrations
dva rails db:seed          # Seed database
dva rails db:reset         # Reset database
```

### Bundle Commands
```bash
dva bundle install         # Install gems
dva bundle update          # Update gems
```

### Testing
```bash
dva rspec                  # Run all tests
dva rspec spec/models/     # Run specific tests
dva plugin test            # Run plugin tests
```

### Shell Access
```bash
dva bash                   # Open bash shell
dva shell                  # Same as bash
dva psql                   # PostgreSQL console
dva redis-cli              # Redis CLI
```

### Discourse Specific
```bash
dva discourse admin:create     # Create admin user
dva discourse precompile       # Precompile assets
dva plugin install            # Install plugin dependencies
```

## Directory Structure

### Option A: Discourse with Plugins
```
gorisa-web-discourse/
├── dva.yml
├── docker-compose.yml
├── discourse/              # Discourse core
│   └── ...
└── plugins/
    └── gorisa-plugins/     # Your plugin
        └── ...
```

### Option B: Plugin Development Only
```
gorisa-plugins/
├── dva.yml
├── docker-compose.yml      # Minimal compose with Discourse
└── plugin.rb
```

## Example dva.yml

Minimal version for quick setup:

```yaml
version: '0.1.0'

lifecycle:
  - name: compose
    plugin: compose
    order: 10
    compose:
      files:
        - docker-compose.yml

interaction:
  rails:
    description: Run Rails commands
    service: discourse  # Change if your service name is different
    command: bundle exec rails

  bundle:
    description: Run Bundler
    service: discourse
    command: bundle

  bash:
    description: Open shell
    service: discourse
    command: bash

provision:
  - dva compose up -d postgres redis
  - dva bundle install
  - dva rails db:create db:migrate
```

## Common Workflows

### Daily Development

```bash
# Start environment
dva compose up -d

# Open console to test
dva rails console

# Run migrations after changes
dva rails db:migrate

# Restart Rails (if needed)
dva compose restart discourse

# View logs
dva compose logs -f discourse
```

### Plugin Development

```bash
# Open shell in Discourse container
dva bash

# Inside container, test your plugin
cd plugins/gorisa-plugins
bundle exec rspec

# Or use dva command
dva plugin test
```

### Database Management

```bash
# Create fresh database
dva rails db:create

# Run migrations
dva rails db:migrate

# Seed data
dva rails db:seed

# Reset everything
dva rails db:reset
```

## Troubleshooting

### "Could not find dva.yml"

Make sure dva.yml is in your project root:

```bash
# Check current directory
pwd

# Create dva.yml
cp examples/discourse-plugin-dev.yml dva.yml
```

### "Service 'discourse' not found"

Your docker-compose.yml might use a different service name. Check:

```bash
grep 'services:' -A 20 docker-compose.yml
```

Then update `service: discourse` in dva.yml to match (e.g., `service: app`).

### "Connection refused to postgres"

Wait a bit longer for services to start:

```bash
dva compose up -d postgres redis
sleep 10
dva rails db:create
```

### Gems not installing

```bash
# Clear bundle cache
dva compose down -v
dva compose up -d
dva bundle install
```

## Advanced Configuration

### Using Different Compose Files

```yaml
lifecycle:
  - name: compose
    plugin: compose
    order: 10
    compose:
      files:
        - docker-compose.yml
        - docker-compose.development.yml
      project_name: gorisa-discourse
```

### Environment Variables

```yaml
environment:
  RAILS_ENV: development
  DISCOURSE_HOSTNAME: localhost:3000
  DISCOURSE_DEVELOPER_EMAILS: admin@example.com
```

### Custom Plugin Commands

```yaml
interaction:
  plugin:
    description: Plugin commands
    service: discourse
    command: bundle exec rails
    subcommands:
      lint:
        description: Lint plugin code
        command: bundle exec rubocop plugins/gorisa-plugins

      test:fast:
        description: Fast plugin tests
        environment:
          RAILS_ENV: test
        command: bundle exec rspec plugins/gorisa-plugins --tag ~slow
```

## References

- [Discourse Development Guide](https://meta.discourse.org/t/beginners-guide-to-install-discourse-for-development-using-docker/102009)
- [DVA Documentation](../README.md)
- [DVA Examples](README.md)
- [discourse-plugin-dev.yml](discourse-plugin-dev.yml) - Full example
