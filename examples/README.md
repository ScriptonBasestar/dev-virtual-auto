# DVA Configuration Examples

This directory contains comprehensive examples of DVA configurations for various use cases and project types. Each example demonstrates different features and best practices.

## Quick Start

If you're new to DVA, start with [`basic.yml`](basic.yml) - it contains the essential commands you'll need for a simple Rails project.

## Examples by Use Case

### 🎯 Getting Started

- **[basic.yml](basic.yml)** - Simple Rails project configuration
  - Perfect for beginners
  - Essential commands: shell, rails, bundle, rake
  - Single provision profile
  - Use when: Starting a new Rails project or learning DVA

### 🚀 Full-Stack Applications

- **[full-stack.yml](full-stack.yml)** - Comprehensive Rails + Node.js configuration
  - Rails backend + React/Vue frontend
  - Multiple Docker Compose files
  - Complex subcommand structures
  - Advanced provision profiles (default, reset, seed, deploy)
  - Kubernetes and infra management
  - Use when: Building production-ready full-stack applications

### ☸️ Kubernetes Environments

- **[kubernetes.yml](kubernetes.yml)** - Kubernetes-focused configuration
  - kubectl runner examples
  - Pod targeting
  - Namespace configuration
  - Port forwarding and log streaming
  - Deployment and rollback provisions
  - Use when: Developing against Kubernetes clusters

### 🟢 Node.js Projects

- **[nodejs.yml](nodejs.yml)** - Node.js/Express application
  - npm and yarn command examples
  - MongoDB and Redis integration
  - Development and production modes
  - TypeScript support
  - Use when: Building Node.js applications

### 🤖 LLM/AI Integration

- **[llm-integration.yml](llm-integration.yml)** - LLM-friendly features and patterns
  - Command discovery with `dva manifest`
  - Execution planning with `--explain`
  - Structured output (JSON/YAML)
  - Token efficiency patterns
  - Programmatic usage examples
  - CI/CD integration patterns
  - Use when: Integrating with AI assistants or building automation

## Examples by Feature

### 📦 Provision Profiles

- **[provision-profiles.yml](provision-profiles.yml)** - Comprehensive provision examples
  - Multiple named profiles: default, reset, seed, test, ci, deploy
  - Each profile serves a different purpose
  - Demonstrates best practices for project automation
  - Use when: You need different setup scenarios (development, testing, CI/CD)

- **[provision-step-syntax.yml](provision-step-syntax.yml)** - New step/run/note syntax (v0.1.0+)
  - Cleaner provision scripts without repetitive echo commands
  - Automatic step numbering with progress indicators (📦 [1/4])
  - Multi-command steps with `run:` arrays
  - Informational notes with `note:` property
  - No quote escaping needed
  - Use when: You want readable provision scripts with structured output

### 🧩 Module System

- **[modules/](modules/)** - Modular configuration example
  - **[main.yml](modules/main.yml)** - Main configuration importing modules
  - **[.dva/sast.yml](modules/.dva/sast.yml)** - Static analysis and security tools module
  - **[.dva/testing.yml](modules/.dva/testing.yml)** - Testing framework module
  - Use when: Large projects with shared configurations across teams

### 🔄 Configuration Migration

- **[MIGRATE.md](MIGRATE.md)** - DVA configuration upgrade guide
  - Step-by-step migration walkthrough
  - Common migration scenarios with before/after examples
  - AI-assisted migration with Claude/ChatGPT
  - Troubleshooting deprecated features
  - Best practices for safe upgrades
  - Use when: Upgrading DVA versions or adopting new features

### 🔐 Environment Variables

- **[env-file-basic.yml](env-file-basic.yml)** - Basic .env file usage
  - Separate secrets from configuration
  - Simple `env_file: .env` configuration
  - Git-ignored secrets management
  - Use when: Managing secrets and local configuration

- **[env-file-priority.yml](env-file-priority.yml)** - Priority control
  - Multiple .env files with layering
  - before_environment vs after_environment priority
  - Per-file required flags
  - Command-specific env_file
  - Use when: Complex multi-environment setups

- **[env-file-multi-env.yml](env-file-multi-env.yml)** - Multi-environment support
  - Environment-specific .env files (.env.development, .env.staging, .env.production)
  - Variable interpolation ($VAR, ${VAR})
  - Dynamic file selection based on ENV variable
  - Use when: Supporting development, staging, production environments

## Using These Examples

### 1. Copy an Example

```bash
# Copy an example to your project root
cp examples/basic.yml ./dva.yml

# Or for modules
cp -r examples/modules/main.yml ./dva.yml
cp -r examples/modules/.dva ./
```

### 2. Customize for Your Project

Edit the copied file to match your:
- Service names in docker-compose.yml
- Database credentials
- Port numbers
- Commands and scripts

### 3. Validate Your Configuration

```bash
# Validate your dva.yml
dva validate

# Or validate a specific example
dva validate -c examples/basic.yml
```

## Configuration File Structure

All examples follow this general structure:

```yaml
version: '0.1.0'              # Minimum required dva version

environment:                  # Environment variables
  VAR_NAME: value

compose:                      # Docker Compose configuration
  files:
    - docker-compose.yml
  project_name: myapp

interaction:                  # Interactive commands
  command-name:
    description: What this does
    service: container-name
    command: actual command
    subcommands:              # Optional nested commands
      sub-name:
        command: sub command

provision:                    # Setup automation
  profile-name:
    - command 1
    - command 2

kubectl:                      # Kubernetes config (optional)
  namespace: myapp-dev

modules:                      # Module imports (optional)
  - module-name

infra:                        # Infrastructure services (optional)
  service-name:
    git: repo-url
```

## Common Patterns

### Environment Variables

```yaml
environment:
  # Static values
  RAILS_ENV: development

  # With defaults
  PORT: ${PORT:-3000}

  # References
  DATABASE_URL: postgres://user:password@db:5432/myapp_development
```

### Subcommands

```yaml
interaction:
  rails:
    command: bundle exec rails
    subcommands:
      console:
        command: console
      db:
        subcommands:
          migrate:
            command: db:migrate
```

### Provision Profiles

**Note**: As of v0.1.0, `dva provision` automatically starts containers if needed.

```yaml
provision:
  default:              # 'dva provision' or 'dva provision default'
    - dva bundle install
    - dva rails db:create
    - dva rails db:migrate

  reset:                # 'dva provision reset'
    - dva compose down --volumes
    - dva bundle install
```

### Step Syntax (v0.1.0+)

New structured syntax for cleaner provision scripts:

```yaml
provision:
  default:
    # Named step with single command
    - step: Installing Ruby gems
      run: dva bundle install

    # Step with multiple commands
    - step: Setting up database
      run:
        - dva rails db:create
        - dva rails db:migrate

    # Step with note (no commands)
    - step: Setup complete
      note: |
        Run 'dva rails server' to start
        Open http://localhost:3000
```

**Output**:
```
📦 [1/3] Installing Ruby gems
   → dva bundle install

📦 [2/3] Setting up database
   → dva rails db:create
   → dva rails db:migrate

📦 [3/3] Setup complete
   ℹ️  Run 'dva rails server' to start
   ℹ️  Open http://localhost:3000
```

See [provision-step-syntax.yml](provision-step-syntax.yml) for complete examples.

## Validation

All examples in this directory are validated against the schema. You can validate any example:

```bash
# Validate a specific example
DVA_FILE=examples/basic.yml dva validate

# Validate all examples
for file in examples/*.yml; do
  echo "Validating $file..."
  DVA_FILE=$file dva validate
done
```

## LLM-Friendly Features (v0.1.0+)

DVA includes several features designed for AI/LLM integration:

### Command Discovery

```bash
# Get complete command registry
dva manifest                # JSON output (default)
dva manifest -f yaml        # YAML output

# List commands with metadata
dva ls --format json        # Structured command list
dva ls --detailed           # Show runner types and targets
```

### Execution Planning

```bash
# See what will run before executing
dva shell --explain         # Show execution plan
dva rake db:migrate -e      # Short form with -e
```

### Token Efficiency

The new documentation system reduces LLM token usage by 65-70%:

- **CONTEXT_MAP.md** - Navigate files by task type
- **CLAUDE.md** - Compressed architecture guide
- **AGENTS.md** - Component-specific guides
- **Code headers** - Quick file understanding

See [llm-integration.yml](llm-integration.yml) for comprehensive examples.

## Tips and Best Practices

1. **Start Simple**: Begin with `basic.yml` and add features as needed
2. **Use Provision Profiles**: Create profiles for different scenarios (development, testing, deployment)
3. **Separate Concerns**: Use `dva up` for containers, `dva provision` for initialization (v0.1.0+)
4. **Document Your Commands**: Use `description` fields to help team members and LLMs
5. **Leverage Subcommands**: Organize related commands hierarchically
6. **Version Control**: Commit your `dva.yml` to share with your team
7. **Modules for Scale**: Use modules for large projects with many commands
8. **Validate Often**: Run `dva validate` after making changes
9. **LLM Integration**: Use `dva manifest` for programmatic command discovery
10. **Explain Before Execute**: Use `--explain` to validate command execution plans

## Contributing

Found a bug in an example or have a suggestion for a new one? Please open an issue or pull request!

## Learn More

- [DVA Documentation](https://github.com/ScriptonBasestar/dev-virtual-auto)
- [Schema Reference](../schema.json)
- [Roadmap](../docs/ROADMAP.md)

---

**Version**: 0.1.0
**Last Updated**: March 2026
