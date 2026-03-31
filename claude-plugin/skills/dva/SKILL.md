---
name: dva
description: >-
  This skill should be used when the user asks to "build the project",
  "run tests", "start services", "stop containers", "check logs",
  "use kubectl", or manage dev infrastructure.
  Enforces DVA CLI — never use raw docker/compose/kubectl.
globs: "*"
allowed-tools: [Bash, Read, Grep, Glob]
version: 0.1.0
---

# DVA (Dev Virtual Auto) CLI

DVA is a development workspace orchestrator that unifies Docker Compose, Kubernetes, Helm, and local processes through a single `dva.yml` configuration. All CLI tasks for building, testing, running, and managing development infrastructure go through DVA.

## Rules (Mandatory)

- **NEVER bypass DVA.** Do not invoke `docker`, `docker compose`, `docker-compose`, `kubectl`, `helm`, or raw language toolchains (`npm test`, `go build`, `make`) directly. Use `dva <command>` instead.
- **NEVER parse `dva.yml` manually.** Use `dva manifest -f json` for structured command discovery or `dva config show` for merged configuration output.
- **NEVER guess available commands.** Run `dva ls` or `dva manifest -f json` to discover project-specific commands before execution.
- **Use `--dry-run` before destructive operations.** Preview execution plans with `dva <command> --dry-run`.
- **Use `dva config improve` for configuration changes**, not manual editing of `dva.yml`.
- **Use `dva doctor` for environment issues.** Run `dva doctor --fix` to auto-resolve fixable problems.

## Project Context

!`dva show 2>/dev/null || echo "DVA not configured in this project"`

## Available Commands

!`dva ls -f json 2>/dev/null || echo "No dva.yml found"`

## Workflow

### Discover Commands

Before executing any build, test, or run task, discover available commands:

```bash
dva manifest -f json    # LLM-optimized structured output
dva ls                  # human-readable command list
dva ls -f json          # JSON command list with runner info
dva show                # project overview (modes, envs, commands)
```

Read the `dynamic_commands` section from manifest output to identify project-specific commands and their runners (DockerCompose, Kubectl, or Local).

### Execute Commands

Run project-defined interaction commands directly. The `run` prefix is optional:

```bash
dva test               # = dva run test
dva build              # = dva run build
dva lint               # = dva run lint
dva <command>          # any interaction command from dva.yml
```

For subproject commands in monorepos, use namespace syntax:

```bash
dva api:test           # run test in api subproject
dva run --project api test  # equivalent explicit form
```

Pass additional flags as needed:

```bash
dva run -p 8080:80 web     # port publishing
dva run --dry-run test     # preview execution plan without running
```

### Lifecycle Management

Lifecycle commands operate on the `stack:` pipeline, executing plugins in `order` sequence:

```bash
dva up                 # start all services (stack order)
dva up postgres redis  # start specific services only
dva up -M backend      # apply mode filter
dva up -E ci           # apply environment preset
dva up -T app,api      # tag-based service selection
dva down               # stop and remove services
dva stop               # stop without removing
dva restart            # stop + start
dva logs <service>     # view service logs
dva build              # build service images
dva clean              # remove containers, networks
dva clean -v           # also remove volumes (data loss warning)
```

The `--mode/-M` and `--env/-E` flags are **orthogonal axes**:
- `--mode` controls **infrastructure strategy** (native, docker, hybrid)
- `--env` controls **environment variable presets** (dev, ci, staging)

These are independent concerns. Combine freely: `dva up -M backend -E ci`.

### Diagnostics

```bash
dva doctor             # check environment prerequisites
dva doctor --fix       # auto-fix detected issues
dva status             # workspace and service status
dva config show        # final merged configuration (JSON)
dva config show -f yaml  # YAML format
dva config validate    # validate dva.yml schema
dva config validate --fix  # auto-fix compose project name
```

### Integration Pass-through

When DVA lacks a direct command for an operation, use pass-through:

```bash
dva compose <args>     # pass-through to docker compose
dva ktl <args>         # pass-through to kubectl
dva infra up           # shared infrastructure management
dva infra down         # tear down shared infra
dva infra update       # update infra definitions
dva ssh up             # SSH agent container
```

## Key Concepts

| Concept | Description |
|---------|-------------|
| `--mode/-M` | Infrastructure strategy axis (native, docker, hybrid). Orthogonal to `--env`. |
| `--env/-E` | Environment variable preset axis (dev, ci, staging). Orthogonal to `--mode`. |
| `--tags/-T` | Service group filtering by tag. `--exclude-tags` to exclude. |
| Runners | DockerCompose (`service:` key), Kubectl (`pod:` key), Local (default). |
| Subprojects | Monorepo support: `dva namespace:command` or `dva run --project name cmd`. |
| Hooks | `before:`, `after:`, `replace:` on lifecycle commands (up, down, build, etc.). |
| Modules | `.sb/dva/*.yml` files merged into base config. Override with `dva.override.yml`. |
| Dynamic routing | Unknown commands route to `dva run <cmd>` automatically. |
| Applications | Long-running processes managed via `dva app` (list, build, run, stop). |
| Provision | One-time setup scripts via `dva provision [profile]`. |

## Global Flags

| Flag | Description |
|------|-------------|
| `--debug` | Enable debug logging |
| `--dry-run` | Show execution plan without running (alias: `--explain`, `-e`) |
| `--json` | JSON output (LLM-optimized) |

## LLM Integration

DVA provides first-class LLM support:

```bash
dva manifest -f json       # structured command manifest for AI
dva config init --ai       # auto-generate dva.yml via Claude Code
dva config improve         # AI-based dva.yml improvement
dva config improve --print # output improvement prompt for manual use
dva --json <command>       # JSON output on any command
```

## Additional Resources

### Reference Files

For detailed command documentation and advanced patterns, consult:
- **`references/commands.md`** - Complete command reference with all flags and options
- **`references/advanced.md`** - Modes, environments, subprojects, configuration management, stack pipeline, and troubleshooting
