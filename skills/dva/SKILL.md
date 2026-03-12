---
name: DVA (Docker Virtual Auto) Integration
description: Instructions for using DVA to execute project-specific commands and manage containers.
---

# Using DVA (Docker Virtual Auto)

This project uses `dva` (Docker Virtual Auto) to manage Docker Compose and Kubernetes interactions, as well as project-specific scripts. The `dva.yml` file acts as the configuration hub for all commands.

## When to use this skill
- You need to spin up or tear down services (`dva up`, `dva down`).
- You need to run tests, linting, or building tasks defined in the project.
- You need to interact with a container (e.g., getting a shell or running a specific script inside it).
- You want to understand what high-level commands are available in this repository.

## Capabilities

DVA provides both **static commands** (built-in Docker/Kubernetes wrappers) and **dynamic commands** (defined in `dva.yml` under `interaction`).

### Essential Workflow

1. **Discover Available Commands**
   Before guessing how to run tests or build the project, ALWAYS check the available DVA commands by running:
   ```bash
   ./bin/dva manifest -f json
   # or if dva is globally installed:
   dva manifest -f json
   ```
   *Note: Read the `dynamic_commands` section from the JSON output to find the exact commands the user has defined for this project.*

2. **Execute Dynamic Commands**
   If you find a command like `"test": { ... }` in the dynamic commands, you should run it simply with:
   ```bash
   dva test
   # or
   dva run test
   ```
   DVA will automatically route this execution to the correct Docker, Kubernetes, or local environment.

3. **Manage Lifecycle**
   - Start background services: `dva up`
   - Stop and cleanup: `dva down`
   - Restart services: `dva down && dva up`

4. **Debugging Execution**
   If you want to see what command DVA *will* run without actually running it, use:
   ```bash
   dva run <command> --explain
   ```

## Rules and Best Practices
- **Do not manually read `docker-compose.yml` and run `docker compose exec ...`** if there is a `dva` command available for the task. `dva` handles the correct arguments and environments.
- **Do not parse `dva.yml` manually.** Always use `dva manifest -f json` (or `dva ls --json`) to get the evaluated configuration, as DVA natively merges overrides and module configurations (`.dva/*.yml`).
- If a command fails, use the `--explain` flag to understand what underlying runner (DockerCompose, Kubectl, Local) is doing.
