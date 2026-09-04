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

Scaffold a new `dva.yml` configuration from a Compose file in the current directory.
`dva init` is a backward-compatible alias for `dva config init`; it does not scaffold
Compose-less, non-standard, or multi-project layouts. For those layouts, run
`am run dva-discover` first, and use `am run dva-improve -p mode=rewrite` only when a
full rewrite is intentional.

```bash
dva config init                  # auto-detect project structure
dva config init -t node          # use template (minimal, rails, node, python, go)
dva config init --recursive      # also scaffold dva.yml in detected sub-projects
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
dva up local-dev --force      # force-recreate compose services
dva up local-dev --no-wait    # return immediately (skip wait)
```

Automatic skip: if all services are already running and healthy, `dva up` displays status without restarting. Use `--force` to override.

| Flag | Alias | Description |
|------|-------|-------------|
| `--force` | | Force-recreate compose services (`--force-recreate`) |
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

### `dva build [PLAN] [ENTRY] [SERVICE...]`

Build a plan's entries: compose images for compose runners, `runners.native.build` commands
for native ones. Supports lifecycle hooks.

```bash
dva build                 # build the default plan's entries
dva build local-dev       # build the named plan's entries
dva build local-dev api   # build one entry within the plan
dva build compose web     # compose entry, single service
```

### `dva logs [PLAN] [ENTRY] [SERVICE...]`

View a plan's output. Compose entries pass through to `docker compose logs`; process and
script entries read `.sb/dva/logs/<name>.log`.

```bash
dva logs                  # the default plan's entries
dva logs local-dev        # the named plan's entries
dva logs local-dev api    # one entry within the plan
```

### `dva down [PLAN] --purge`

`dva clean` was removed; its destruction is a flag on the teardown it always belonged to.

```bash
dva down local-dev              # containers + networks
dva down local-dev --volumes    # + volumes (data loss warning)
dva down local-dev --purge      # + local images + provision markers (data loss warning)
dva down local-dev --purge --force   # skip confirmation prompt
```

| Flag | Description |
|------|-------------|
| `--volumes` | Remove volumes (data loss) |
| `--purge` | Volumes, locally built images, and provision markers |
| `--force` | Skip the `--purge` confirmation |

`--purge` prompts for confirmation. Where nothing can answer the prompt — a pipe, a CI
runner, `</dev/null` — the command removes nothing and **fails**, naming `--force` as the way
to proceed non-interactively. Declining at a terminal (`n`, or Enter for the default) still
exits 0: an answer was given and honoured.

Provision markers are keyed by profile rather than by plan, so `--purge` clears every marker
in the config directory — the same reach `clean` had. `--dry-run` names each one it would
delete instead of deleting it.

Because `--purge` is a flag, it has no `interaction:` key. A config that hooked
`interaction.clean.before` must move that step; `dva config validate` names the removal
rather than silently treating `clean` as a plain command.

## Project Management

### `dva show`

Display a summary of what `dva.yml` declares. One section per declared area — stack entries with
their runners, plans, modes, environments, interactions, and so on — and sections the config does
not declare are omitted rather than printed empty. Read the output rather than this list; the
sections follow the config, so an enumeration here would drift.

Stack rows name the entry, which is what a plan's `entries[].name` and the tag filters take.

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

### `am run dva-discover` / `am run dva-improve` / `dva config docs`

Analyze a project before changing configuration, then improve an existing configuration.
Full rewrites are explicit opt-in.

```bash
am run dva-discover            # analyze project and discover configuration candidates
am run dva-improve             # run AI-based configuration improvement
am run dva-improve -p mode=rewrite # run AI improvement (rewrite from scratch)
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

### `dva kubectl <args>`

Kubectl pass-through. Forward raw kubectl commands.

```bash
dva kubectl get pods          # list pods
dva kubectl logs pod-name     # view pod logs
dva kubectl exec -it pod sh   # exec into pod
```

`dva ktl` is a visible compatibility name for the same command.

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

Manifest includes: `dva_version`, `schema_version`, `default_plan`, `default_plan_source`, `static_commands`, `dynamic_commands`, `runners`, `plans`, `subprojects`, `health_checks`, `compose_files`, `environment_keys`. `default_plan` is the effective bare-lifecycle selection when one exists; `default_plan_source` is always one of `explicit`, `implicit-single`, or `none`. Plan entries expose resolved runner, order, dependency, wave, and service-selection metadata without variable values or runner configuration.

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

`help`, `version`, `ls`, `compose`, `up`, `stop`, `down`, `build`, `run`, `provision`, `validate`, `manifest`, `kubectl`, `ktl`, `ssh`, `console`, `completion`, `init`, `status`, `config`, `logs`, `restart`, `show`, `doctor`, `skill`, `agent-deny`

## Hookable Lifecycle Commands

These 6 commands support `before:`, `after:`, and `replace:` hooks in the `interaction:` section:

`up`, `down`, `stop`, `restart`, `build`, `logs`
