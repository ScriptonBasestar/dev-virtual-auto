---
description: Always use DVA (Dev Virtual Auto) for CLI tasks, building, running tests, and managing dev environments. Never use raw docker/compose/kubectl directly.
globs: *
---

# Use DVA (Dev Virtual Auto)

You **MUST** use the `dva` CLI instead of raw `docker`, `docker compose`, `kubectl`, or language tools directly. DVA ensures execution matches the project's `dva.yml` configuration with correct environments, overrides, and networking.

## Workflow

### 1. Discover Available Commands

Before guessing how to build/test/run, always check what commands exist:

```bash
dva manifest -f json
```

Read the `dynamic_commands` section from the output to find project-specific commands.

For a human-readable overview of modes, environments, and commands:

```bash
dva show
```

### 2. Execute Commands

Run project-defined commands directly:

```bash
dva test           # run tests
dva build          # build the project
dva run <command>  # explicit form
```

### 3. Lifecycle Management

```bash
dva up             # start services
dva down           # stop and remove services
dva stop           # stop without removing
dva restart        # stop + start
dva logs <service> # view logs
dva clean          # remove containers, networks, volumes
```

### 4. Infrastructure & Integration

```bash
dva compose <args> # pass-through to docker compose
dva ktl <args>     # pass-through to kubectl
dva infra up       # shared background infrastructure
dva ssh            # SSH agent container management
```

### 5. Diagnostics

```bash
dva doctor         # check environment prerequisites
dva status         # workspace status overview
dva config show    # show final merged configuration
dva config validate # validate dva.yml syntax/schema
```

## Rules

- **Never bypass DVA.** Do not use `docker compose exec ...` or `kubectl exec ...` if DVA has a command for it.
- **Never parse `dva.yml` manually.** Use `dva manifest -f json` to get the evaluated configuration (merges overrides from `.sb/dva/*.yml`).
- **Use `--dry-run` flag** to see what DVA will execute without running it.
- **Configuration changes** are managed by `dva config improve`, not manually.
