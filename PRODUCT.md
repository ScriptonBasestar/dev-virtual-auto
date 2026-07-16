# DVA (Dev Virtual Auto)

> One declarative interface for local development environments

## What is DVA?

DVA is a local-first development environment orchestrator. It combines reusable stack
declarations with named execution plans so developers can operate containers, clusters, and local
processes through one project-owned interface.

## Why?

Development environments become difficult to reproduce when commands are scattered across shell
history, Compose files, cluster tooling, Make targets, and personal setup notes.

- **Fragmented execution**: Each backend exposes different commands and lifecycle semantics.
- **Implicit knowledge**: Developers must remember file combinations, service subsets, and order.
- **Configuration drift**: Local, staging, remote, and team setups duplicate similar definitions.
- **Agent guesswork**: Coding agents cannot safely operate workflows they cannot discover.

DVA makes execution intent explicit, resolves it into a validated plan, and delegates each action
to the tool that owns the underlying resource.

## Who is it for?

| User Type | Core Need |
|-----------|-----------|
| **Application developers** | Start and inspect a complete project without memorizing backend commands |
| **Devbox and monorepo teams** | Share root orchestration while preserving subproject ownership |
| **Infrastructure-heavy projects** | Coordinate local processes, containers, and cluster tools |
| **AI coding agent users** | Discover valid project commands and effective configuration before execution |

## Key Capabilities

- **Declarative stack catalog**: Define reusable logical execution targets and their runners
- **Named execution plans**: Select targets, runner, service subset, order, and dependencies
- **Environment composition**: Apply environment, site, variables, modules, and local overrides
- **Project commands**: Expose one-shot interactions and one-time provisioning procedures
- **Validation and diagnostics**: Validate schema and semantics, inspect effective configuration,
  and diagnose prerequisites
- **Machine-readable discovery**: Publish commands and resolved configuration for coding agents
- **Subproject composition**: Import active child projects without taking ownership of their native commands

## Product Boundaries

DVA coordinates existing developer tools; it does not replace them.

- Compose, Kubernetes, Helm, and process tools retain ownership of their native resources.
- DVA does not hide destructive lifecycle operations behind implicit automation.
- Provisioning is for one-time preparation; repeatable startup belongs to execution plans.
- Project secrets remain outside shared declarative configuration.
- AI-assisted improvement is optional and must preserve the same validation and ownership rules.

## Current Status

| Item | Status |
|------|--------|
| **Maturity** | Active development — named plans are the preferred model; legacy lifecycle surfaces are being migrated |
| **Primary interface** | `dva` CLI with project-owned `dva.yml` |
| **Execution model** | Reusable declarations resolved into immutable named plans |
| **Agent integration** | Machine-readable manifest plus optional Claude and agent-mesh workflows |

## Learn More

- [README.md](README.md) — Installation and quick start
- [USAGE.md](USAGE.md) — Command and configuration reference
- [SOUL.md](SOUL.md) — Durable design principles
- [ARCHITECTURE.md](ARCHITECTURE.md) — System boundaries and data flow
- [Configuration Merge Semantics](docs/30-config-merge-semantics.md) — Composition rules
- [Execution Plan Resolution](docs/31-execution-plan-resolution.md) — Plan resolution contract
- [Declarative Stack and Plans](docs/40-declarative-stack-and-plans.md) — Execution model design
