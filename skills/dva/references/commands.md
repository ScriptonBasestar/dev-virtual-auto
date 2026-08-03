# DVA Command Reference

Complete reference for all DVA CLI commands, flags, and options.

## Global Flags

Available on all commands:

| Flag | Alias | Description |
|------|-------|-------------|
| `--debug` | | Enable debug logging |
| `--dry-run` | `--explain`, `-e` | Preview flag; prove non-mutation in a disposable fixture before lifecycle use |
| `--json` | | JSON output (LLM-optimized) |

## Core Commands

### `dva run <command> [args]`

Execute interaction commands defined in `dva.yml`. The `run` prefix is optional — unknown commands are automatically routed to `dva run`.

```bash
dva run shell               # explicit form
dva shell                   # implicit form (same result)
dva api:test                # namespace syntax for subprojects
dva run --project api test  # explicit subproject form
```

| Flag | Description |
|------|-------------|
| `-p PORT:PORT` | Publish port (e.g., `-p 8080:80`) |
| `-e`, `--explain` | Dry-run alias, show execution plan |
| `--project NAME` | Target a specific subproject |

Dynamic routing: any unrecognized command is routed to `dva run <cmd>`. Reserved command names (up, down, build, etc.) cannot be overridden by interaction commands.

### `dva ls`

List all available interaction commands.

```bash
dva ls                # table format
dva ls -f json        # JSON output
dva ls -f yaml        # YAML output
dva ls -d             # detailed info (runner type, service, command)
```

### `dva version`

Display version, commit hash, and build date.

### `dva init` / `dva config init`

Scaffold a new `dva.yml` configuration. `dva init` is a backward-compatible alias for `dva config init`.

```bash
dva config init                  # auto-detect project structure
dva config init -t node          # use template (minimal, rails, node, python, go)
dva config init -p               # output LLM prompt instead of creating file
dva config init --ai             # auto-generate via Claude Code CLI
dva config init --ai --no-ai-docs  # skip agent docs during AI generation
dva config init -v               # verbose progress during AI generation
dva config init --devcontainer   # include .devcontainer/devcontainer.json
dva config init --all            # enable all detected features
```

## Lifecycle Commands

Lifecycle commands operate on the `stack:` pipeline, executing plugins in `order` sequence. The hookable lifecycle commands support `before:`, `after:`, and `replace:` hooks.

### `dva up <PLAN>`

Start services via stack plugins.

```bash
dva up local-infra            # execute a named plan
dva up local-dev              # execute compose + native runners selected by the plan
dva up local-dev --force      # ignore health checks, force restart
dva up local-dev --no-wait    # return immediately (skip wait)
```

Automatic skip: if all services are already running and healthy, `dva up` displays status without restarting. Use `--force` to override.

| Flag | Alias | Description |
|------|-------|-------------|
| `--force` | | Ignore health, force restart |
| `--no-wait` | | Return immediately |
| `--var KEY=VAL` | | Override a plan variable |
| `--dry-run` | | Preview without mutation |

The `--mode`, `--env`, and tag filters remain available only for legacy
configurations without named plans.

### `dva down <PLAN>`

Stop and remove all services.

```bash
dva down local-dev        # reverse-order teardown of the named plan
```

Use the same plan name used for `up`.

### `dva restart <PLAN>`

Stop and start the same named plan while preserving its runner and service selection.

```bash
dva restart local-dev
dva restart local-dev --dry-run
```

### `dva stop <PLAN>`

Stop the named plan without removing resources.

```bash
dva stop local-dev
```

Use the same plan name used for `up`.

### `dva build [SERVICE...]`

Build service images. Supports lifecycle hooks.

```bash
dva build                 # build all
dva build api             # build specific service
```

### `dva logs [SERVICE]`

View container logs.

```bash
dva logs                  # all service logs
dva logs api              # specific service logs
```

### `dva clean`

Remove containers, networks, and optionally volumes/images.

```bash
dva clean                 # containers + networks
dva clean -v              # + volumes (data loss warning)
dva clean -i              # + locally built images
dva clean -f              # skip confirmation prompt
```

| Flag | Description |
|------|-------------|
| `-v` | Remove volumes (data loss) |
| `-i` | Remove local images |
| `-f` | Skip confirmation |

`-v`/`-i` prompt for confirmation. Where nothing can answer the prompt — a pipe, a CI
runner, `</dev/null` — the command removes nothing and **fails**, naming `-f` as the way to
proceed non-interactively. Declining at a terminal (`n`, or Enter for the default) still
exits 0: an answer was given and honoured.

### `dva app` (legacy)

Manage legacy `applications:` entries. New configurations use stack runners and plans.

```bash
dva app ls                # list defined applications
dva app build             # build applications
dva app up [NAME]         # start application(s)
dva app down [NAME]       # stop application(s)
dva app log [NAME]        # view application logs
dva app restart [NAME]    # restart application(s)
```

## Project Management

### `dva show`

Display a summary of what `dva.yml` declares. One section per declared area — stack entries with
their runners, plans, modes, environments, interactions, and so on — and sections the config does
not declare are omitted rather than printed empty. Read the output rather than this list; the
sections follow the config, so an enumeration here would drift.

Stack rows name the entry, which is the argument `dva stack up <name>` and the tag filters take.

```bash
dva show                  # human-readable summary
dva show --json           # JSON output
```

### `dva status`

Display workspace status: container states, health check results, command counts.

```bash
dva status                # human-readable
dva status --json         # JSON output
```

### `dva config show`

Output final merged configuration (base + modules + override applied).

```bash
dva config show           # JSON format (default)
dva config show -f yaml   # YAML format
```

### `dva validate` / `dva config validate`

Validate `dva.yml` schema and syntax.

```bash
dva validate              # schema + compose project name check
dva validate --fix        # auto-fix compose project name mismatch
dva config validate       # schema + compose project name check
dva config validate --fix # auto-fix compose project name mismatch
```

### `am run dva-improve` / `dva config docs`

AI-based configuration improvement.

```bash
am run dva-improve             # run AI-based configuration improvement
am run dva-improve param.mode=rewrite # run AI improvement (rewrite from scratch)
dva config docs                # generate/update AI agent config docs (CLAUDE.md/AGENTS.md)
```

## Integration Tools

### `dva compose <args>`

Docker Compose pass-through. Forward raw compose commands.

```bash
dva compose ps            # list containers
dva compose exec api sh   # exec into container
dva compose pull          # pull images
```

### `dva ktl <args>`

Kubectl pass-through. Forward raw kubectl commands.

```bash
dva ktl get pods          # list pods
dva ktl logs pod-name     # view pod logs
dva ktl exec -it pod sh   # exec into pod
```

### `dva infra <action> [SERVICE]`

Manage shared background infrastructure services (git-based).

```bash
dva infra up              # start shared infra
dva infra down            # stop shared infra
dva infra update          # update infra definitions
dva infra up postgres     # start specific infra service
```

### `dva ssh <action>`

SSH agent container management.

```bash
dva ssh up                # start SSH agent
dva ssh down              # stop SSH agent
dva ssh status            # check SSH agent status
```

## Advanced Utilities

### `dva manifest`

Output structured command manifest optimized for LLM consumption.

```bash
dva manifest              # default output
dva manifest -f json      # JSON format
dva manifest -f yaml      # YAML format
```

Manifest includes: `dva_version`, `schema_version`, `static_commands`, `dynamic_commands`, `runners`, `plans`, `subprojects`, `health_checks`, `compose_files`, `environment_keys`. Plan entries expose resolved runner, order, dependency, wave, and service-selection metadata without variable values or runner configuration.

### `dva provision [PROFILE]`

Execute provisioning scripts (one-time setup tasks).

```bash
dva provision             # run default profile
dva provision setup       # run specific profile
dva provision --list      # list available profiles
```

Provision supports parallel batch execution for independent steps.

### `dva doctor`

Check environment prerequisites and diagnose configuration issues.

```bash
dva doctor                # run all checks
dva doctor --fix          # auto-fix fixable issues
dva doctor --json         # JSON output
```

Built-in checks: Docker daemon, compose files, devcontainer.json, .gitignore for `.sb/dva/`.
User-defined checks from `checks:` section: `file_exists`, `command`, `docker_socket`.

### `dva completion <shell>`

Generate shell completion scripts.

```bash
dva completion bash       # bash completions
dva completion zsh        # zsh completions
dva completion fish       # fish completions
```

## Reserved Command Names

These built-in commands cannot be overridden by interaction commands:

`help`, `version`, `ls`, `compose`, `up`, `stop`, `down`, `build`, `clean`, `run`, `provision`, `validate`, `manifest`, `ktl`, `ssh`, `infra`, `console`, `completion`, `init`, `status`, `config`, `logs`, `restart`, `show`, `doctor`, `app`, `stack`

## Hookable Lifecycle Commands

These 7 commands support `before:`, `after:`, and `replace:` hooks in the `interaction:` section:

`up`, `down`, `stop`, `restart`, `build`, `clean`, `logs`
