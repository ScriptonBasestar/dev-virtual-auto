---
name: dva
description: >-
  This skill should be used when the user asks to "build the project",
  "run tests", "start services", "stop containers", "check logs",
  "use kubectl", or manage dev infrastructure.
  Enforces DVA CLI discovery and safe plan-based execution; use raw tools only
  for configuration validation or when DVA has no equivalent.
allowed-tools: [Bash, Read, Grep, Glob]
---

# DVA (Dev Virtual Auto) CLI

DVA is a development workspace orchestrator that unifies Docker Compose, Kubernetes, Helm, and local processes through a single `dva.yml` configuration. All CLI tasks for building, testing, running, and managing development infrastructure go through DVA.

## Rules (Mandatory)

- **Use DVA for declared workflows.** Raw tools are allowed for read-only
  validation (`docker compose ... config`) or when no DVA command exists; state why.
- **NEVER parse `dva.yml` manually.** Use `dva manifest -f json` for structured command discovery or `dva config show` for merged configuration output.
- **NEVER guess available commands.** Run `dva ls` or `dva manifest -f json` to discover project-specific commands before execution.
- **Do not trust lifecycle `--dry-run` blindly.** Prove the installed version is
  non-mutating in a disposable fixture before using it with up/down/provision.
- **Prefer `am run dva-improve` for broad configuration rewrites.** For targeted config authoring or migration, start from the bundled templates and validate with `dva config validate`.
- **Use `dva doctor` read-only first.** `--fix` requires explicit authority and
  review of the proposed mutation.

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
dva config show -f yaml # merged configuration
```

Read the `dynamic_commands` section from manifest output to identify project-specific commands and their runners (DockerCompose, Kubectl, or Local).

### Author or Migrate Configuration

For repeatable `dva.yml` work, load **`references/patterns.md`** first, then choose a template:

```text
assets/templates/root-devbox-plan.yml       # parent devbox: compose-owned full stack + native dev runners
assets/templates/subproject-local.yml       # child project: local interactions/provision only
assets/templates/migrate-modes-to-plans.yml # old modes/applications -> stack runners + plans
```

Use the declarative structure by default:

```text
stack = reusable declarations
plans = executable names
environments = dev/stg/prd vars
sites = local/office/remote/cloud host differences
interaction = one-shot convenience commands
provision = one-time setup procedures
```

For new/rewrite configuration, use named plans. Treat `modes` and
`applications` as migration-only legacy sections. After editing, run:

```bash
dva config validate
dva config validate --strict
dva ls
dva manifest -f json
```

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

Lifecycle commands execute a named plan whose entries select stack runners,
service subsets, order, and dependencies:

```bash
dva up local-infra      # start the named plan
dva up local-dev        # compose infra + native app runners
dva down local-dev      # reverse-order teardown for the same plan
dva stop local-dev      # stop without removing for the same plan
dva restart local-dev  # stop + start the named plan
dva logs <service>     # view service logs
dva build              # build service images
dva clean              # remove containers, networks
dva clean -v           # also remove volumes (data loss warning)
```

Plans select an `environment` and `site`. Do not create a separate `docker`
runner for a service already owned by a Compose entry: `docker` means a
standalone `docker run` target, while `compose` owns Compose services.

### Diagnostics

```bash
dva doctor             # check environment prerequisites
dva doctor --fix       # mutation; explicit authority required
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
| Plans | Named executable combinations of stack entries and runners. |
| Environments | dev/stg/prd variable differences selected by plans. |
| Sites | local/office/remote/cloud host differences selected by plans. |
| Runners | DockerCompose (`service:` key), Kubectl (`pod:` key), Local (default). |
| Subprojects | Monorepo support: `dva namespace:command` or `dva run --project name cmd`. |
| Hooks | `before:`, `after:`, `replace:` on lifecycle commands (up, down, build, etc.). |
| Modules | `.sb/dva/*.yml` files merged into base config. Override with `dva.override.yml`. |
| Dynamic routing | Unknown commands route to `dva run <cmd>` automatically. |
| Applications | Legacy long-running process model; migrate to stack runners + plans. |
| Provision | One-time setup only; lifecycle startup belongs to plans. |

## Global Flags

| Flag | Description |
|------|-------------|
| `--debug` | Enable debug logging |
| `--dry-run` | Preview flag; verify non-mutation for the installed version before lifecycle use. |
| `--json` | JSON output (LLM-optimized) |

## LLM Integration

DVA provides first-class LLM support:

```bash
am run dva-discover        # project analysis and options discovery
am run dva-improve         # AI-based dva.yml generation/improvement
am run dva-diagnose        # AI-based error diagnosis and configuration fix
dva manifest -f json       # structured command manifest for AI
dva config docs            # regenerate AI agent configuration (CLAUDE.md/AGENTS.md)
dva --json <command>       # JSON output on any command
```

## Additional Resources

### Reference Files

For detailed command documentation and advanced patterns, consult:
- **`references/commands.md`** - Complete command reference with all flags and options
- **`references/advanced.md`** - Plans, environments, sites, legacy migration, subprojects, and stack runners
- **`references/patterns.md`** - Standard config authoring workflow, migration checklist, naming conventions, and validation gates

### Templates

Reusable YAML starters live under **`assets/templates/`**:
- **`root-devbox-plan.yml`** - Standard root devbox with compose infrastructure, app runners, environments, sites, plans, interactions, provision, checks, endpoints
- **`subproject-local.yml`** - Lightweight subproject config for local commands and setup
- **`migrate-modes-to-plans.yml`** - Migration skeleton for replacing `modes`/`applications` with `plans` and multi-runner `stack` entries
